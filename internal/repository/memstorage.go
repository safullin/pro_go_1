package repository

import "sync"

// MetricsRepository описывает операции обновления метрик.
type MetricsRepository interface {
	UpdateGauge(name string, value float64)
	AddCounter(name string, delta int64)
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
func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges[name] = value
}

// AddCounter добавляет delta к метрике типа counter.
func (s *MemStorage) AddCounter(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters[name] += delta
}
