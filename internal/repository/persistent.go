package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/safullin/pro_go_1/internal/model"
)

// PersistentStorage сохраняет in-memory метрики в файл.
type PersistentStorage struct {
	*MemStorage
	filePath   string
	syncWrites bool
}

// NewPersistentStorage создаёт хранилище с файловой персистентностью.
func NewPersistentStorage(filePath string, syncWrites bool) *PersistentStorage {
	return &PersistentStorage{
		MemStorage: NewMemStorage(),
		filePath:   filePath,
		syncWrites: syncWrites,
	}
}

// UpdateGauge обновляет gauge и при необходимости синхронно сохраняет состояние.
func (s *PersistentStorage) UpdateGauge(name string, value float64) {
	s.MemStorage.UpdateGauge(name, value)
	if s.syncWrites {
		_ = s.Save()
	}
}

// AddCounter обновляет counter и при необходимости синхронно сохраняет состояние.
func (s *PersistentStorage) AddCounter(name string, delta int64) {
	s.MemStorage.AddCounter(name, delta)
	if s.syncWrites {
		_ = s.Save()
	}
}

// Save сохраняет текущие метрики в файл.
func (s *PersistentStorage) Save() error {
	if s.filePath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.Snapshot(), "", "  ")
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(s.filePath), "metrics-*.json")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, s.filePath)
}

// RestoreFromFile восстанавливает метрики из файла, если он существует.
func (s *PersistentStorage) RestoreFromFile() error {
	if s.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var metrics []model.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return err
	}

	s.Restore(metrics)
	return nil
}

// RunPersistencePeriodically периодически сохраняет метрики и делает финальный flush на остановке.
func (s *PersistentStorage) RunPersistencePeriodically(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		<-ctx.Done()
		_ = s.Save()
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = s.Save()
		case <-ctx.Done():
			_ = s.Save()
			return
		}
	}
}
