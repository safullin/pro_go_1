package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "invalid config", args: []string{"-unknown"}, message: "parse agent config"},
		{name: "invalid key", args: []string{"-g=localhost:3200", "-crypto-key=" + filepath.Join(t.TempDir(), "missing.pem")}, message: "load public key"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run(test.args)
			if err == nil {
				t.Fatal("run() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("run() error = %q, want %q", err, test.message)
			}
		})
	}
}
