package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	size   int
	status int
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.size += n
	return n, err
}

// RequestLogger логирует сведения о запросе и ответе.
func RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w}

			next.ServeHTTP(lrw, r)

			if lrw.status == 0 {
				lrw.status = http.StatusOK
			}

			log.Info().
				Str("uri", r.RequestURI).
				Str("method", r.Method).
				Dur("duration", time.Since(start)).
				Msg("request")

			log.Info().
				Int("status", lrw.status).
				Int("size", lrw.size).
				Msg("response")
		})
	}
}
