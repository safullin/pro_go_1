package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

const gzipEncoding = "gzip"

type gzipResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	decided     bool
	compressing bool
	gzipWriter  *gzip.Writer
}

// Gzip распаковывает входящие gzip-запросы и сжимает подходящие ответы.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), gzipEncoding) {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			r.Body = &gzipReadCloser{
				Reader: zr,
				body:   r.Body,
			}
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), gzipEncoding) {
			next.ServeHTTP(w, r)
			return
		}

		gzw := &gzipResponseWriter{ResponseWriter: w}
		defer func() {
			_ = gzw.Close()
		}()

		next.ServeHTTP(gzw, r)
	})
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.status = statusCode
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.decided {
		w.prepareHeaders(p)
	}

	if w.compressing {
		return w.gzipWriter.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *gzipResponseWriter) Flush() {
	if !w.decided {
		w.prepareHeaders(nil)
	}

	if w.compressing {
		w.gzipWriter.Flush()
	}

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Close() error {
	if !w.decided {
		w.prepareHeaders(nil)
	}
	if w.compressing {
		return w.gzipWriter.Close()
	}
	return nil
}

func (w *gzipResponseWriter) prepareHeaders(sample []byte) {
	if w.decided {
		return
	}

	if contentType := w.Header().Get("Content-Type"); contentType == "" && len(sample) > 0 {
		w.Header().Set("Content-Type", http.DetectContentType(sample))
	}

	if shouldCompressContentType(w.Header().Get("Content-Type")) {
		w.compressing = true
		w.Header().Set("Content-Encoding", gzipEncoding)
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")
		w.gzipWriter = gzip.NewWriter(w.ResponseWriter)
	}

	if !w.wroteHeader {
		if w.status == 0 {
			w.status = http.StatusOK
		}
		w.ResponseWriter.WriteHeader(w.status)
		w.wroteHeader = true
	}

	w.decided = true
}

func shouldCompressContentType(contentType string) bool {
	return strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/html")
}

type gzipReadCloser struct {
	Reader *gzip.Reader
	body   io.Closer
}

func (r *gzipReadCloser) Read(p []byte) (int, error) {
	return r.Reader.Read(p)
}

func (r *gzipReadCloser) Close() error {
	if r.Reader != nil {
		_ = r.Reader.Close()
	}
	return r.body.Close()
}
