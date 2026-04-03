package handler_test

import (
	"net/http"
	"net/http/httptest"
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
