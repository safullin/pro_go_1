package config

import (
	"flag"
	"os"
)

// Config содержит параметры приложения.
type Config struct {
	Address     string
	DatabaseDSN string
}

// New читает конфигурацию из переменных окружения и флагов командной строки.
func New() Config {
	envAddress := os.Getenv("SERVER_ADDRESS")
	if envAddress == "" {
		envAddress = ":8080"
	}

	envDSN := os.Getenv("DATABASE_DSN")

	cfg := Config{}
	flag.StringVar(&cfg.Address, "a", envAddress, "HTTP server address")
	flag.StringVar(&cfg.DatabaseDSN, "d", envDSN, "PostgreSQL DSN")
	flag.Parse()

	return cfg
}
