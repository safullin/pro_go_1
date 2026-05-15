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
			args: []string{"-a=localhost:9090", "-i=10", "-f=/tmp/metrics.json", "-d=postgres://user:pass@localhost/db", "-k=secret", "-r=false"},
			want: ServerConfig{
				Address:         "localhost:9090",
				StoreInterval:   10 * time.Second,
				FileStoragePath: "/tmp/metrics.json",
				DatabaseDSN:     "postgres://user:pass@localhost/db",
				Key:             "secret",
				FileStorage:     true,
				Restore:         false,
			},
		},
		{
			name: "env overrides flag",
			args: []string{"-a=localhost:9090", "-i=10", "-f=/tmp/metrics.json", "-r=false"},
			env: map[string]string{
				"ADDRESS":           "localhost:9191",
				"STORE_INTERVAL":    "5",
				"FILE_STORAGE_PATH": "/var/tmp/metrics.json",
				"DATABASE_DSN":      "postgres://postgres:postgres@localhost/praktikum",
				"KEY":               "env-secret",
				"RESTORE":           "true",
			},
			want: ServerConfig{
				Address:         "localhost:9191",
				StoreInterval:   5 * time.Second,
				FileStoragePath: "/var/tmp/metrics.json",
				DatabaseDSN:     "postgres://postgres:postgres@localhost/praktikum",
				Key:             "env-secret",
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
			args: []string{"-a=localhost:9090", "-r=3", "-p=1", "-k=secret", "-l=7"},
			want: AgentConfig{
				Address:        "http://localhost:9090",
				ReportInterval: 3 * time.Second,
				PollInterval:   1 * time.Second,
				Key:            "secret",
				RateLimit:      7,
			},
		},
		{
			name: "env overrides flags",
			args: []string{"-a=localhost:9090", "-r=3", "-p=1", "-l=2"},
			env: map[string]string{
				"ADDRESS":         "localhost:9191",
				"REPORT_INTERVAL": "5",
				"POLL_INTERVAL":   "4",
				"KEY":             "env-secret",
				"RATE_LIMIT":      "9",
			},
			want: AgentConfig{
				Address:        "http://localhost:9191",
				ReportInterval: 5 * time.Second,
				PollInterval:   4 * time.Second,
				Key:            "env-secret",
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
