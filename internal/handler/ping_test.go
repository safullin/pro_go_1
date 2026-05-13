package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockPingService struct {
	err error
}

func (m mockPingService) Ping() error {
	return m.err
}

func TestPingHandler_ServeHTTP_OK(t *testing.T) {
	h := NewPingHandler(mockPingService{})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusOK)
	}
}

func TestPingHandler_ServeHTTP_Error(t *testing.T) {
	h := NewPingHandler(mockPingService{err: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusInternalServerError)
	}
}
