package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServerJSONConfig(t *testing.T) {
	path := writeConfigFile(t, `{
		"address": "localhost:9090",
		"restore": false,
		"store_interval": "1500ms",
		"store_file": "/tmp/config-metrics.json",
		"database_dsn": "postgres://config",
		"key": "config-signature",
		"crypto_key": "/tmp/config-private.pem",
		"audit_file": "/tmp/config-audit.log",
		"audit_url": "https://audit.example/config"
	}`)

	got, err := parseServerConfig([]string{"-config=" + path}, envMap(nil))
	if err != nil {
		t.Fatalf("parseServerConfig() error: %v", err)
	}
	want := ServerConfig{
		Address:         "localhost:9090",
		StoreInterval:   1500 * time.Millisecond,
		FileStoragePath: "/tmp/config-metrics.json",
		DatabaseDSN:     "postgres://config",
		Key:             "config-signature",
		CryptoKey:       "/tmp/config-private.pem",
		AuditFile:       "/tmp/config-audit.log",
		AuditURL:        "https://audit.example/config",
		FileStorage:     true,
		Restore:         false,
	}
	if got != want {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestAgentJSONConfig(t *testing.T) {
	path := writeConfigFile(t, `{
		"address": "localhost:9091",
		"report_interval": "1500ms",
		"poll_interval": "250ms",
		"key": "config-signature",
		"crypto_key": "/tmp/config-public.pem",
		"rate_limit": 7
	}`)

	got, err := parseAgentConfig([]string{"-c", path}, envMap(nil))
	if err != nil {
		t.Fatalf("parseAgentConfig() error: %v", err)
	}
	want := AgentConfig{
		Address:        "http://localhost:9091",
		ReportInterval: 1500 * time.Millisecond,
		PollInterval:   250 * time.Millisecond,
		Key:            "config-signature",
		CryptoKey:      "/tmp/config-public.pem",
		RateLimit:      7,
	}
	if got != want {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestJSONConfigPriority(t *testing.T) {
	serverPath := writeConfigFile(t, `{
		"address": "localhost:9000",
		"store_interval": "10s",
		"store_file": "/tmp/config.json",
		"crypto_key": "/tmp/config-private.pem"
	}`)
	serverConfig, err := parseServerConfig(
		[]string{
			"-c=" + serverPath,
			"-a=localhost:9001",
			"-i=2",
			"-f=/tmp/flag.json",
			"-crypto-key=/tmp/flag-private.pem",
		},
		envMap(map[string]string{
			"ADDRESS":      "localhost:9002",
			"STORE_FILE":   "/tmp/env.json",
			"CRYPTO_KEY":   "/tmp/env-private.pem",
			"CONFIG":       serverPath,
			"AUDIT_FILE":   "/tmp/env-audit.log",
			"DATABASE_DSN": "postgres://env",
		}),
	)
	if err != nil {
		t.Fatalf("parseServerConfig() error: %v", err)
	}
	if serverConfig.Address != "localhost:9002" {
		t.Errorf("server address = %q, want env value", serverConfig.Address)
	}
	if serverConfig.StoreInterval != 2*time.Second {
		t.Errorf("store interval = %s, want flag value", serverConfig.StoreInterval)
	}
	if serverConfig.FileStoragePath != "/tmp/env.json" {
		t.Errorf("store file = %q, want env value", serverConfig.FileStoragePath)
	}
	if serverConfig.CryptoKey != "/tmp/env-private.pem" {
		t.Errorf("crypto key = %q, want env value", serverConfig.CryptoKey)
	}

	agentPath := writeConfigFile(t, `{
		"address": "localhost:9100",
		"report_interval": "10s",
		"poll_interval": "10s",
		"crypto_key": "/tmp/config-public.pem",
		"rate_limit": 2
	}`)
	agentConfig, err := parseAgentConfig(
		[]string{
			"-config=" + agentPath,
			"-a=localhost:9101",
			"-r=3",
			"-p=2",
			"-crypto-key=/tmp/flag-public.pem",
			"-l=4",
		},
		envMap(map[string]string{
			"ADDRESS":    "localhost:9102",
			"CRYPTO_KEY": "/tmp/env-public.pem",
			"RATE_LIMIT": "5",
		}),
	)
	if err != nil {
		t.Fatalf("parseAgentConfig() error: %v", err)
	}
	if agentConfig.Address != "http://localhost:9102" {
		t.Errorf("agent address = %q, want env value", agentConfig.Address)
	}
	if agentConfig.ReportInterval != 3*time.Second {
		t.Errorf("report interval = %s, want flag value", agentConfig.ReportInterval)
	}
	if agentConfig.PollInterval != 2*time.Second {
		t.Errorf("poll interval = %s, want flag value", agentConfig.PollInterval)
	}
	if agentConfig.CryptoKey != "/tmp/env-public.pem" {
		t.Errorf("crypto key = %q, want env value", agentConfig.CryptoKey)
	}
	if agentConfig.RateLimit != 5 {
		t.Errorf("rate limit = %d, want env value", agentConfig.RateLimit)
	}
}

func TestConfigEnvironmentOverridesFlagPath(t *testing.T) {
	flagPath := writeConfigFile(t, `{"address":"localhost:9200"}`)
	envPath := writeConfigFile(t, `{"address":"localhost:9201"}`)

	got, err := parseAgentConfig(
		[]string{"-c=" + flagPath},
		envMap(map[string]string{"CONFIG": envPath}),
	)
	if err != nil {
		t.Fatalf("parseAgentConfig() error: %v", err)
	}
	if got.Address != "http://localhost:9201" {
		t.Fatalf("address = %q, want env config value", got.Address)
	}
}

func TestInvalidJSONConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "invalid JSON",
			content: `{"address":`,
		},
		{
			name:    "unknown field",
			content: `{"unknown":true}`,
		},
		{
			name:    "invalid duration",
			content: `{"report_interval":"soon"}`,
		},
		{
			name:    "trailing data",
			content: `{"address":"localhost:8080"} {}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfigFile(t, test.content)
			if _, err := parseAgentConfig([]string{"-c=" + path}, envMap(nil)); err == nil {
				t.Fatal("parseAgentConfig() error = nil, want error")
			}
		})
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
