package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/safullin/pro_go_1/internal/audit"
	"github.com/safullin/pro_go_1/internal/buildinfo"
	"github.com/safullin/pro_go_1/internal/config"
	"github.com/safullin/pro_go_1/internal/cryptoutil"
	grpcapi "github.com/safullin/pro_go_1/internal/grpcserver"
	"github.com/safullin/pro_go_1/internal/handler"
	"github.com/safullin/pro_go_1/internal/middleware"
	metricspb "github.com/safullin/pro_go_1/internal/proto"
	"github.com/safullin/pro_go_1/internal/repository"
	"github.com/safullin/pro_go_1/internal/server"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

const gracefulShutdownTimeout = 5 * time.Second

func main() {
	buildinfo.Print(os.Stdout, buildVersion, buildDate, buildCommit)
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) (runErr error) {
	cfg, err := config.ParseServerConfig(args)
	if err != nil {
		return fmt.Errorf("parse server config: %w", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	var decryptMiddleware func(http.Handler) http.Handler
	if cfg.CryptoKey != "" {
		privateKey, err := cryptoutil.LoadPrivateKey(cfg.CryptoKey)
		if err != nil {
			return fmt.Errorf("load private key: %w", err)
		}
		decryptMiddleware = middleware.Decrypt(privateKey)
	}

	observers := make([]audit.Observer, 0, 2)
	if cfg.AuditFile != "" {
		fileObserver, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			return fmt.Errorf("create audit file observer: %w", err)
		}
		defer func() {
			if err := fileObserver.Close(); err != nil {
				log.Printf("close audit file: %v", err)
			}
		}()
		observers = append(observers, fileObserver)
	}
	if cfg.AuditURL != "" {
		observers = append(observers, audit.NewHTTPObserver(cfg.AuditURL))
	}
	auditor := audit.NewPublisher(observers...)
	defer auditor.Close()

	var (
		metricsStorage repository.MetricsRepository
		pinger         handler.Pinger
	)

	if cfg.DatabaseDSN != "" {
		storage, err := repository.NewPostgresStorage(ctx, cfg.DatabaseDSN)
		if err != nil {
			return fmt.Errorf("create postgres storage: %w", err)
		}
		defer func() {
			runErr = errors.Join(runErr, storage.Close())
		}()

		metricsStorage = storage
		pinger = storage
	} else if cfg.FileStorage {
		storage := repository.NewPersistentStorage(cfg.FileStoragePath, cfg.StoreInterval == 0)
		if cfg.Restore {
			if err := storage.RestoreFromFile(); err != nil {
				return fmt.Errorf("restore metrics: %w", err)
			}
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			storage.RunPersistencePeriodically(ctx, cfg.StoreInterval)
		}()
		defer func() {
			stop()
			<-done
			if err := storage.Save(); err != nil {
				runErr = errors.Join(runErr, fmt.Errorf("save metrics on shutdown: %w", err))
			}
		}()
		metricsStorage = storage
	} else {
		metricsStorage = repository.NewMemStorage()
	}
	httpHandler, err := server.NewServerWithOptions(metricsStorage, server.Options{
		Key:           cfg.Key,
		TrustedSubnet: cfg.TrustedSubnet,
		Auditor:       auditor,
		Pinger:        pinger,
	})
	if err != nil {
		return fmt.Errorf("create HTTP server: %w", err)
	}
	if decryptMiddleware != nil {
		httpHandler = decryptMiddleware(httpHandler)
	}

	srv := &http.Server{
		Addr:    cfg.Address,
		Handler: httpHandler,
	}

	var (
		grpcServer   *grpc.Server
		grpcListener net.Listener
	)
	if cfg.GRPCAddress != "" {
		interceptor, err := grpcapi.TrustedSubnetInterceptor(cfg.TrustedSubnet)
		if err != nil {
			return fmt.Errorf("create trusted subnet interceptor: %w", err)
		}
		grpcListener, err = net.Listen("tcp", cfg.GRPCAddress)
		if err != nil {
			return fmt.Errorf("listen gRPC: %w", err)
		}
		defer grpcListener.Close()
		grpcServer = grpc.NewServer(grpc.UnaryInterceptor(interceptor))
		metricspb.RegisterMetricsServer(grpcServer, grpcapi.New(metricsStorage, auditor))
	}

	httpListener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	defer httpListener.Close()
	return runServers(ctx, srv, httpListener, grpcServer, grpcListener)
}

func runServers(ctx context.Context, httpServer *http.Server, httpListener net.Listener, grpcServer *grpc.Server, grpcListener net.Listener) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	serverCount := 1
	errorsChannel := make(chan error, 2)
	go func() {
		errorsChannel <- runServer(runCtx, httpServer, httpListener)
	}()
	if grpcServer != nil && grpcListener != nil {
		serverCount++
		go func() {
			errorsChannel <- runGRPCServer(runCtx, grpcServer, grpcListener)
		}()
	}

	serveErr := <-errorsChannel
	cancel()
	for i := 1; i < serverCount; i++ {
		serveErr = errors.Join(serveErr, <-errorsChannel)
	}
	return serveErr
}

func runServer(ctx context.Context, srv *http.Server, listener net.Listener) error {
	return runServerWithShutdownTimeout(ctx, srv, listener, gracefulShutdownTimeout)
}

func runGRPCServer(ctx context.Context, srv *grpc.Server, listener net.Listener) error {
	return runGRPCServerWithShutdownTimeout(ctx, srv, listener, gracefulShutdownTimeout)
}

func runGRPCServerWithShutdownTimeout(ctx context.Context, srv *grpc.Server, listener net.Listener, timeout time.Duration) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- srv.Serve(listener)
	}()

	select {
	case err := <-serveErrors:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			srv.GracefulStop()
		}()
		timer := time.NewTimer(timeout)
		select {
		case <-stopped:
			timer.Stop()
		case <-timer.C:
			srv.Stop()
			<-stopped
		}
		serveErr := <-serveErrors
		if errors.Is(serveErr, grpc.ErrServerStopped) {
			return nil
		}
		return serveErr
	}
}

func runServerWithShutdownTimeout(ctx context.Context, srv *http.Server, listener net.Listener, timeout time.Duration) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- srv.Serve(listener)
	}()

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		shutdownErr := srv.Shutdown(shutdownCtx)
		serveErr := <-serveErrors
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(shutdownErr, serveErr)
	}
}
