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

	"github.com/safullin/pro_go_1/internal/audit"
	"github.com/safullin/pro_go_1/internal/buildinfo"
	"github.com/safullin/pro_go_1/internal/config"
	"github.com/safullin/pro_go_1/internal/cryptoutil"
	"github.com/safullin/pro_go_1/internal/middleware"
	"github.com/safullin/pro_go_1/internal/repository"
	"github.com/safullin/pro_go_1/internal/server"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	buildinfo.Print(os.Stdout, buildVersion, buildDate, buildCommit)

	cfg, err := config.ParseServerConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
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
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		decryptMiddleware = middleware.Decrypt(privateKey)
	}

	observers := make([]audit.Observer, 0, 2)
	if cfg.AuditFile != "" {
		fileObserver, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
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
		handler         http.Handler
		persistenceDone <-chan struct{}
		flushStorage    func() error
	)

	if cfg.DatabaseDSN != "" {
		storage, err := repository.NewPostgresStorage(ctx, cfg.DatabaseDSN)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer storage.Close()

		handler = server.NewServerWithKeyAndAudit(storage, cfg.Key, auditor, storage)
	} else if cfg.FileStorage {
		storage := repository.NewPersistentStorage(cfg.FileStoragePath, cfg.StoreInterval == 0)
		if cfg.Restore {
			if err := storage.RestoreFromFile(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		done := make(chan struct{})
		persistenceDone = done
		flushStorage = storage.Save
		go func() {
			defer close(done)
			storage.RunPersistencePeriodically(ctx, cfg.StoreInterval)
		}()
		handler = server.NewServerWithKeyAndAudit(storage, cfg.Key, auditor)
	} else {
		handler = server.NewServerWithKeyAndAudit(repository.NewMemStorage(), cfg.Key, auditor)
	}
	if decryptMiddleware != nil {
		handler = decryptMiddleware(handler)
	}

	srv := &http.Server{
		Addr:    cfg.Address,
		Handler: handler,
	}

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		log.Fatal(err)
	}
	serveErr := runServer(ctx, srv, listener)
	stop()

	if persistenceDone != nil {
		<-persistenceDone
	}
	if flushStorage != nil {
		if err := flushStorage(); err != nil {
			log.Printf("save metrics on shutdown: %v", err)
		}
	}
	if serveErr != nil {
		log.Fatal(serveErr)
	}
}

func runServer(ctx context.Context, srv *http.Server, listener net.Listener) error {
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
		shutdownErr := srv.Shutdown(context.Background())
		serveErr := <-serveErrors
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(shutdownErr, serveErr)
	}
}
