package server

import (
	"net/http"

	"github.com/safullin/pro_go_1/internal/handler"
	"github.com/safullin/pro_go_1/internal/repository"
)

// NewServer собирает HTTP-сервер приложения на стандартном роутере.
func NewServer(storage repository.MetricsRepository) http.Handler {
	mux := http.NewServeMux()
	metricsHandler := handler.NewMetricsHandler(storage)
	mux.HandleFunc("/update/", metricsHandler.UpdateMetric)
	return mux
}
