package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type serverFileConfig struct {
	Address       *string `json:"address"`
	Restore       *bool   `json:"restore"`
	StoreInterval *string `json:"store_interval"`
	StoreFile     *string `json:"store_file"`
	DatabaseDSN   *string `json:"database_dsn"`
	Key           *string `json:"key"`
	CryptoKey     *string `json:"crypto_key"`
	AuditFile     *string `json:"audit_file"`
	AuditURL      *string `json:"audit_url"`
	TrustedSubnet *string `json:"trusted_subnet"`
}

type agentFileConfig struct {
	Address        *string `json:"address"`
	ReportInterval *string `json:"report_interval"`
	PollInterval   *string `json:"poll_interval"`
	Key            *string `json:"key"`
	CryptoKey      *string `json:"crypto_key"`
	RateLimit      *int    `json:"rate_limit"`
}

func loadServerFileConfig(path string, cfg *ServerConfig) error {
	var fileConfig serverFileConfig
	if err := readJSONConfig(path, &fileConfig); err != nil {
		return err
	}
	if fileConfig.Address != nil {
		cfg.Address = *fileConfig.Address
	}
	if fileConfig.Restore != nil {
		cfg.Restore = *fileConfig.Restore
	}
	if fileConfig.StoreInterval != nil {
		interval, err := time.ParseDuration(*fileConfig.StoreInterval)
		if err != nil {
			return fmt.Errorf("invalid store interval in config: %w", err)
		}
		if interval < 0 {
			return fmt.Errorf("store interval must be non-negative")
		}
		cfg.StoreInterval = interval
	}
	if fileConfig.StoreFile != nil {
		cfg.FileStoragePath = *fileConfig.StoreFile
		cfg.FileStorage = *fileConfig.StoreFile != ""
	}
	if fileConfig.DatabaseDSN != nil {
		cfg.DatabaseDSN = *fileConfig.DatabaseDSN
	}
	if fileConfig.Key != nil {
		cfg.Key = *fileConfig.Key
	}
	if fileConfig.CryptoKey != nil {
		cfg.CryptoKey = *fileConfig.CryptoKey
	}
	if fileConfig.AuditFile != nil {
		cfg.AuditFile = *fileConfig.AuditFile
	}
	if fileConfig.AuditURL != nil {
		cfg.AuditURL = *fileConfig.AuditURL
	}
	if fileConfig.TrustedSubnet != nil {
		cfg.TrustedSubnet = *fileConfig.TrustedSubnet
	}
	return nil
}

func loadAgentFileConfig(path string, cfg *AgentConfig) error {
	var fileConfig agentFileConfig
	if err := readJSONConfig(path, &fileConfig); err != nil {
		return err
	}
	if fileConfig.Address != nil {
		cfg.Address = *fileConfig.Address
	}
	if fileConfig.ReportInterval != nil {
		interval, err := time.ParseDuration(*fileConfig.ReportInterval)
		if err != nil {
			return fmt.Errorf("invalid report interval in config: %w", err)
		}
		if interval <= 0 {
			return fmt.Errorf("report interval must be positive")
		}
		cfg.ReportInterval = interval
	}
	if fileConfig.PollInterval != nil {
		interval, err := time.ParseDuration(*fileConfig.PollInterval)
		if err != nil {
			return fmt.Errorf("invalid poll interval in config: %w", err)
		}
		if interval <= 0 {
			return fmt.Errorf("poll interval must be positive")
		}
		cfg.PollInterval = interval
	}
	if fileConfig.Key != nil {
		cfg.Key = *fileConfig.Key
	}
	if fileConfig.CryptoKey != nil {
		cfg.CryptoKey = *fileConfig.CryptoKey
	}
	if fileConfig.RateLimit != nil {
		if *fileConfig.RateLimit <= 0 {
			return fmt.Errorf("rate limit must be positive")
		}
		cfg.RateLimit = *fileConfig.RateLimit
	}
	return nil
}

func readJSONConfig(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode config file: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode config file: unexpected trailing data")
	}
	return nil
}

func findConfigPath(args []string, lookup envLookup) string {
	var path string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-c", "--c", "-config", "--config":
			if index+1 < len(args) {
				path = args[index+1]
				index++
			}
		default:
			for _, prefix := range []string{"-c=", "--c=", "-config=", "--config="} {
				if value, ok := strings.CutPrefix(args[index], prefix); ok {
					path = value
					break
				}
			}
		}
	}
	if value, ok := lookup("CONFIG"); ok && value != "" {
		path = value
	}
	return path
}
