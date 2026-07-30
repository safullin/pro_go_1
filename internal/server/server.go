package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/safullin/pro_go_1/internal/audit"
	"github.com/safullin/pro_go_1/internal/handler"
	"github.com/safullin/pro_go_1/internal/logger"
	"github.com/safullin/pro_go_1/internal/middleware"
	"github.com/safullin/pro_go_1/internal/repository"
)

// NewServer собирает HTTP-сервер приложения на chi.
func NewServer(storage repository.MetricsRepository, pingers ...handler.Pinger) http.Handler {
	return NewServerWithKey(storage, "", pingers...)
}

// NewServerWithAudit собирает HTTP-сервер с аудитом полученных метрик.
func NewServerWithAudit(storage repository.MetricsRepository, auditor *audit.Publisher, pingers ...handler.Pinger) http.Handler {
	return NewServerWithKeyAndAudit(storage, "", auditor, pingers...)
}

// NewServerWithKey собирает HTTP-сервер приложения с опциональной подписью данных.
func NewServerWithKey(storage repository.MetricsRepository, key string, pingers ...handler.Pinger) http.Handler {
	return NewServerWithKeyAndAudit(storage, key, nil, pingers...)
}

// NewServerWithKeyAndAudit собирает HTTP-сервер с подписью данных и аудитом.
func NewServerWithKeyAndAudit(storage repository.MetricsRepository, key string, auditor *audit.Publisher, pingers ...handler.Pinger) http.Handler {
	router := chi.NewRouter()
	metricsHandler := handler.NewMetricsHandler(storage, auditor)
	var pinger handler.Pinger
	if len(pingers) > 0 {
		pinger = pingers[0]
	}

	router.Use(middleware.Signature(key))
	router.Use(middleware.Gzip)
	router.Use(middleware.RequestLogger(logger.New()))
	router.Get("/ping", handler.NewPingHandler(pinger).Ping)
	router.Post("/updates", metricsHandler.UpdateMetricsJSON)
	router.Post("/updates/", metricsHandler.UpdateMetricsJSON)
	router.Post("/update", metricsHandler.UpdateMetricJSON)
	router.Post("/update/", metricsHandler.UpdateMetricJSON)
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Post("/value", metricsHandler.GetMetricValueJSON)
	router.Post("/value/", metricsHandler.GetMetricValueJSON)
	router.Get("/value/{type}/{name}", metricsHandler.GetMetricValue)
	router.Get("/", metricsHandler.ListMetrics)
	return router
}
