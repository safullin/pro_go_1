package handler

import (
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

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
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	rawValue := chi.URLParam(r, "value")

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

// GetMetricValue возвращает текущее значение метрики в text/plain.
func (h *MetricsHandler) GetMetricValue(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	var value string
	switch metricType {
	case model.Gauge:
		gauge, ok := h.storage.GetGauge(metricName)
		if !ok {
			http.NotFound(w, r)
			return
		}
		value = strconv.FormatFloat(gauge, 'f', -1, 64)
	case model.Counter:
		counter, ok := h.storage.GetCounter(metricName)
		if !ok {
			http.NotFound(w, r)
			return
		}
		value = strconv.FormatInt(counter, 10)
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, value)
}

// ListMetrics отдаёт HTML-страницу со списком известных метрик.
func (h *MetricsHandler) ListMetrics(w http.ResponseWriter, _ *http.Request) {
	metrics := h.storage.List()

	var body strings.Builder
	body.WriteString("<!DOCTYPE html><html><head><title>Metrics</title></head><body><h1>Metrics</h1><ul>")
	for _, metric := range metrics {
		body.WriteString("<li>")
		body.WriteString(html.EscapeString(metric.Type))
		body.WriteString(" ")
		body.WriteString(html.EscapeString(metric.Name))
		body.WriteString(": ")
		body.WriteString(html.EscapeString(metric.Value))
		body.WriteString("</li>")
	}
	body.WriteString("</ul></body></html>")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body.String())
}
