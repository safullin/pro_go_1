package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/safullin/pro_go_1/internal/middleware"
)

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/1", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusCreated)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected log lines count: got %d want %d", len(lines), 2)
	}

	var requestLog map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &requestLog); err != nil {
		t.Fatalf("failed to decode request log: %v", err)
	}
	if requestLog["level"] != "info" {
		t.Fatalf("unexpected request level: got %v want %v", requestLog["level"], "info")
	}
	if requestLog["uri"] != "/update/gauge/test/1" {
		t.Fatalf("unexpected request uri: got %v", requestLog["uri"])
	}
	if requestLog["method"] != http.MethodPost {
		t.Fatalf("unexpected request method: got %v want %v", requestLog["method"], http.MethodPost)
	}
	if _, ok := requestLog["duration"]; !ok {
		t.Fatal("request log does not contain duration")
	}

	var responseLog map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &responseLog); err != nil {
		t.Fatalf("failed to decode response log: %v", err)
	}
	if responseLog["level"] != "info" {
		t.Fatalf("unexpected response level: got %v want %v", responseLog["level"], "info")
	}
	if responseLog["status"] != float64(http.StatusCreated) {
		t.Fatalf("unexpected response status: got %v want %v", responseLog["status"], http.StatusCreated)
	}
	if responseLog["size"] != float64(2) {
		t.Fatalf("unexpected response size: got %v want %v", responseLog["size"], 2)
	}
}
