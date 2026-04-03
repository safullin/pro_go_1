package model

const (
	Counter = "counter"
	Gauge   = "gauge"

	PollCountMetric   = "PollCount"
	RandomValueMetric = "RandomValue"
)

// StoredMetric описывает метрику для вывода на HTML-странице.
type StoredMetric struct {
	Name  string
	Type  string
	Value string
}
