package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/safullin/gophermart/internal/gophermart/auth"
	"github.com/safullin/gophermart/internal/gophermart/model"
	"github.com/safullin/gophermart/internal/gophermart/storage"
)

type fakeStore struct {
	user        model.User
	orders      []model.Order
	withdrawals []model.Withdrawal
	balance     model.Balance
	orderResult int
	withdrawOK  bool
}

func (s *fakeStore) CreateUser(context.Context, string, string) (int64, error) {
	return 7, nil
}

func (s *fakeStore) UserByLogin(context.Context, string) (model.User, error) {
	return s.user, nil
}

func (s *fakeStore) CreateOrder(context.Context, int64, string) (int, error) {
	return s.orderResult, nil
}

func (s *fakeStore) UserOrders(context.Context, int64) ([]model.Order, error) {
	return s.orders, nil
}

func (s *fakeStore) Balance(context.Context, int64) (model.Balance, error) {
	return s.balance, nil
}

func (s *fakeStore) Withdraw(context.Context, int64, string, float64) (bool, error) {
	return s.withdrawOK, nil
}

func (s *fakeStore) Withdrawals(context.Context, int64) ([]model.Withdrawal, error) {
	return s.withdrawals, nil
}

func TestDigitsOnly(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "123", want: true},
		{value: "", want: false},
		{value: "12 3", want: false},
		{value: "abc", want: false},
	}

	for _, tt := range tests {
		if got := digitsOnly(tt.value); got != tt.want {
			t.Fatalf("digitsOnly(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestRegister(t *testing.T) {
	handler := New(&fakeStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"login":"user","password":"pass"}`))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusOK)
	}
	if res.Header().Get("Authorization") == "" {
		t.Fatal("expected authorization header")
	}
	if len(res.Result().Cookies()) == 0 {
		t.Fatal("expected auth cookie")
	}
}

func TestLogin(t *testing.T) {
	hash, err := auth.HashPassword("pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	handler := New(&fakeStore{user: model.User{ID: 7, Login: "user", PasswordHash: hash}})
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"login":"user","password":"pass"}`))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusOK)
	}
}

func TestUploadOrder(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		result     int
		wantStatus int
	}{
		{name: "accepted", body: "12345678903", result: storage.OrderAccepted, wantStatus: http.StatusAccepted},
		{name: "same user", body: "12345678903", result: storage.OrderExistsSameUser, wantStatus: http.StatusOK},
		{name: "other user", body: "12345678903", result: storage.OrderExistsOtherUser, wantStatus: http.StatusConflict},
		{name: "invalid luhn", body: "12345678904", result: storage.OrderAccepted, wantStatus: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New(&fakeStore{orderResult: tt.result})
			req := authRequest(http.MethodPost, "/api/user/orders", tt.body)
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("unexpected status: got %d want %d", res.Code, tt.wantStatus)
			}
		})
	}
}

func TestOrders(t *testing.T) {
	accrual := 42.5
	handler := New(&fakeStore{orders: []model.Order{
		{Number: "12345678903", Status: model.OrderStatusProcessed, Accrual: &accrual, UploadedAt: time.Unix(1, 0)},
	}})
	req := authRequest(http.MethodGet, "/api/user/orders", "")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), `"accrual":42.5`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
}

func TestOrdersNoContent(t *testing.T) {
	handler := New(&fakeStore{})
	req := authRequest(http.MethodGet, "/api/user/orders", "")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusNoContent)
	}
}

func TestBalance(t *testing.T) {
	handler := New(&fakeStore{balance: model.Balance{Current: 10, Withdrawn: 3}})
	req := authRequest(http.MethodGet, "/api/user/balance", "")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), `"current":10`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
}

func TestWithdraw(t *testing.T) {
	handler := New(&fakeStore{withdrawOK: true})
	req := authRequest(http.MethodPost, "/api/user/balance/withdraw", `{"order":"12345678903","sum":7}`)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusOK)
	}
}

func TestWithdrawInsufficientFunds(t *testing.T) {
	handler := New(&fakeStore{})
	req := authRequest(http.MethodPost, "/api/user/balance/withdraw", `{"order":"12345678903","sum":7}`)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusPaymentRequired {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusPaymentRequired)
	}
}

func TestWithdrawals(t *testing.T) {
	handler := New(&fakeStore{withdrawals: []model.Withdrawal{
		{Order: "12345678903", Sum: 7, ProcessedAt: time.Unix(1, 0)},
	}})
	req := authRequest(http.MethodGet, "/api/user/withdrawals", "")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), `"sum":7`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
}

func TestUnauthorized(t *testing.T) {
	handler := New(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", res.Code, http.StatusUnauthorized)
	}
}

func authRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+auth.NewToken(7))
	return req
}
