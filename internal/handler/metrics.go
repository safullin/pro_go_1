package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/safullin/pro_go_1/internal/model"
	"github.com/safullin/pro_go_1/internal/repository"
)

// MetricsHandler обновляет метрики по HTTP.
type MetricsHandler struct {
	storage repository.MetricsRepository
}

// NewMetricsHandler создаёт обработчик поверх хранилища.
func NewMetricsHandler(storage repository.MetricsRepository) *MetricsHandler {
	return &MetricsHandler{storage: storage}
}

// UpdateMetric обрабатывает POST /update/{type}/{name}/{value}.
func (h *MetricsHandler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid method", http.StatusBadRequest)
		return
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/update/"), "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(w, r)
		return
	}
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "invalid metric value", http.StatusBadRequest)
		return
	}

	metricType := parts[0]
	metricName := parts[1]
	rawValue := parts[2]

	switch metricType {
	case model.Gauge:
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			http.Error(w, "invalid gauge value", http.StatusBadRequest)
			return
		}
		h.storage.UpdateGauge(metricName, value)
	case model.Counter:
		value, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			http.Error(w, "invalid counter value", http.StatusBadRequest)
			return
		}
		h.storage.AddCounter(metricName, value)
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}
