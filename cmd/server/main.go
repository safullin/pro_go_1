package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/safullin/pro_go_1/internal/audit"
	"github.com/safullin/pro_go_1/internal/config"
	"github.com/safullin/pro_go_1/internal/repository"
	"github.com/safullin/pro_go_1/internal/server"
)

func main() {
	cfg, err := config.ParseServerConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
		handler http.Handler
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

		go storage.RunPersistencePeriodically(ctx, cfg.StoreInterval)
		handler = server.NewServerWithKeyAndAudit(storage, cfg.Key, auditor)
	} else {
		handler = server.NewServerWithKeyAndAudit(repository.NewMemStorage(), cfg.Key, auditor)
	}

	srv := &http.Server{
		Addr:    cfg.Address,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
