package middleware

import (
	"fmt"
	"net"
	"net/http"
)

// TrustedSubnet ограничивает доступ адресами из указанной подсети.
func TrustedSubnet(cidr string) (func(http.Handler) http.Handler, error) {
	if cidr == "" {
		return func(next http.Handler) http.Handler { return next }, nil
	}

	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("parse trusted subnet: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := net.ParseIP(r.Header.Get("X-Real-IP"))
			if ip == nil || !subnet.Contains(ip) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
