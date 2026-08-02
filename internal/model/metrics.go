package model

const (
	// Counter обозначает метрику типа counter.
	Counter = "counter"
	// Gauge обозначает метрику типа gauge.
	Gauge = "gauge"

	// PollCountMetric содержит число опросов runtime.
	PollCountMetric = "PollCount"
	// RandomValueMetric содержит случайное значение агента.
	RandomValueMetric = "RandomValue"
)

// generate:reset
// Metrics описывает JSON-представление метрики для API.
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

// StoredMetric описывает метрику для вывода на HTML-странице.
type StoredMetric struct {
	Name  string
	Type  string
	Value string
}
