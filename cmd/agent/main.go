package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/safullin/pro_go_1/internal/agent"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	metricsAgent := agent.New("http://localhost:8080", 2*time.Second, 10*time.Second)
	metricsAgent.Run(ctx)
}
