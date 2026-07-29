package middleware_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/safullin/pro_go_1/internal/cryptoutil"
	"github.com/safullin/pro_go_1/internal/middleware"
)

func TestDecrypt(t *testing.T) {
	privateKey := generateRSAKey(t)
	body := bytes.Repeat([]byte("encrypted metrics"), 100)
	encrypted, err := cryptoutil.Encrypt(body, &privateKey.PublicKey)
	if err != nil {
		t.Fatalf("encrypt body: %v", err)
	}

	var received []byte
	handler := middleware.Decrypt(privateKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read decrypted body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(encrypted))
	request.Header.Set(cryptoutil.Header, cryptoutil.Algorithm)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !bytes.Equal(received, body) {
		t.Fatalf("body = %q, want %q", received, body)
	}
}

func TestDecryptRejectsInvalidBody(t *testing.T) {
	privateKey := generateRSAKey(t)
	handler := middleware.Decrypt(privateKey)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler called with invalid encrypted body")
	}))

	request := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader([]byte("invalid")))
	request.Header.Set(cryptoutil.Header, cryptoutil.Algorithm)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestDecryptSkipsPlainBody(t *testing.T) {
	const body = "plain body"
	var received string
	handler := middleware.Decrypt(generateRSAKey(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		received = string(data)
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if received != body {
		t.Fatalf("body = %q, want %q", received, body)
	}
}

func TestDecryptRejectsUnsupportedAlgorithm(t *testing.T) {
	handler := middleware.Decrypt(generateRSAKey(t))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler called with unsupported algorithm")
	}))

	request := httptest.NewRequest(http.MethodPost, "/updates/", nil)
	request.Header.Set(cryptoutil.Header, "unsupported")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}
