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

// Options хранит дополнительные параметры HTTP-сервера.
type Options struct {
	Key           string
	TrustedSubnet string
	Auditor       *audit.Publisher
	Pinger        handler.Pinger
}

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
	options := Options{Key: key, Auditor: auditor}
	if len(pingers) > 0 {
		options.Pinger = pingers[0]
	}
	serverHandler, _ := NewServerWithOptions(storage, options)
	return serverHandler
}

// NewServerWithOptions собирает HTTP-сервер с заданными параметрами.
func NewServerWithOptions(storage repository.MetricsRepository, options Options) (http.Handler, error) {
	router := chi.NewRouter()
	metricsHandler := handler.NewMetricsHandler(storage, options.Auditor)
	trustedSubnet, err := middleware.TrustedSubnet(options.TrustedSubnet)
	if err != nil {
		return nil, err
	}

	router.Use(middleware.Signature(options.Key))
	router.Use(middleware.Gzip)
	router.Use(middleware.RequestLogger(logger.New()))
	router.Get("/ping", handler.NewPingHandler(options.Pinger).Ping)
	router.With(trustedSubnet).Post("/updates", metricsHandler.UpdateMetricsJSON)
	router.With(trustedSubnet).Post("/updates/", metricsHandler.UpdateMetricsJSON)
	router.With(trustedSubnet).Post("/update", metricsHandler.UpdateMetricJSON)
	router.With(trustedSubnet).Post("/update/", metricsHandler.UpdateMetricJSON)
	router.With(trustedSubnet).Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Post("/value", metricsHandler.GetMetricValueJSON)
	router.Post("/value/", metricsHandler.GetMetricValueJSON)
	router.Get("/value/{type}/{name}", metricsHandler.GetMetricValue)
	router.Get("/", metricsHandler.ListMetrics)
	return router, nil
}
