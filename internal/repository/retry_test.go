package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestRetryPostgresConnectionRetriesConnectionClass(t *testing.T) {
	restoreDelays := setPostgresRetryDelaysForTest()
	defer restoreDelays()

	attempts := 0
	err := retryPostgresConnection(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return &pq.Error{Code: "08006"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry operation: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("unexpected attempts count: got %d want %d", attempts, 3)
	}
}

func TestRetryPostgresConnectionRetriesBadConn(t *testing.T) {
	restoreDelays := setPostgresRetryDelaysForTest()
	defer restoreDelays()

	attempts := 0
	err := retryPostgresConnection(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 2 {
			return driver.ErrBadConn
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry operation: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("unexpected attempts count: got %d want %d", attempts, 2)
	}
}

func TestRetryPostgresConnectionSkipsNonRetriableErrors(t *testing.T) {
	restoreDelays := setPostgresRetryDelaysForTest()
	defer restoreDelays()

	attempts := 0
	wantErr := errors.New("syntax error")
	err := retryPostgresConnection(context.Background(), func(context.Context) error {
		attempts++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if attempts != 1 {
		t.Fatalf("unexpected attempts count: got %d want %d", attempts, 1)
	}
}

func setPostgresRetryDelaysForTest() func() {
	oldDelays := postgresRetryDelays
	postgresRetryDelays = []time.Duration{0, 0, 0}
	return func() {
		postgresRetryDelays = oldDelays
	}
}
