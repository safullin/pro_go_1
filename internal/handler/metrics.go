package handler

import (
	"encoding/json"
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

// UpdateMetricJSON обрабатывает POST /update/.
func (h *MetricsHandler) UpdateMetricJSON(w http.ResponseWriter, r *http.Request) {
	metric, err := decodeMetric(r)
	if err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if metric.ID == "" {
		http.Error(w, "metric id is required", http.StatusNotFound)
		return
	}

	switch metric.MType {
	case model.Gauge:
		if metric.Value == nil {
			http.Error(w, "gauge value is required", http.StatusBadRequest)
			return
		}
		h.storage.UpdateGauge(metric.ID, *metric.Value)
		metric.Delta = nil
	case model.Counter:
		if metric.Delta == nil {
			http.Error(w, "counter delta is required", http.StatusBadRequest)
			return
		}
		h.storage.AddCounter(metric.ID, *metric.Delta)
		value, _ := h.storage.GetCounter(metric.ID)
		metric.Value = nil
		metric.Delta = int64Ptr(value)
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, metric)
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

// GetMetricValueJSON возвращает текущее значение метрики в application/json.
func (h *MetricsHandler) GetMetricValueJSON(w http.ResponseWriter, r *http.Request) {
	metric, err := decodeMetric(r)
	if err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if metric.ID == "" {
		http.Error(w, "metric id is required", http.StatusNotFound)
		return
	}

	switch metric.MType {
	case model.Gauge:
		value, ok := h.storage.GetGauge(metric.ID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		metric.Delta = nil
		metric.Value = float64Ptr(value)
	case model.Counter:
		value, ok := h.storage.GetCounter(metric.ID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		metric.Value = nil
		metric.Delta = int64Ptr(value)
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, metric)
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

func decodeMetric(r *http.Request) (model.Metrics, error) {
	defer r.Body.Close()

	var metric model.Metrics
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metric); err != nil {
		return model.Metrics{}, err
	}

	return metric, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, metric model.Metrics) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(metric)
}

func float64Ptr(value float64) *float64 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
