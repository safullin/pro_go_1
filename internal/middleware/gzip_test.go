package middleware_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/safullin/pro_go_1/internal/middleware"
)

func TestGzipDecompressesRequestBody(t *testing.T) {
	var gotBody string
	handler := middleware.Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(mustGzip(t, []byte(`{"id":"Alloc"}`))))
	req.Header.Set("Content-Encoding", "gzip")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusOK)
	}
	if gotBody != `{"id":"Alloc"}` {
		t.Fatalf("unexpected request body: got %q", gotBody)
	}
}

func TestGzipCompressesJSONResponse(t *testing.T) {
	handler := middleware.Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/value/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("unexpected content encoding: got %q want %q", got, "gzip")
	}
	if body := string(mustGunzip(t, res.Body.Bytes())); body != `{"status":"ok"}` {
		t.Fatalf("unexpected response body: got %q", body)
	}
}

func TestGzipCompressesHTMLResponse(t *testing.T) {
	handler := middleware.Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<h1>metrics</h1>"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("unexpected content encoding: got %q want %q", got, "gzip")
	}
	if body := string(mustGunzip(t, res.Body.Bytes())); body != "<h1>metrics</h1>" {
		t.Fatalf("unexpected response body: got %q", body)
	}
}

func TestGzipSkipsTextPlainResponse(t *testing.T) {
	handler := middleware.Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("plain"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("unexpected content encoding: got %q want empty", got)
	}
	if body := strings.TrimSpace(res.Body.String()); body != "plain" {
		t.Fatalf("unexpected response body: got %q want %q", body, "plain")
	}
}

func mustGzip(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func mustGunzip(t *testing.T, data []byte) []byte {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer zr.Close()

	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzipped body: %v", err)
	}
	return body
}
