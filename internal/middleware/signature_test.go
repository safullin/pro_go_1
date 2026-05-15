package middleware_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/safullin/pro_go_1/internal/middleware"
	"github.com/safullin/pro_go_1/internal/signature"
)

func TestSignatureChecksRequestAndSignsResponse(t *testing.T) {
	const key = "secret"
	body := []byte(`{"id":"Alloc","type":"gauge","value":42}`)

	handler := middleware.Signature(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(gotBody) != string(body) {
			t.Fatalf("unexpected request body: got %q want %q", gotBody, body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set(signature.Header, signature.Sum(body, key))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusOK)
	}
	if !signature.Valid(res.Body.Bytes(), key, res.Header().Get(signature.Header)) {
		t.Fatalf("response hash is invalid: %q", res.Header().Get(signature.Header))
	}
}

func TestSignatureRejectsInvalidRequestHash(t *testing.T) {
	const key = "secret"

	handler := middleware.Signature(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called for invalid hash")
	}))

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set(signature.Header, "bad-hash")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d want %d", res.Code, http.StatusBadRequest)
	}
	if !signature.Valid(res.Body.Bytes(), key, res.Header().Get(signature.Header)) {
		t.Fatalf("error response hash is invalid: %q", res.Header().Get(signature.Header))
	}
}
