package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/safullin/pro_go_1/internal/handler"
	"github.com/safullin/pro_go_1/internal/logger"
	"github.com/safullin/pro_go_1/internal/middleware"
	"github.com/safullin/pro_go_1/internal/repository"
)

// NewServer собирает HTTP-сервер приложения на chi.
func NewServer(storage repository.MetricsRepository) http.Handler {
	router := chi.NewRouter()
	metricsHandler := handler.NewMetricsHandler(storage)
	router.Use(middleware.RequestLogger(logger.New()))
	router.Post("/update", metricsHandler.UpdateMetricJSON)
	router.Post("/update/", metricsHandler.UpdateMetricJSON)
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Post("/value", metricsHandler.GetMetricValueJSON)
	router.Post("/value/", metricsHandler.GetMetricValueJSON)
	router.Get("/value/{type}/{name}", metricsHandler.GetMetricValue)
	router.Get("/", metricsHandler.ListMetrics)
	return router
}
