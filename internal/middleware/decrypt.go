package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/safullin/pro_go_1/internal/cryptoutil"
)

// Decrypt расшифровывает тела запросов, отправленные агентом.
func Decrypt(privateKey *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if privateKey == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			algorithm := r.Header.Get(cryptoutil.Header)
			if algorithm == "" {
				next.ServeHTTP(w, r)
				return
			}
			if algorithm != cryptoutil.Algorithm {
				http.Error(w, "unsupported encryption algorithm", http.StatusBadRequest)
				return
			}

			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				http.Error(w, "read encrypted body error", http.StatusBadRequest)
				return
			}

			decrypted, err := cryptoutil.Decrypt(body, privateKey)
			if err != nil {
				http.Error(w, "decrypt request body error", http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(decrypted))
			r.ContentLength = int64(len(decrypted))
			r.Header.Del(cryptoutil.Header)
			next.ServeHTTP(w, r)
		})
	}
}
