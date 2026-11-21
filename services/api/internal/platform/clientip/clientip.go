// Package clientip resolves the real client address (ARCHITECTURE AD-16): forwarding headers are honoured only when the TCP peer is a
// trusted proxy, and X-Forwarded-For is read right to left so entries the client prepended can never win.
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// ParseTrusted parses CIDRs or bare IPs. An empty list trusts nobody (the socket address is always used).
func ParseTrusted(list []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, s := range list {
		if !strings.Contains(s, "/") {
			ip := net.ParseIP(s)
			if ip == nil {
				return nil, fmt.Errorf("clientip: %q is not an IP or CIDR", s)
			}
			bits := 128
			if ip.To4() != nil {
				ip, bits = ip.To4(), 32
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("clientip: %q: %v", s, err)
		}
		out = append(out, n)
	}
	return out, nil
}

func contains(nets []*net.IPNet, ip net.IP) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// From returns the client IP for r.
func From(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return host
	}
	if !contains(trusted, peer) {
		return peer.String()
	}
	cur := peer
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(hops[i]))
		if ip == nil {
			break // malformed hop: do not look further left
		}
		cur = ip
		if !contains(trusted, ip) {
			break
		}
	}
	return cur.String()
}
