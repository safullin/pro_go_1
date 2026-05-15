package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/safullin/pro_go_1/internal/signature"
)

// Signature проверяет подпись входящих данных и подписывает ответы.
func Signature(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldCheckRequestHash(r) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					writeSignedError(w, key, "read request body error", http.StatusBadRequest)
					return
				}
				_ = r.Body.Close()

				if !signature.Valid(body, key, r.Header.Get(signature.Header)) {
					writeSignedError(w, key, "invalid hash", http.StatusBadRequest)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}

			recorder := newSignedResponseWriter(w)
			next.ServeHTTP(recorder, r)
			recorder.WriteSigned(key)
		})
	}
}

func shouldCheckRequestHash(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func writeSignedError(w http.ResponseWriter, key string, message string, code int) {
	body := []byte(message + "\n")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set(signature.Header, signature.Sum(body, key))
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

type signedResponseWriter struct {
	http.ResponseWriter
	body   bytes.Buffer
	status int
}

func newSignedResponseWriter(w http.ResponseWriter) *signedResponseWriter {
	return &signedResponseWriter{ResponseWriter: w}
}

func (w *signedResponseWriter) WriteHeader(statusCode int) {
	if w.status != 0 {
		return
	}
	w.status = statusCode
}

func (w *signedResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *signedResponseWriter) WriteSigned(key string) {
	body := w.body.Bytes()
	w.Header().Set(signature.Header, signature.Sum(body, key))
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(w.status)
	_, _ = w.ResponseWriter.Write(body)
}
