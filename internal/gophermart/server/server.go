package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/safullin/gophermart/internal/gophermart/auth"
	"github.com/safullin/gophermart/internal/gophermart/luhn"
	"github.com/safullin/gophermart/internal/gophermart/middleware"
	"github.com/safullin/gophermart/internal/gophermart/model"
	"github.com/safullin/gophermart/internal/gophermart/storage"
)

type Server struct {
	storage store
}

type contextKey string

const userIDKey contextKey = "user_id"

type store interface {
	CreateUser(ctx context.Context, login string, passwordHash string) (int64, error)
	UserByLogin(ctx context.Context, login string) (model.User, error)
	CreateOrder(ctx context.Context, userID int64, number string) (int, error)
	UserOrders(ctx context.Context, userID int64) ([]model.Order, error)
	Balance(ctx context.Context, userID int64) (model.Balance, error)
	Withdraw(ctx context.Context, userID int64, order string, sum float64) (bool, error)
	Withdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
}

func New(storage store) http.Handler {
	s := &Server{storage: storage}
	router := chi.NewRouter()
	router.Use(middleware.Gzip)
	router.Post("/api/user/register", s.register)
	router.Post("/api/user/login", s.login)
	router.Group(func(r chi.Router) {
		r.Use(s.auth)
		r.Post("/api/user/orders", s.uploadOrder)
		r.Get("/api/user/orders", s.orders)
		r.Get("/api/user/balance", s.balance)
		r.Post("/api/user/balance/withdraw", s.withdraw)
		r.Get("/api/user/withdrawals", s.withdrawals)
	})
	return router
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil || req.Login == "" || req.Password == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	userID, err := s.storage.CreateUser(r.Context(), req.Login, hash)
	if storage.IsUniqueViolation(err) {
		http.Error(w, "login already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setAuth(w, userID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil || req.Login == "" || req.Password == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user, err := s.storage.UserByLogin(r.Context(), req.Login)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	setAuth(w, user.ID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) uploadOrder(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	number := strings.TrimSpace(string(body))
	if number == "" || !digitsOnly(number) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !luhn.Valid(number) {
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}

	result, err := s.storage.CreateOrder(r.Context(), userID(r), number)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	switch result {
	case storage.OrderAccepted:
		w.WriteHeader(http.StatusAccepted)
	case storage.OrderExistsSameUser:
		w.WriteHeader(http.StatusOK)
	case storage.OrderExistsOtherUser:
		http.Error(w, "order belongs to another user", http.StatusConflict)
	}
}

func (s *Server) orders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.storage.UserOrders(r.Context(), userID(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]orderResponse, 0, len(orders))
	for _, order := range orders {
		resp = append(resp, orderResponse{
			Number:     order.Number,
			Status:     order.Status,
			Accrual:    order.Accrual,
			UploadedAt: order.UploadedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) balance(w http.ResponseWriter, r *http.Request) {
	balance, err := s.storage.Balance(r.Context(), userID(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, balance)
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
	var req withdrawRequest
	if err := decodeJSON(r, &req); err != nil || req.Order == "" || req.Sum <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !luhn.Valid(req.Order) {
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}

	ok, err := s.storage.Withdraw(r.Context(), userID(r), req.Order, req.Sum)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "not enough points", http.StatusPaymentRequired)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) withdrawals(w http.ResponseWriter, r *http.Request) {
	withdrawals, err := s.storage.Withdrawals(r.Context(), userID(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]withdrawalResponse, 0, len(withdrawals))
	for _, withdrawal := range withdrawals {
		resp = append(resp, withdrawalResponse{
			Order:       withdrawal.Order,
			Sum:         withdrawal.Sum,
			ProcessedAt: withdrawal.ProcessedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if cookie, err := r.Cookie(auth.CookieName); err == nil {
			token = cookie.Value
		}
		if token == "" {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				token = strings.TrimPrefix(header, "Bearer ")
			}
		}

		id, err := auth.UserID(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, id)))
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func setAuth(w http.ResponseWriter, userID int64) {
	token := auth.NewToken(userID)
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Authorization", "Bearer "+token)
}

func userID(r *http.Request) int64 {
	value, _ := r.Context().Value(userIDKey).(int64)
	return value
}

func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type credentialsRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type withdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type orderResponse struct {
	Accrual    *float64 `json:"accrual,omitempty"`
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	UploadedAt string   `json:"uploaded_at"`
}

type withdrawalResponse struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}
