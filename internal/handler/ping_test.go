package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/safullin/pro_go_1/internal/handler"
)

type fakePinger struct {
	err error
}

func (p fakePinger) PingContext(context.Context) error {
	return p.err
}

func TestPingHandler(t *testing.T) {
	tests := []struct {
		name       string
		pinger     handler.Pinger
		wantStatus int
	}{
		{
			name:       "database ok",
			pinger:     fakePinger{},
			wantStatus: http.StatusOK,
		},
		{
			name:       "database error",
			pinger:     fakePinger{err: errors.New("connection refused")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "database is not configured",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			res := httptest.NewRecorder()

			handler.NewPingHandler(tt.pinger).Ping(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("unexpected status code: got %d want %d", res.Code, tt.wantStatus)
			}
		})
	}
}
