package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedSubnet(t *testing.T) {
	tests := []struct {
		name       string
		cidr       string
		address    string
		wantStatus int
	}{
		{name: "empty subnet", wantStatus: http.StatusNoContent},
		{name: "allowed IPv4", cidr: "192.0.2.0/24", address: "192.0.2.10", wantStatus: http.StatusNoContent},
		{name: "forbidden IPv4", cidr: "192.0.2.0/24", address: "198.51.100.10", wantStatus: http.StatusForbidden},
		{name: "missing address", cidr: "192.0.2.0/24", wantStatus: http.StatusForbidden},
		{name: "invalid address", cidr: "192.0.2.0/24", address: "unknown", wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter, err := TrustedSubnet(test.cidr)
			if err != nil {
				t.Fatalf("TrustedSubnet() error: %v", err)
			}
			handler := filter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(http.MethodPost, "/updates/", nil)
			request.Header.Set("X-Real-IP", test.address)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestTrustedSubnetRejectsInvalidCIDR(t *testing.T) {
	if _, err := TrustedSubnet("not-a-subnet"); err == nil {
		t.Fatal("TrustedSubnet() error = nil, want error")
	}
}
