package handler_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/safullin/pro_go_1/internal/model"
	"github.com/safullin/pro_go_1/internal/repository"
	"github.com/safullin/pro_go_1/internal/server"
	"github.com/safullin/pro_go_1/internal/signature"
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
	_ = storage.UpdateGauge(context.Background(), "Alloc", 123.456)
	_ = storage.AddCounter(context.Background(), "PollCount", 7)
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

func TestUpdateMetricJSON(t *testing.T) {
	storage := repository.NewMemStorage()
	srv := server.NewServer(storage)

	tests := []struct {
		name            string
		body            string
		wantStatus      int
		wantContentType string
	}{
		{
			name:            "gauge ok",
			body:            `{"id":"Alloc","type":"gauge","value":100.5}`,
			wantStatus:      http.StatusOK,
			wantContentType: "application/json",
		},
		{
			name:       "missing id",
			body:       `{"type":"gauge","value":100.5}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unknown type",
			body:       `{"id":"Alloc","type":"unknown","value":100.5}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       `{"id":"Alloc"`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/update/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()

			srv.ServeHTTP(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("unexpected status code: got %d want %d", res.Code, tt.wantStatus)
			}
			if tt.wantContentType != "" && !strings.Contains(res.Header().Get("Content-Type"), tt.wantContentType) {
				t.Fatalf("unexpected content type: got %q want %q", res.Header().Get("Content-Type"), tt.wantContentType)
			}
		})
	}
}

func TestUpdateMetricsJSON(t *testing.T) {
	storage := repository.NewMemStorage()
	srv := server.NewServer(storage)

	body := `[
		{"id":"Alloc","type":"gauge","value":100.5},
		{"id":"PollCount","type":"counter","delta":4},
		{"id":"PollCount","type":"counter","delta":3}
	]`
	req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("unexpected content type: got %q", got)
	}
	if got, ok := storage.GetGauge(context.Background(), "Alloc"); !ok || got != 100.5 {
		t.Fatalf("unexpected gauge value: got %v ok=%v", got, ok)
	}
	if got, ok := storage.GetCounter(context.Background(), "PollCount"); !ok || got != 7 {
		t.Fatalf("unexpected counter value: got %v ok=%v", got, ok)
	}

	var metrics []model.Metrics
	if err := json.NewDecoder(res.Body).Decode(&metrics); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("unexpected response metrics count: got %d want %d", len(metrics), 3)
	}
}

func TestUpdateMetricsJSONRejectsInvalidBatch(t *testing.T) {
	storage := repository.NewMemStorage()
	srv := server.NewServer(storage)

	req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(`[{"id":"Alloc","type":"gauge"}]`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusBadRequest)
	}
	if _, ok := storage.GetGauge(context.Background(), "Alloc"); ok {
		t.Fatal("invalid batch must not update storage")
	}
}

func TestUpdateMetricsJSONWithSignatureAndGzip(t *testing.T) {
	const key = "secret"

	storage := repository.NewMemStorage()
	srv := server.NewServerWithKey(storage, key)

	body := []byte(`[{"id":"Alloc","type":"gauge","value":100.5}]`)
	compressedBody := mustGzip(t, body)
	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(compressedBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set(signature.Header, signature.Sum(compressedBody, key))
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusOK)
	}
	if !signature.Valid(res.Body.Bytes(), key, res.Header().Get(signature.Header)) {
		t.Fatalf("invalid response signature: %q", res.Header().Get(signature.Header))
	}
	if got, ok := storage.GetGauge(context.Background(), "Alloc"); !ok || got != 100.5 {
		t.Fatalf("unexpected gauge value: got %v ok=%v", got, ok)
	}
}

func TestGetMetricValueJSON(t *testing.T) {
	storage := repository.NewMemStorage()
	_ = storage.UpdateGauge(context.Background(), "Alloc", 123.456)
	_ = storage.AddCounter(context.Background(), "PollCount", 7)
	srv := server.NewServer(storage)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantType   string
		wantGauge  *float64
		wantCount  *int64
	}{
		{
			name:       "gauge",
			body:       `{"id":"Alloc","type":"gauge"}`,
			wantStatus: http.StatusOK,
			wantType:   model.Gauge,
			wantGauge:  float64Ptr(123.456),
		},
		{
			name:       "counter",
			body:       `{"id":"PollCount","type":"counter"}`,
			wantStatus: http.StatusOK,
			wantType:   model.Counter,
			wantCount:  int64Ptr(7),
		},
		{
			name:       "unknown metric",
			body:       `{"id":"Missing","type":"gauge"}`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/value/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()

			srv.ServeHTTP(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("unexpected status code: got %d want %d", res.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if got := res.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
				t.Fatalf("unexpected content type: got %q", got)
			}

			var metric model.Metrics
			if err := json.NewDecoder(res.Body).Decode(&metric); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if metric.ID == "" || metric.MType != tt.wantType {
				t.Fatalf("unexpected metric payload: %#v", metric)
			}
			if tt.wantGauge != nil {
				if metric.Value == nil || *metric.Value != *tt.wantGauge {
					t.Fatalf("unexpected gauge value: got %#v want %v", metric.Value, *tt.wantGauge)
				}
			}
			if tt.wantCount != nil {
				if metric.Delta == nil || *metric.Delta != *tt.wantCount {
					t.Fatalf("unexpected counter value: got %#v want %v", metric.Delta, *tt.wantCount)
				}
			}
		})
	}
}

func TestListMetrics(t *testing.T) {
	storage := repository.NewMemStorage()
	_ = storage.UpdateGauge(context.Background(), "Alloc", 42)
	_ = storage.AddCounter(context.Background(), "PollCount", 3)
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

func float64Ptr(value float64) *float64 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func mustGzip(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}
