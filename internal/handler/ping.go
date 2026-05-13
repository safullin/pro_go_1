package handler

import "net/http"

// PingService описывает минимальный контракт для проверки БД.
type PingService interface {
	Ping() error
}

// PingHandler отвечает за GET /ping.
type PingHandler struct {
	service PingService
}

func NewPingHandler(service PingService) *PingHandler {
	return &PingHandler{service: service}
}

func (h *PingHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	if err := h.service.Ping(); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
