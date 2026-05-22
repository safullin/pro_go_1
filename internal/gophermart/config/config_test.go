package config

import "testing"

func env(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestParse(t *testing.T) {
	cfg, err := parse(
		[]string{"-a=localhost:9090", "-d=postgres://flag", "-r=localhost:9091"},
		env(map[string]string{
			"RUN_ADDRESS":            "localhost:8081",
			"DATABASE_URI":           "postgres://env",
			"ACCRUAL_SYSTEM_ADDRESS": "http://localhost:8082/",
		}),
	)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.RunAddress != "localhost:8081" {
		t.Fatalf("unexpected run address: %q", cfg.RunAddress)
	}
	if cfg.DatabaseURI != "postgres://env" {
		t.Fatalf("unexpected database uri: %q", cfg.DatabaseURI)
	}
	if cfg.AccrualSystemAddress != "http://localhost:8082" {
		t.Fatalf("unexpected accrual address: %q", cfg.AccrualSystemAddress)
	}
}

func TestParseRequiresDatabaseURI(t *testing.T) {
	if _, err := parse(nil, env(nil)); err == nil {
		t.Fatal("expected database uri error")
	}
}
