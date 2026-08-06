package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/safullin/pro_go_1/internal/agent"
	"github.com/safullin/pro_go_1/internal/buildinfo"
	"github.com/safullin/pro_go_1/internal/config"
	"github.com/safullin/pro_go_1/internal/cryptoutil"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	buildinfo.Print(os.Stdout, buildVersion, buildDate, buildCommit)

	cfg, err := config.ParseAgentConfig(os.Args[1:])
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

	metricsAgent := agent.New(cfg.Address, cfg.PollInterval, cfg.ReportInterval, cfg.Key)
	metricsAgent.SetRateLimit(cfg.RateLimit)
	if cfg.CryptoKey != "" {
		publicKey, err := cryptoutil.LoadPublicKey(cfg.CryptoKey)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		metricsAgent.SetPublicKey(publicKey)
	}
	metricsAgent.Run(ctx)
}
