package repository

import (
	"context"
	"testing"

	"github.com/safullin/pro_go_1/internal/model"
)

func BenchmarkMemStorageUpdateMetrics(b *testing.B) {
	gauge := 100.5
	counter := int64(4)
	metrics := []model.Metrics{
		{ID: "Alloc", MType: model.Gauge, Value: &gauge},
		{ID: "HeapAlloc", MType: model.Gauge, Value: &gauge},
		{ID: "PollCount", MType: model.Counter, Delta: &counter},
		{ID: "RandomValue", MType: model.Gauge, Value: &gauge},
	}
	storage := NewMemStorage()
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		if _, err := storage.UpdateMetrics(ctx, metrics); err != nil {
			b.Fatalf("update metrics: %v", err)
		}
	}
}
