package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/safullin/pro_go_1/internal/handler"
	"github.com/safullin/pro_go_1/internal/repository"
)

func ExampleMetricsHandler_UpdateMetricsJSON() {
	storage := repository.NewMemStorage()
	metricsHandler := handler.NewMetricsHandler(storage)

	req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(`[{"id":"Alloc","type":"gauge","value":100.5},{"id":"PollCount","type":"counter","delta":3}]`))
	res := httptest.NewRecorder()
	metricsHandler.UpdateMetricsJSON(res, req)

	fmt.Println(res.Code)
	fmt.Print(strings.TrimSpace(res.Body.String()))

	// Output:
	// 200
	// [{"id":"Alloc","type":"gauge","value":100.5},{"id":"PollCount","type":"counter","delta":3}]
}

func ExampleMetricsHandler_GetMetricValue() {
	storage := repository.NewMemStorage()
	_ = storage.UpdateGauge(context.Background(), "Alloc", 100.5)
	metricsHandler := handler.NewMetricsHandler(storage)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("type", "gauge")
	routeContext.URLParams.Add("name", "Alloc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	res := httptest.NewRecorder()
	metricsHandler.GetMetricValue(res, req)

	fmt.Println(res.Code)
	fmt.Print(strings.TrimSpace(res.Body.String()))

	// Output:
	// 200
	// 100.5
}
