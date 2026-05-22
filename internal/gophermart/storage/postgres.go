package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/safullin/gophermart/internal/gophermart/model"
)

const (
	OrderAccepted = iota
	OrderExistsSameUser
	OrderExistsOtherUser
)

type Storage struct {
	db *sql.DB
}

func Open(ctx context.Context, databaseURI string) (*Storage, error) {
	db, err := sql.Open("postgres", databaseURI)
	if err != nil {
		return nil, err
	}
	storage := &Storage{db: db}
	if err := storage.db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := storage.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return storage, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) CreateUser(ctx context.Context, login string, passwordHash string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login,
		passwordHash,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Storage) UserByLogin(ctx context.Context, login string) (model.User, error) {
	var user model.User
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, login, password_hash FROM users WHERE login = $1`,
		login,
	).Scan(&user.ID, &user.Login, &user.PasswordHash)
	return user, err
}

func (s *Storage) CreateOrder(ctx context.Context, userID int64, number string) (int, error) {
	var insertedUserID int64
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO orders (number, user_id, status)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (number) DO NOTHING
		 RETURNING user_id`,
		number,
		userID,
		model.OrderStatusNew,
	).Scan(&insertedUserID)
	if err == nil {
		return OrderAccepted, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	var ownerID int64
	if err := s.db.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE number = $1`, number).Scan(&ownerID); err != nil {
		return 0, err
	}
	if ownerID == userID {
		return OrderExistsSameUser, nil
	}
	return OrderExistsOtherUser, nil
}

func (s *Storage) UserOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT number, status, accrual, uploaded_at
		 FROM orders
		 WHERE user_id = $1
		 ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]model.Order, 0)
	for rows.Next() {
		var order model.Order
		var accrual sql.NullFloat64
		if err := rows.Scan(&order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return nil, err
		}
		order.UserID = userID
		if accrual.Valid {
			order.Accrual = &accrual.Float64
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *Storage) Balance(ctx context.Context, userID int64) (model.Balance, error) {
	var balance model.Balance
	err := s.db.QueryRowContext(
		ctx,
		`SELECT
		    COALESCE((SELECT SUM(accrual) FROM orders WHERE user_id = $1 AND status = $2), 0)
		    - COALESCE((SELECT SUM(amount) FROM withdrawals WHERE user_id = $1), 0) AS current,
		    COALESCE((SELECT SUM(amount) FROM withdrawals WHERE user_id = $1), 0) AS withdrawn`,
		userID,
		model.OrderStatusProcessed,
	).Scan(&balance.Current, &balance.Withdrawn)
	return balance, err
}

func (s *Storage) Withdraw(ctx context.Context, userID int64, order string, sum float64) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(
		ctx,
		`WITH balance AS (
		    SELECT COALESCE((SELECT SUM(accrual) FROM orders WHERE user_id = $1 AND status = $4), 0)
		         - COALESCE((SELECT SUM(amount) FROM withdrawals WHERE user_id = $1), 0) AS current
		)
		INSERT INTO withdrawals (user_id, order_number, amount)
		SELECT $1, $2, $3 FROM balance WHERE current >= $3
		RETURNING id`,
		userID,
		order,
		sum,
		model.OrderStatusProcessed,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Storage) Withdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT order_number, amount, processed_at
		 FROM withdrawals
		 WHERE user_id = $1
		 ORDER BY processed_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	withdrawals := make([]model.Withdrawal, 0)
	for rows.Next() {
		var withdrawal model.Withdrawal
		if err := rows.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	return withdrawals, rows.Err()
}

func (s *Storage) PendingOrders(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT number
		 FROM orders
		 WHERE status IN ($1, $2)
		 ORDER BY uploaded_at
		 LIMIT $3`,
		model.OrderStatusNew,
		model.OrderStatusProcessing,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	numbers := make([]string, 0)
	for rows.Next() {
		var number string
		if err := rows.Scan(&number); err != nil {
			return nil, err
		}
		numbers = append(numbers, number)
	}
	return numbers, rows.Err()
}

func (s *Storage) UpdateOrder(ctx context.Context, number string, status string, accrual *float64) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE orders SET status = $2, accrual = $3 WHERE number = $1`,
		number,
		status,
		accrual,
	)
	return err
}

func IsUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && string(pqErr.Code) == "23505"
}

func (s *Storage) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    number TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    accrual DOUBLE PRECISION,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS orders_user_uploaded_idx ON orders (user_id, uploaded_at DESC);
CREATE INDEX IF NOT EXISTS orders_status_uploaded_idx ON orders (status, uploaded_at);

CREATE TABLE IF NOT EXISTS withdrawals (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_number TEXT NOT NULL,
    amount DOUBLE PRECISION NOT NULL CHECK (amount > 0),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS withdrawals_user_processed_idx ON withdrawals (user_id, processed_at DESC);
`)
	if err != nil {
		return err
	}
	return nil
}
