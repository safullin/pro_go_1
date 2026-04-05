package config

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	DefaultAddress        = "localhost:8080"
	DefaultReportInterval = 10 * time.Second
	DefaultPollInterval   = 2 * time.Second
)

// ServerConfig хранит параметры запуска HTTP-сервера.
type ServerConfig struct {
	Address string
}

// AgentConfig хранит параметры запуска агента.
type AgentConfig struct {
	Address        string
	ReportInterval time.Duration
	PollInterval   time.Duration
}

// ParseServerConfig парсит флаги сервера.
func ParseServerConfig(args []string) (ServerConfig, error) {
	cfg := ServerConfig{
		Address: DefaultAddress,
	}

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Address, "a", cfg.Address, "HTTP server address")

	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}

	return cfg, nil
}

// ParseAgentConfig парсит флаги агента.
func ParseAgentConfig(args []string) (AgentConfig, error) {
	reportIntervalSeconds := int(DefaultReportInterval / time.Second)
	pollIntervalSeconds := int(DefaultPollInterval / time.Second)

	cfg := AgentConfig{
		Address:        DefaultAddress,
		ReportInterval: DefaultReportInterval,
		PollInterval:   DefaultPollInterval,
	}

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Address, "a", cfg.Address, "HTTP server address")
	fs.IntVar(&reportIntervalSeconds, "r", reportIntervalSeconds, "report interval in seconds")
	fs.IntVar(&pollIntervalSeconds, "p", pollIntervalSeconds, "poll interval in seconds")

	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}
	if reportIntervalSeconds <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be positive")
	}
	if pollIntervalSeconds <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be positive")
	}

	cfg.Address = normalizeHTTPAddress(cfg.Address)
	cfg.ReportInterval = time.Duration(reportIntervalSeconds) * time.Second
	cfg.PollInterval = time.Duration(pollIntervalSeconds) * time.Second

	return cfg, nil
}

func normalizeHTTPAddress(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return strings.TrimRight(address, "/")
	}
	return "http://" + strings.TrimRight(address, "/")
}
