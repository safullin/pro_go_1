package handler

import (
	"context"
	"net/http"
	"time"
)

// Pinger описывает проверку доступности внешнего хранилища.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// PingHandler проверяет соединение с базой данных.
type PingHandler struct {
	pinger Pinger
}

// NewPingHandler создаёт обработчик проверки соединения с базой данных.
func NewPingHandler(pinger Pinger) *PingHandler {
	return &PingHandler{pinger: pinger}
}

// Ping обрабатывает GET /ping.
func (h *PingHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil {
		http.Error(w, "database is not configured", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.pinger.PingContext(ctx); err != nil {
		http.Error(w, "database ping failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
