package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/safullin/pro_go_1/internal/handler"
	"github.com/safullin/pro_go_1/internal/repository"
)

func BenchmarkUpdateMetricsJSON(b *testing.B) {
	const body = `[{"id":"Alloc","type":"gauge","value":100.5},{"id":"HeapAlloc","type":"gauge","value":200.5},{"id":"PollCount","type":"counter","delta":4},{"id":"RandomValue","type":"gauge","value":300.5}]`

	metricsHandler := handler.NewMetricsHandler(repository.NewMemStorage())
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(body))
		res := httptest.NewRecorder()
		metricsHandler.UpdateMetricsJSON(res, req)
		if res.Code != http.StatusOK {
			b.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusOK)
		}
	}
}
