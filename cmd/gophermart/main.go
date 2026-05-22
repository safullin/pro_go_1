package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/safullin/gophermart/internal/gophermart/accrual"
	"github.com/safullin/gophermart/internal/gophermart/config"
	"github.com/safullin/gophermart/internal/gophermart/server"
	"github.com/safullin/gophermart/internal/gophermart/storage"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.Open(ctx, cfg.DatabaseURI)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()

	if cfg.AccrualSystemAddress != "" {
		worker := accrual.NewWorker(store, accrual.NewClient(cfg.AccrualSystemAddress))
		go worker.Run(ctx)
	}

	srv := &http.Server{
		Addr:    cfg.RunAddress,
		Handler: server.New(store),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
