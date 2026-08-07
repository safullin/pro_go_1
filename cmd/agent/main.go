package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/safullin/pro_go_1/internal/agent"
	"github.com/safullin/pro_go_1/internal/buildinfo"
	"github.com/safullin/pro_go_1/internal/config"
	"github.com/safullin/pro_go_1/internal/cryptoutil"
	metricspb "github.com/safullin/pro_go_1/internal/proto"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	buildinfo.Print(os.Stdout, buildVersion, buildDate, buildCommit)
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) (runErr error) {
	cfg, err := config.ParseAgentConfig(args)
	if err != nil {
		return fmt.Errorf("parse agent config: %w", err)
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
	if cfg.GRPCAddress != "" {
		connection, err := grpc.NewClient(cfg.GRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("create gRPC client: %w", err)
		}
		defer func() {
			runErr = errors.Join(runErr, connection.Close())
		}()
		metricsAgent.SetGRPCClient(metricspb.NewMetricsClient(connection))
	}
	if cfg.CryptoKey != "" {
		publicKey, err := cryptoutil.LoadPublicKey(cfg.CryptoKey)
		if err != nil {
			return fmt.Errorf("load public key: %w", err)
		}
		metricsAgent.SetPublicKey(publicKey)
	}
	metricsAgent.Run(ctx)
	return nil
}
