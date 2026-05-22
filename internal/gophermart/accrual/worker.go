package accrual

import (
	"context"
	"errors"
	"time"

	"github.com/safullin/gophermart/internal/gophermart/model"
	"github.com/safullin/gophermart/internal/gophermart/storage"
)

type Worker struct {
	storage *storage.Storage
	client  *Client
}

func NewWorker(storage *storage.Storage, client *Client) *Worker {
	return &Worker{storage: storage, client: client}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *Worker) process(ctx context.Context) {
	numbers, err := w.storage.PendingOrders(ctx, 10)
	if err != nil {
		return
	}
	for _, number := range numbers {
		delay := w.processOrder(ctx, number)
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (w *Worker) processOrder(ctx context.Context, number string) time.Duration {
	result, delay, err := w.client.Result(ctx, number)
	if err != nil {
		if errors.Is(err, ErrNoContent) {
			_ = w.storage.UpdateOrder(ctx, number, model.OrderStatusProcessing, nil)
		}
		return delay
	}

	switch result.Status {
	case StatusRegistered, StatusProcessing:
		_ = w.storage.UpdateOrder(ctx, number, model.OrderStatusProcessing, nil)
	case StatusInvalid:
		_ = w.storage.UpdateOrder(ctx, number, model.OrderStatusInvalid, nil)
	case StatusProcessed:
		_ = w.storage.UpdateOrder(ctx, number, model.OrderStatusProcessed, result.Accrual)
	}
	return 0
}
