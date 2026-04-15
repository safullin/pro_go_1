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
				Address: DefaultAddress,
			},
		},
		{
			name: "custom address",
			args: []string{"-a=localhost:9090"},
			want: ServerConfig{
				Address: "localhost:9090",
			},
		},
		{
			name: "env overrides flag",
			args: []string{"-a=localhost:9090"},
			env: map[string]string{
				"ADDRESS": "localhost:9191",
			},
			want: ServerConfig{
				Address: "localhost:9191",
			},
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
			},
		},
		{
			name: "custom values",
			args: []string{"-a=localhost:9090", "-r=3", "-p=1"},
			want: AgentConfig{
				Address:        "http://localhost:9090",
				ReportInterval: 3 * time.Second,
				PollInterval:   1 * time.Second,
			},
		},
		{
			name: "env overrides flags",
			args: []string{"-a=localhost:9090", "-r=3", "-p=1"},
			env: map[string]string{
				"ADDRESS":         "localhost:9191",
				"REPORT_INTERVAL": "5",
				"POLL_INTERVAL":   "4",
			},
			want: AgentConfig{
				Address:        "http://localhost:9191",
				ReportInterval: 5 * time.Second,
				PollInterval:   4 * time.Second,
			},
		},
		{
			name: "keeps scheme and trims slash",
			args: []string{"-a=http://localhost:9090/"},
			want: AgentConfig{
				Address:        "http://localhost:9090",
				ReportInterval: 10 * time.Second,
				PollInterval:   2 * time.Second,
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
