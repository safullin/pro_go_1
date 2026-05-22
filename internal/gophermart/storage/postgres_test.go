package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/safullin/gophermart/internal/gophermart/model"
)

func newMockStorage(t *testing.T) (*Storage, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	return &Storage{db: db}, mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
		_ = db.Close()
	}
}

func TestCreateUser(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("login", "hash").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))

	id, err := storage.CreateUser(context.Background(), "login", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if id != 7 {
		t.Fatalf("unexpected id: got %d want %d", id, 7)
	}
}

func TestUserByLogin(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	mock.ExpectQuery("SELECT id, login, password_hash FROM users").
		WithArgs("login").
		WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password_hash"}).AddRow(7, "login", "hash"))

	user, err := storage.UserByLogin(context.Background(), "login")
	if err != nil {
		t.Fatalf("user by login: %v", err)
	}
	if user.ID != 7 || user.Login != "login" || user.PasswordHash != "hash" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestCreateOrder(t *testing.T) {
	tests := []struct {
		name       string
		ownerID    int64
		wantResult int
		inserted   bool
	}{
		{name: "accepted", ownerID: 10, inserted: true, wantResult: OrderAccepted},
		{name: "same user", ownerID: 10, wantResult: OrderExistsSameUser},
		{name: "other user", ownerID: 11, wantResult: OrderExistsOtherUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, mock, done := newMockStorage(t)
			defer done()

			query := mock.ExpectQuery("INSERT INTO orders").WithArgs("12345678903", int64(10), model.OrderStatusNew)
			if tt.inserted {
				query.WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(tt.ownerID))
			} else {
				query.WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery("SELECT user_id FROM orders").
					WithArgs("12345678903").
					WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(tt.ownerID))
			}

			result, err := storage.CreateOrder(context.Background(), 10, "12345678903")
			if err != nil {
				t.Fatalf("create order: %v", err)
			}
			if result != tt.wantResult {
				t.Fatalf("unexpected result: got %d want %d", result, tt.wantResult)
			}
		})
	}
}

func TestUserOrders(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	now := time.Now()
	rows := sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}).
		AddRow("12345678903", model.OrderStatusProcessed, 42.5, now).
		AddRow("0", model.OrderStatusProcessing, nil, now.Add(-time.Minute))
	mock.ExpectQuery("SELECT number, status, accrual, uploaded_at").
		WithArgs(int64(7)).
		WillReturnRows(rows)

	orders, err := storage.UserOrders(context.Background(), 7)
	if err != nil {
		t.Fatalf("user orders: %v", err)
	}
	if len(orders) != 2 || orders[0].Accrual == nil || *orders[0].Accrual != 42.5 {
		t.Fatalf("unexpected orders: %#v", orders)
	}
}

func TestBalance(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	mock.ExpectQuery("SELECT").
		WithArgs(int64(7), model.OrderStatusProcessed).
		WillReturnRows(sqlmock.NewRows([]string{"current", "withdrawn"}).AddRow(100.5, 20.0))

	balance, err := storage.Balance(context.Background(), 7)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if balance.Current != 100.5 || balance.Withdrawn != 20 {
		t.Fatalf("unexpected balance: %#v", balance)
	}
}

func TestWithdraw(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	mock.ExpectQuery("WITH balance").
		WithArgs(int64(7), "12345678903", 30.0, model.OrderStatusProcessed).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	ok, err := storage.Withdraw(context.Background(), 7, "12345678903", 30)
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if !ok {
		t.Fatal("expected successful withdraw")
	}
}

func TestWithdrawInsufficientFunds(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	mock.ExpectQuery("WITH balance").
		WithArgs(int64(7), "12345678903", 30.0, model.OrderStatusProcessed).
		WillReturnError(sql.ErrNoRows)

	ok, err := storage.Withdraw(context.Background(), 7, "12345678903", 30)
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if ok {
		t.Fatal("expected insufficient funds")
	}
}

func TestWithdrawals(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	now := time.Now()
	mock.ExpectQuery("SELECT order_number, amount, processed_at").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"order_number", "amount", "processed_at"}).AddRow("12345678903", 30.0, now))

	withdrawals, err := storage.Withdrawals(context.Background(), 7)
	if err != nil {
		t.Fatalf("withdrawals: %v", err)
	}
	if len(withdrawals) != 1 || withdrawals[0].Order != "12345678903" || withdrawals[0].Sum != 30 {
		t.Fatalf("unexpected withdrawals: %#v", withdrawals)
	}
}

func TestPendingOrders(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	mock.ExpectQuery("SELECT number").
		WithArgs(model.OrderStatusNew, model.OrderStatusProcessing, 10).
		WillReturnRows(sqlmock.NewRows([]string{"number"}).AddRow("12345678903").AddRow("0"))

	orders, err := storage.PendingOrders(context.Background(), 10)
	if err != nil {
		t.Fatalf("pending orders: %v", err)
	}
	if len(orders) != 2 || orders[0] != "12345678903" {
		t.Fatalf("unexpected pending orders: %#v", orders)
	}
}

func TestUpdateOrder(t *testing.T) {
	storage, mock, done := newMockStorage(t)
	defer done()

	accrual := 42.5
	mock.ExpectExec("UPDATE orders SET status").
		WithArgs("12345678903", model.OrderStatusProcessed, &accrual).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := storage.UpdateOrder(context.Background(), "12345678903", model.OrderStatusProcessed, &accrual); err != nil {
		t.Fatalf("update order: %v", err)
	}
}
