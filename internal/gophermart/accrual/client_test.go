package accrual

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"12345678903","status":"PROCESSED","accrual":42.5}`))
	}))
	defer server.Close()

	result, delay, err := NewClient(server.URL).Result(context.Background(), "12345678903")
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if delay != 0 {
		t.Fatalf("unexpected delay: %v", delay)
	}
	if result.Status != StatusProcessed || result.Accrual == nil || *result.Accrual != 42.5 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestClientRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, delay, err := NewClient(server.URL).Result(context.Background(), "12345678903")
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if delay != 3*time.Second {
		t.Fatalf("unexpected delay: got %v want %v", delay, 3*time.Second)
	}
}
