package config

import (
	"testing"
	"time"
)

func TestParseServerConfig(t *testing.T) {
	tests := []struct {
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
			name:    "unknown flag",
			args:    []string{"-x=1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseServerConfig(tt.args)
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
			name:    "unknown flag",
			args:    []string{"-x=1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAgentConfig(tt.args)
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
