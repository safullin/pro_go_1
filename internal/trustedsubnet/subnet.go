package trustedsubnet

import (
	"fmt"
	"net"
)

// Filter проверяет принадлежность IP-адреса доверенной подсети.
type Filter struct {
	subnet *net.IPNet
}

// New создаёт фильтр для подсети в формате CIDR.
func New(cidr string) (Filter, error) {
	if cidr == "" {
		return Filter{}, nil
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return Filter{}, fmt.Errorf("parse trusted subnet: %w", err)
	}
	return Filter{subnet: network}, nil
}

// Allows сообщает, разрешён ли указанный IP-адрес.
func (f Filter) Allows(address string) bool {
	if f.subnet == nil {
		return true
	}
	ip := net.ParseIP(address)
	return ip != nil && f.subnet.Contains(ip)
}
