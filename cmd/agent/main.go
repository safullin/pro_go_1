package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/safullin/pro_go_1/internal/agent"
	"github.com/safullin/pro_go_1/internal/config"
)

func main() {
	cfg, err := config.ParseAgentConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	metricsAgent := agent.New(cfg.Address, cfg.PollInterval, cfg.ReportInterval)
	metricsAgent.Run(ctx)
}
