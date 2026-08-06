package middleware

import (
	"net/http"

	"github.com/safullin/pro_go_1/internal/trustedsubnet"
)

// TrustedSubnet ограничивает доступ адресами из указанной подсети.
func TrustedSubnet(cidr string) (func(http.Handler) http.Handler, error) {
	filter, err := trustedsubnet.New(cidr)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !filter.Allows(r.Header.Get("X-Real-IP")) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
