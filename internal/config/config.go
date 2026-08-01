package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAddress задаёт адрес HTTP-сервера по умолчанию.
	DefaultAddress = "localhost:8080"
	// DefaultReportInterval задаёт интервал отправки метрик по умолчанию.
	DefaultReportInterval = 10 * time.Second
	// DefaultPollInterval задаёт интервал сбора метрик по умолчанию.
	DefaultPollInterval = 2 * time.Second
	// DefaultRateLimit задаёт число одновременных запросов агента по умолчанию.
	DefaultRateLimit = 1
	// DefaultStoreInterval задаёт интервал сохранения метрик по умолчанию.
	DefaultStoreInterval = 300 * time.Second
	// DefaultFileStoragePath задаёт путь к файловому хранилищу по умолчанию.
	DefaultFileStoragePath = "metrics-db.json"
	// DefaultRestore определяет восстановление метрик из файла по умолчанию.
	DefaultRestore = true
)

// ServerConfig хранит параметры запуска HTTP-сервера.
type ServerConfig struct {
	Address         string
	StoreInterval   time.Duration
	FileStoragePath string
	DatabaseDSN     string
	Key             string
	CryptoKey       string
	AuditFile       string
	AuditURL        string
	FileStorage     bool
	Restore         bool
}

// AgentConfig хранит параметры запуска агента.
type AgentConfig struct {
	Address        string
	ReportInterval time.Duration
	PollInterval   time.Duration
	Key            string
	CryptoKey      string
	RateLimit      int
}

type envLookup func(string) (string, bool)

// ParseServerConfig парсит флаги сервера.
func ParseServerConfig(args []string) (ServerConfig, error) {
	return parseServerConfig(args, os.LookupEnv)
}

func parseServerConfig(args []string, lookup envLookup) (ServerConfig, error) {
	storeIntervalSeconds := int(DefaultStoreInterval / time.Second)

	cfg := ServerConfig{
		Address:         DefaultAddress,
		StoreInterval:   DefaultStoreInterval,
		FileStoragePath: DefaultFileStoragePath,
		Restore:         DefaultRestore,
	}

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Address, "a", cfg.Address, "HTTP server address")
	fs.IntVar(&storeIntervalSeconds, "i", storeIntervalSeconds, "store interval in seconds")
	fs.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "path to metrics storage file")
	fs.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "database connection string")
	fs.StringVar(&cfg.Key, "k", cfg.Key, "hash signature key")
	fs.StringVar(&cfg.CryptoKey, "crypto-key", cfg.CryptoKey, "private key file")
	fs.BoolVar(&cfg.Restore, "r", cfg.Restore, "restore metrics from file on startup")
	fs.StringVar(&cfg.AuditFile, "audit-file", cfg.AuditFile, "audit log file")
	fs.StringVar(&cfg.AuditURL, "audit-url", cfg.AuditURL, "audit receiver URL")

	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "f" && cfg.FileStoragePath != "" {
			cfg.FileStorage = true
		}
	})
	if storeIntervalSeconds < 0 {
		return ServerConfig{}, fmt.Errorf("store interval must be non-negative")
	}
	if value, ok := lookup("ADDRESS"); ok && value != "" {
		cfg.Address = value
	}
	if value, ok := lookup("STORE_INTERVAL"); ok && value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("invalid store interval: %w", err)
		}
		if seconds < 0 {
			return ServerConfig{}, fmt.Errorf("store interval must be non-negative")
		}
		storeIntervalSeconds = seconds
	}
	if value, ok := lookup("FILE_STORAGE_PATH"); ok && value != "" {
		cfg.FileStoragePath = value
		cfg.FileStorage = true
	}
	if value, ok := lookup("DATABASE_DSN"); ok && value != "" {
		cfg.DatabaseDSN = value
	}
	if value, ok := lookup("KEY"); ok && value != "" {
		cfg.Key = value
	}
	if value, ok := lookup("CRYPTO_KEY"); ok && value != "" {
		cfg.CryptoKey = value
	}
	if value, ok := lookup("AUDIT_FILE"); ok && value != "" {
		cfg.AuditFile = value
	}
	if value, ok := lookup("AUDIT_URL"); ok && value != "" {
		cfg.AuditURL = value
	}
	if value, ok := lookup("RESTORE"); ok && value != "" {
		restore, err := strconv.ParseBool(value)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("invalid restore flag: %w", err)
		}
		cfg.Restore = restore
	}

	cfg.StoreInterval = time.Duration(storeIntervalSeconds) * time.Second

	return cfg, nil
}

// ParseAgentConfig парсит флаги агента.
func ParseAgentConfig(args []string) (AgentConfig, error) {
	return parseAgentConfig(args, os.LookupEnv)
}

func parseAgentConfig(args []string, lookup envLookup) (AgentConfig, error) {
	reportIntervalSeconds := int(DefaultReportInterval / time.Second)
	pollIntervalSeconds := int(DefaultPollInterval / time.Second)

	cfg := AgentConfig{
		Address:        DefaultAddress,
		ReportInterval: DefaultReportInterval,
		PollInterval:   DefaultPollInterval,
		RateLimit:      DefaultRateLimit,
	}

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Address, "a", cfg.Address, "HTTP server address")
	fs.IntVar(&reportIntervalSeconds, "r", reportIntervalSeconds, "report interval in seconds")
	fs.IntVar(&pollIntervalSeconds, "p", pollIntervalSeconds, "poll interval in seconds")
	fs.StringVar(&cfg.Key, "k", cfg.Key, "hash signature key")
	fs.StringVar(&cfg.CryptoKey, "crypto-key", cfg.CryptoKey, "public key file")
	fs.IntVar(&cfg.RateLimit, "l", cfg.RateLimit, "maximum concurrent requests")

	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}
	if reportIntervalSeconds <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be positive")
	}
	if pollIntervalSeconds <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be positive")
	}
	if cfg.RateLimit <= 0 {
		return AgentConfig{}, fmt.Errorf("rate limit must be positive")
	}
	if value, ok := lookup("ADDRESS"); ok && value != "" {
		cfg.Address = value
	}
	if value, ok := lookup("REPORT_INTERVAL"); ok && value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return AgentConfig{}, fmt.Errorf("invalid report interval: %w", err)
		}
		if seconds <= 0 {
			return AgentConfig{}, fmt.Errorf("report interval must be positive")
		}
		reportIntervalSeconds = seconds
	}
	if value, ok := lookup("POLL_INTERVAL"); ok && value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return AgentConfig{}, fmt.Errorf("invalid poll interval: %w", err)
		}
		if seconds <= 0 {
			return AgentConfig{}, fmt.Errorf("poll interval must be positive")
		}
		pollIntervalSeconds = seconds
	}
	if value, ok := lookup("KEY"); ok && value != "" {
		cfg.Key = value
	}
	if value, ok := lookup("CRYPTO_KEY"); ok && value != "" {
		cfg.CryptoKey = value
	}
	if value, ok := lookup("RATE_LIMIT"); ok && value != "" {
		rateLimit, err := strconv.Atoi(value)
		if err != nil {
			return AgentConfig{}, fmt.Errorf("invalid rate limit: %w", err)
		}
		if rateLimit <= 0 {
			return AgentConfig{}, fmt.Errorf("rate limit must be positive")
		}
		cfg.RateLimit = rateLimit
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
