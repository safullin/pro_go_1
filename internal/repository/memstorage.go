package repository

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"github.com/safullin/pro_go_1/internal/model"
)

// MetricsRepository описывает операции обновления метрик.
type MetricsRepository interface {
	UpdateGauge(name string, value float64) error
	AddCounter(name string, delta int64) error
	UpdateMetrics(ctx context.Context, metrics []model.Metrics) ([]model.Metrics, error)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	List() []model.StoredMetric
}

// MemStorage хранит метрики в памяти.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage создаёт пустое хранилище.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// UpdateGauge заменяет значение метрики типа gauge.
func (s *MemStorage) UpdateGauge(name string, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges[name] = value
	return nil
}

// AddCounter добавляет delta к метрике типа counter.
func (s *MemStorage) AddCounter(name string, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters[name] += delta
	return nil
}

// UpdateMetrics обновляет несколько метрик одним критическим участком.
func (s *MemStorage) UpdateMetrics(_ context.Context, metrics []model.Metrics) ([]model.Metrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := make([]model.Metrics, 0, len(metrics))
	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			if metric.Value == nil {
				continue
			}
			value := *metric.Value
			s.gauges[metric.ID] = value
			updated = append(updated, model.Metrics{
				ID:    metric.ID,
				MType: model.Gauge,
				Value: &value,
			})
		case model.Counter:
			if metric.Delta == nil {
				continue
			}
			s.counters[metric.ID] += *metric.Delta
			value := s.counters[metric.ID]
			updated = append(updated, model.Metrics{
				ID:    metric.ID,
				MType: model.Counter,
				Delta: &value,
			})
		}
	}

	return updated, nil
}

// GetGauge возвращает значение gauge по имени.
func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.gauges[name]
	return value, ok
}

// GetCounter возвращает значение counter по имени.
func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.counters[name]
	return value, ok
}

// List возвращает снимок всех метрик, отсортированный по типу и имени.
func (s *MemStorage) List() []model.StoredMetric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make([]model.StoredMetric, 0, len(s.gauges)+len(s.counters))
	for name, value := range s.gauges {
		metrics = append(metrics, model.StoredMetric{
			Name:  name,
			Type:  model.Gauge,
			Value: strconv.FormatFloat(value, 'f', -1, 64),
		})
	}
	for name, value := range s.counters {
		metrics = append(metrics, model.StoredMetric{
			Name:  name,
			Type:  model.Counter,
			Value: strconv.FormatInt(value, 10),
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].Type == metrics[j].Type {
			return metrics[i].Name < metrics[j].Name
		}
		return metrics[i].Type < metrics[j].Type
	})

	return metrics
}

// Snapshot возвращает все метрики в JSON-совместимом виде.
func (s *MemStorage) Snapshot() []model.Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make([]model.Metrics, 0, len(s.gauges)+len(s.counters))
	for name, value := range s.gauges {
		value := value
		metrics = append(metrics, model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &value,
		})
	}
	for name, value := range s.counters {
		value := value
		metrics = append(metrics, model.Metrics{
			ID:    name,
			MType: model.Counter,
			Delta: &value,
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].MType == metrics[j].MType {
			return metrics[i].ID < metrics[j].ID
		}
		return metrics[i].MType < metrics[j].MType
	})

	return metrics
}

// Restore загружает метрики в память, заменяя текущее состояние.
func (s *MemStorage) Restore(metrics []model.Metrics) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges = make(map[string]float64)
	s.counters = make(map[string]int64)

	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			if metric.Value != nil {
				s.gauges[metric.ID] = *metric.Value
			}
		case model.Counter:
			if metric.Delta != nil {
				s.counters[metric.ID] = *metric.Delta
			}
		}
	}
}
