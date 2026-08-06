package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/safullin/pro_go_1/internal/repository"
)

func TestServerChecksTrustedSubnetForUpdates(t *testing.T) {
	handler, err := NewServerWithOptions(repository.NewMemStorage(), Options{TrustedSubnet: "192.0.2.0/24"})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error: %v", err)
	}

	tests := []struct {
		name       string
		address    string
		wantStatus int
	}{
		{name: "allowed", address: "192.0.2.10", wantStatus: http.StatusOK},
		{name: "forbidden", address: "198.51.100.10", wantStatus: http.StatusForbidden},
		{name: "missing", wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1", nil)
			request.Header.Set("X-Real-IP", test.address)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}

	request := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("read status = %d, want %d", response.Code, http.StatusOK)
	}
}
