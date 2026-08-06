package config

import (
	"testing"
	"time"
)

func envMap(values map[string]string) envLookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestParseServerConfig(t *testing.T) {
	tests := []struct {
		env     map[string]string
		name    string
		args    []string
		want    ServerConfig
		wantErr bool
	}{
		{
			name: "defaults",
			args: nil,
			want: ServerConfig{
				Address:         DefaultAddress,
				StoreInterval:   DefaultStoreInterval,
				FileStoragePath: DefaultFileStoragePath,
				Restore:         DefaultRestore,
			},
		},
		{
			name: "custom address",
			args: []string{"-a=localhost:9090", "-g=localhost:3200", "-i=10", "-f=/tmp/metrics.json", "-d=postgres://user:pass@localhost/db", "-k=secret", "-crypto-key=/tmp/private.pem", "-r=false", "--audit-file=/tmp/audit.log", "--audit-url=https://audit.example/events", "-t=192.0.2.0/24"},
			want: ServerConfig{
				Address:         "localhost:9090",
				GRPCAddress:     "localhost:3200",
				StoreInterval:   10 * time.Second,
				FileStoragePath: "/tmp/metrics.json",
				DatabaseDSN:     "postgres://user:pass@localhost/db",
				Key:             "secret",
				CryptoKey:       "/tmp/private.pem",
				AuditFile:       "/tmp/audit.log",
				AuditURL:        "https://audit.example/events",
				TrustedSubnet:   "192.0.2.0/24",
				FileStorage:     true,
				Restore:         false,
			},
		},
		{
			name: "env overrides flag",
			args: []string{"-a=localhost:9090", "-i=10", "-f=/tmp/metrics.json", "-r=false"},
			env: map[string]string{
				"ADDRESS":           "localhost:9191",
				"GRPC_ADDRESS":      "localhost:3300",
				"STORE_INTERVAL":    "5",
				"FILE_STORAGE_PATH": "/var/tmp/metrics.json",
				"DATABASE_DSN":      "postgres://postgres:postgres@localhost/praktikum",
				"KEY":               "env-secret",
				"CRYPTO_KEY":        "/var/tmp/private.pem",
				"AUDIT_FILE":        "/var/tmp/audit.log",
				"AUDIT_URL":         "https://audit.example/events",
				"TRUSTED_SUBNET":    "198.51.100.0/24",
				"RESTORE":           "true",
			},
			want: ServerConfig{
				Address:         "localhost:9191",
				GRPCAddress:     "localhost:3300",
				StoreInterval:   5 * time.Second,
				FileStoragePath: "/var/tmp/metrics.json",
				DatabaseDSN:     "postgres://postgres:postgres@localhost/praktikum",
				Key:             "env-secret",
				CryptoKey:       "/var/tmp/private.pem",
				AuditFile:       "/var/tmp/audit.log",
				AuditURL:        "https://audit.example/events",
				TrustedSubnet:   "198.51.100.0/24",
				FileStorage:     true,
				Restore:         true,
			},
		},
		{
			name: "empty file flag does not enable file storage",
			args: []string{"-f="},
			want: ServerConfig{
				Address:         DefaultAddress,
				StoreInterval:   DefaultStoreInterval,
				FileStoragePath: "",
				Restore:         DefaultRestore,
			},
		},
		{
			name:    "negative store interval",
			args:    []string{"-i=-1"},
			wantErr: true,
		},
		{
			name: "invalid restore env",
			env: map[string]string{
				"RESTORE": "nope",
			},
			wantErr: true,
		},
		{
			name:    "invalid trusted subnet",
			args:    []string{"-t=not-a-subnet"},
			wantErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"-x=1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServerConfig(tt.args, envMap(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected config: got %#v want %#v", got, tt.want)
			}
		})
	}
}

func TestParseAgentConfig(t *testing.T) {
	tests := []struct {
		env     map[string]string
		name    string
		args    []string
		want    AgentConfig
		wantErr bool
	}{
		{
			name: "defaults",
			args: nil,
			want: AgentConfig{
				Address:        "http://" + DefaultAddress,
				ReportInterval: 10 * time.Second,
				PollInterval:   2 * time.Second,
				RateLimit:      DefaultRateLimit,
			},
		},
		{
			name: "custom values",
			args: []string{"-a=localhost:9090", "-g=localhost:3200", "-r=3", "-p=1", "-k=secret", "-crypto-key=/tmp/public.pem", "-l=7"},
			want: AgentConfig{
				Address:        "http://localhost:9090",
				GRPCAddress:    "localhost:3200",
				ReportInterval: 3 * time.Second,
				PollInterval:   1 * time.Second,
				Key:            "secret",
				CryptoKey:      "/tmp/public.pem",
				RateLimit:      7,
			},
		},
		{
			name: "env overrides flags",
			args: []string{"-a=localhost:9090", "-r=3", "-p=1", "-l=2"},
			env: map[string]string{
				"ADDRESS":         "localhost:9191",
				"GRPC_ADDRESS":    "localhost:3300",
				"REPORT_INTERVAL": "5",
				"POLL_INTERVAL":   "4",
				"KEY":             "env-secret",
				"CRYPTO_KEY":      "/var/tmp/public.pem",
				"RATE_LIMIT":      "9",
			},
			want: AgentConfig{
				Address:        "http://localhost:9191",
				GRPCAddress:    "localhost:3300",
				ReportInterval: 5 * time.Second,
				PollInterval:   4 * time.Second,
				Key:            "env-secret",
				CryptoKey:      "/var/tmp/public.pem",
				RateLimit:      9,
			},
		},
		{
			name: "keeps scheme and trims slash",
			args: []string{"-a=http://localhost:9090/"},
			want: AgentConfig{
				Address:        "http://localhost:9090",
				ReportInterval: 10 * time.Second,
				PollInterval:   2 * time.Second,
				RateLimit:      DefaultRateLimit,
			},
		},
		{
			name:    "zero report interval",
			args:    []string{"-r=0"},
			wantErr: true,
		},
		{
			name:    "negative poll interval",
			args:    []string{"-p=-1"},
			wantErr: true,
		},
		{
			name: "zero report interval from env",
			env: map[string]string{
				"REPORT_INTERVAL": "0",
			},
			wantErr: true,
		},
		{
			name: "invalid poll interval from env",
			env: map[string]string{
				"POLL_INTERVAL": "abc",
			},
			wantErr: true,
		},
		{
			name:    "zero rate limit",
			args:    []string{"-l=0"},
			wantErr: true,
		},
		{
			name: "invalid rate limit from env",
			env: map[string]string{
				"RATE_LIMIT": "abc",
			},
			wantErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"-x=1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAgentConfig(tt.args, envMap(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected config: got %#v want %#v", got, tt.want)
			}
		})
	}
}
