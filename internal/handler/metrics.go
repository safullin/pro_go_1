package handler

import (
	"encoding/json"
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/safullin/pro_go_1/internal/audit"
	"github.com/safullin/pro_go_1/internal/model"
	"github.com/safullin/pro_go_1/internal/repository"
)

// MetricsHandler обновляет метрики по HTTP.
type MetricsHandler struct {
	storage repository.MetricsRepository
	auditor *audit.Publisher
}

// NewMetricsHandler создаёт обработчик поверх хранилища.
func NewMetricsHandler(storage repository.MetricsRepository, auditors ...*audit.Publisher) *MetricsHandler {
	var auditor *audit.Publisher
	if len(auditors) > 0 {
		auditor = auditors[0]
	}
	return &MetricsHandler{storage: storage, auditor: auditor}
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
		if err := h.storage.UpdateGauge(r.Context(), metricName, value); err != nil {
			http.Error(w, "failed to update metric", http.StatusInternalServerError)
			return
		}
	case model.Counter:
		value, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			http.Error(w, "invalid counter value", http.StatusBadRequest)
			return
		}
		if err := h.storage.AddCounter(r.Context(), metricName, value); err != nil {
			http.Error(w, "failed to update metric", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	h.publish(r, []string{metricName})
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
		if err := h.storage.UpdateGauge(r.Context(), metric.ID, *metric.Value); err != nil {
			http.Error(w, "failed to update metric", http.StatusInternalServerError)
			return
		}
		metric.Delta = nil
	case model.Counter:
		if metric.Delta == nil {
			http.Error(w, "counter delta is required", http.StatusBadRequest)
			return
		}
		if err := h.storage.AddCounter(r.Context(), metric.ID, *metric.Delta); err != nil {
			http.Error(w, "failed to update metric", http.StatusInternalServerError)
			return
		}
		value, _ := h.storage.GetCounter(r.Context(), metric.ID)
		metric.Value = nil
		metric.Delta = int64Ptr(value)
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, metric)
	h.publish(r, []string{metric.ID})
}

// UpdateMetricsJSON обрабатывает POST /updates/.
func (h *MetricsHandler) UpdateMetricsJSON(w http.ResponseWriter, r *http.Request) {
	metrics, err := decodeMetrics(r)
	if err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	for i := range metrics {
		if err := validateMetric(&metrics[i]); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	updated, err := h.storage.UpdateMetrics(r.Context(), metrics)
	if err != nil {
		http.Error(w, "failed to update metrics", http.StatusInternalServerError)
		return
	}

	writeJSONMetrics(w, http.StatusOK, updated)
	h.publish(r, metricNames(metrics))
}

// GetMetricValue возвращает текущее значение метрики в text/plain.
func (h *MetricsHandler) GetMetricValue(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	var value string
	switch metricType {
	case model.Gauge:
		gauge, ok := h.storage.GetGauge(r.Context(), metricName)
		if !ok {
			http.NotFound(w, r)
			return
		}
		value = strconv.FormatFloat(gauge, 'f', -1, 64)
	case model.Counter:
		counter, ok := h.storage.GetCounter(r.Context(), metricName)
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
		value, ok := h.storage.GetGauge(r.Context(), metric.ID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		metric.Delta = nil
		metric.Value = float64Ptr(value)
	case model.Counter:
		value, ok := h.storage.GetCounter(r.Context(), metric.ID)
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
func (h *MetricsHandler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.storage.List(r.Context())

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

func decodeMetrics(r *http.Request) ([]model.Metrics, error) {
	defer r.Body.Close()

	var metrics []model.Metrics
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}

func validateMetric(metric *model.Metrics) error {
	if metric.ID == "" {
		return errMetricIDRequired
	}

	switch metric.MType {
	case model.Gauge:
		if metric.Value == nil {
			return errGaugeValueRequired
		}
		metric.Delta = nil
	case model.Counter:
		if metric.Delta == nil {
			return errCounterDeltaRequired
		}
		metric.Value = nil
	default:
		return errUnknownMetricType
	}

	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, metric model.Metrics) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(metric)
}

func writeJSONMetrics(w http.ResponseWriter, statusCode int, metrics []model.Metrics) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(metrics)
}

func float64Ptr(value float64) *float64 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func (h *MetricsHandler) publish(r *http.Request, metrics []string) {
	if h.auditor == nil {
		return
	}
	h.auditor.Publish(r.Context(), audit.Event{
		TS:        time.Now().Unix(),
		Metrics:   metrics,
		IPAddress: clientIP(r.RemoteAddr),
	})
}

func metricNames(metrics []model.Metrics) []string {
	names := make([]string, len(metrics))
	for i := range metrics {
		names[i] = metrics[i].ID
	}
	return names
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

var (
	errMetricIDRequired     = errors.New("metric id is required")
	errGaugeValueRequired   = errors.New("gauge value is required")
	errCounterDeltaRequired = errors.New("counter delta is required")
	errUnknownMetricType    = errors.New("unknown metric type")
)
