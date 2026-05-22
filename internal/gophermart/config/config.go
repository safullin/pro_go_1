package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	defaultRunAddress = "localhost:8080"
)

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
}

func Parse(args []string) (Config, error) {
	return parse(args, os.LookupEnv)
}

func parse(args []string, lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{
		RunAddress: defaultRunAddress,
	}

	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "run address")
	fs.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "database uri")
	fs.StringVar(&cfg.AccrualSystemAddress, "r", cfg.AccrualSystemAddress, "accrual system address")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if value, ok := lookup("RUN_ADDRESS"); ok && value != "" {
		cfg.RunAddress = value
	}
	if value, ok := lookup("DATABASE_URI"); ok && value != "" {
		cfg.DatabaseURI = value
	}
	if value, ok := lookup("ACCRUAL_SYSTEM_ADDRESS"); ok && value != "" {
		cfg.AccrualSystemAddress = value
	}

	if cfg.DatabaseURI == "" {
		return Config{}, fmt.Errorf("database uri is required")
	}
	cfg.AccrualSystemAddress = normalizeHTTPAddress(cfg.AccrualSystemAddress)

	return cfg, nil
}

func normalizeHTTPAddress(address string) string {
	if address == "" {
		return ""
	}
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return strings.TrimRight(address, "/")
	}
	return "http://" + strings.TrimRight(address, "/")
}
