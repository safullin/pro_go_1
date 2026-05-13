package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/lib/pq"

	"github.com/safullin/pro_go_1/internal/model"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var postgresRetryDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

// PostgresStorage хранит метрики в PostgreSQL.
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage открывает соединение с PostgreSQL и применяет миграции.
func NewPostgresStorage(ctx context.Context, dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	storage := &PostgresStorage{db: db}
	if err := storage.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migratePostgres(dsn); err != nil {
		_ = db.Close()
		return nil, err
	}

	return storage, nil
}

// Close закрывает соединение с базой данных.
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

// PingContext проверяет соединение с базой данных.
func (s *PostgresStorage) PingContext(ctx context.Context) error {
	return retryPostgresConnection(ctx, func(ctx context.Context) error {
		return s.db.PingContext(ctx)
	})
}

func migratePostgres(dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		_ = db.Close()
		return err
	}

	databaseDriver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		_ = sourceDriver.Close()
		_ = db.Close()
		return err
	}

	migrator, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", databaseDriver)
	if err != nil {
		_ = sourceDriver.Close()
		_ = databaseDriver.Close()
		return err
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

// UpdateGauge заменяет значение метрики типа gauge.
func (s *PostgresStorage) UpdateGauge(name string, value float64) error {
	return retryPostgresConnection(context.Background(), func(ctx context.Context) error {
		_, err := s.db.ExecContext(
			ctx,
			`INSERT INTO metrics (id, type, value, delta)
			 VALUES ($1, $2, $3, NULL)
			 ON CONFLICT (id, type)
			 DO UPDATE SET value = EXCLUDED.value, delta = NULL`,
			name,
			model.Gauge,
			value,
		)
		return err
	})
}

// AddCounter добавляет delta к метрике типа counter.
func (s *PostgresStorage) AddCounter(name string, delta int64) error {
	return retryPostgresConnection(context.Background(), func(ctx context.Context) error {
		_, err := s.db.ExecContext(
			ctx,
			`INSERT INTO metrics (id, type, delta, value)
			 VALUES ($1, $2, $3, NULL)
			 ON CONFLICT (id, type)
			 DO UPDATE SET delta = COALESCE(metrics.delta, 0) + EXCLUDED.delta, value = NULL`,
			name,
			model.Counter,
			delta,
		)
		return err
	})
}

// UpdateMetrics обновляет несколько метрик в одной транзакции.
func (s *PostgresStorage) UpdateMetrics(ctx context.Context, metrics []model.Metrics) ([]model.Metrics, error) {
	var updated []model.Metrics
	err := retryPostgresConnection(ctx, func(ctx context.Context) error {
		var err error
		updated, err = s.updateMetrics(ctx, metrics)
		return err
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *PostgresStorage) updateMetrics(ctx context.Context, metrics []model.Metrics) ([]model.Metrics, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	updated := make([]model.Metrics, 0, len(metrics))
	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			if metric.Value == nil {
				continue
			}

			var value float64
			if err := tx.QueryRowContext(
				ctx,
				`INSERT INTO metrics (id, type, value, delta)
				 VALUES ($1, $2, $3, NULL)
				 ON CONFLICT (id, type)
				 DO UPDATE SET value = EXCLUDED.value, delta = NULL
				 RETURNING value`,
				metric.ID,
				model.Gauge,
				*metric.Value,
			).Scan(&value); err != nil {
				return nil, err
			}

			updated = append(updated, model.Metrics{
				ID:    metric.ID,
				MType: model.Gauge,
				Value: &value,
			})
		case model.Counter:
			if metric.Delta == nil {
				continue
			}

			var value int64
			if err := tx.QueryRowContext(
				ctx,
				`INSERT INTO metrics (id, type, delta, value)
				 VALUES ($1, $2, $3, NULL)
				 ON CONFLICT (id, type)
				 DO UPDATE SET delta = COALESCE(metrics.delta, 0) + EXCLUDED.delta, value = NULL
				 RETURNING delta`,
				metric.ID,
				model.Counter,
				*metric.Delta,
			).Scan(&value); err != nil {
				return nil, err
			}

			updated = append(updated, model.Metrics{
				ID:    metric.ID,
				MType: model.Counter,
				Delta: &value,
			})
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return updated, nil
}

// GetGauge возвращает значение gauge по имени.
func (s *PostgresStorage) GetGauge(name string) (float64, bool) {
	var value float64
	err := retryPostgresConnection(context.Background(), func(ctx context.Context) error {
		return s.db.QueryRowContext(
			ctx,
			`SELECT value FROM metrics WHERE id = $1 AND type = $2`,
			name,
			model.Gauge,
		).Scan(&value)
	})
	if err != nil {
		return 0, false
	}

	return value, true
}

// GetCounter возвращает значение counter по имени.
func (s *PostgresStorage) GetCounter(name string) (int64, bool) {
	var value int64
	err := retryPostgresConnection(context.Background(), func(ctx context.Context) error {
		return s.db.QueryRowContext(
			ctx,
			`SELECT delta FROM metrics WHERE id = $1 AND type = $2`,
			name,
			model.Counter,
		).Scan(&value)
	})
	if err != nil {
		return 0, false
	}

	return value, true
}

// List возвращает снимок всех метрик, отсортированный по типу и имени.
func (s *PostgresStorage) List() []model.StoredMetric {
	var metrics []model.StoredMetric
	err := retryPostgresConnection(context.Background(), func(ctx context.Context) error {
		rows, err := s.db.QueryContext(
			ctx,
			`SELECT id, type, delta, value FROM metrics`,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		result := make([]model.StoredMetric, 0)
		for rows.Next() {
			var (
				name       string
				metricType string
				delta      sql.NullInt64
				value      sql.NullFloat64
			)
			if err := rows.Scan(&name, &metricType, &delta, &value); err != nil {
				return err
			}

			storedMetric := model.StoredMetric{
				Name: name,
				Type: metricType,
			}
			switch metricType {
			case model.Gauge:
				if value.Valid {
					storedMetric.Value = strconv.FormatFloat(value.Float64, 'f', -1, 64)
				}
			case model.Counter:
				if delta.Valid {
					storedMetric.Value = strconv.FormatInt(delta.Int64, 10)
				}
			}

			result = append(result, storedMetric)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		metrics = result
		return nil
	})
	if err != nil {
		return nil
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].Type == metrics[j].Type {
			return metrics[i].Name < metrics[j].Name
		}
		return metrics[i].Type < metrics[j].Type
	})

	return metrics
}

func retryPostgresConnection(ctx context.Context, operation func(context.Context) error) error {
	err := operation(ctx)
	for _, delay := range postgresRetryDelays {
		if err == nil || !isPostgresConnectionError(err) {
			return err
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return err
		}
		err = operation(ctx)
	}
	return err
}

func isPostgresConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return strings.HasPrefix(string(pqErr.Code), "08")
	}

	return false
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
