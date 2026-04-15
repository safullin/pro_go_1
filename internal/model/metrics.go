package model

const (
	Counter = "counter"
	Gauge   = "gauge"

	PollCountMetric   = "PollCount"
	RandomValueMetric = "RandomValue"
)

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
