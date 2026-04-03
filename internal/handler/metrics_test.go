package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/safullin/pro_go_1/internal/repository"
	"github.com/safullin/pro_go_1/internal/server"
)

func TestUpdateMetricStatuses(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "gauge ok", path: "/update/gauge/Alloc/100.5", wantStatus: http.StatusOK},
		{name: "counter ok", path: "/update/counter/PollCount/10", wantStatus: http.StatusOK},
		{name: "unknown type", path: "/update/unknown/Metric/10", wantStatus: http.StatusBadRequest},
		{name: "invalid gauge", path: "/update/gauge/Alloc/not-number", wantStatus: http.StatusBadRequest},
		{name: "missing metric name", path: "/update/gauge/", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := repository.NewMemStorage()
			srv := server.NewServer(storage)

			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			res := httptest.NewRecorder()

			srv.ServeHTTP(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("unexpected status code: got %d want %d", res.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetMetricValue(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge("Alloc", 123.456)
	storage.AddCounter("PollCount", 7)
	srv := server.NewServer(storage)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "gauge", path: "/value/gauge/Alloc", wantStatus: http.StatusOK, wantBody: "123.456"},
		{name: "counter", path: "/value/counter/PollCount", wantStatus: http.StatusOK, wantBody: "7"},
		{name: "unknown metric", path: "/value/gauge/Missing", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			res := httptest.NewRecorder()

			srv.ServeHTTP(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("unexpected status code: got %d want %d", res.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && strings.TrimSpace(res.Body.String()) != tt.wantBody {
				t.Fatalf("unexpected body: got %q want %q", res.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestListMetrics(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge("Alloc", 42)
	storage.AddCounter("PollCount", 3)
	srv := server.NewServer(storage)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusOK)
	}

	body := res.Body.String()
	if !strings.Contains(body, "Alloc: 42") {
		t.Fatalf("response body does not contain gauge metric: %q", body)
	}
	if !strings.Contains(body, "PollCount: 3") {
		t.Fatalf("response body does not contain counter metric: %q", body)
	}
}
