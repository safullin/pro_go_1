package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentStorageSaveAndRestore(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "metrics.json")

	storage := NewPersistentStorage(filePath, false)
	_ = storage.UpdateGauge(context.Background(), "Alloc", 123.456)
	_ = storage.AddCounter(context.Background(), "PollCount", 7)

	if err := storage.Save(); err != nil {
		t.Fatalf("save metrics: %v", err)
	}

	restored := NewPersistentStorage(filePath, false)
	if err := restored.RestoreFromFile(); err != nil {
		t.Fatalf("restore metrics: %v", err)
	}

	if got, ok := restored.GetGauge(context.Background(), "Alloc"); !ok || got != 123.456 {
		t.Fatalf("unexpected restored gauge: got %v ok=%v", got, ok)
	}
	if got, ok := restored.GetCounter(context.Background(), "PollCount"); !ok || got != 7 {
		t.Fatalf("unexpected restored counter: got %v ok=%v", got, ok)
	}
}

func TestPersistentStorageSyncWrites(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "metrics.json")

	storage := NewPersistentStorage(filePath, true)
	_ = storage.UpdateGauge(context.Background(), "Alloc", 42)

	restored := NewPersistentStorage(filePath, false)
	if err := restored.RestoreFromFile(); err != nil {
		t.Fatalf("restore metrics: %v", err)
	}

	if got, ok := restored.GetGauge(context.Background(), "Alloc"); !ok || got != 42 {
		t.Fatalf("unexpected restored gauge: got %v ok=%v", got, ok)
	}
}

func TestPersistentStoragePeriodicSaveOnShutdown(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "metrics.json")

	storage := NewPersistentStorage(filePath, false)
	_ = storage.AddCounter(context.Background(), "PollCount", 3)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		storage.RunPersistencePeriodically(ctx, time.Hour)
	}()

	cancel()
	<-done

	restored := NewPersistentStorage(filePath, false)
	if err := restored.RestoreFromFile(); err != nil {
		t.Fatalf("restore metrics: %v", err)
	}

	if got, ok := restored.GetCounter(context.Background(), "PollCount"); !ok || got != 3 {
		t.Fatalf("unexpected restored counter: got %v ok=%v", got, ok)
	}
}
