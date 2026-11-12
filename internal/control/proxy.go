package control

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func parseTrustedProxies(values []string) ([]netip.Prefix, error) {
	if len(values) > 64 {
		return nil, fmt.Errorf("trusted_proxies supports at most 64 IP addresses or CIDRs")
	}
	out := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, e := netip.ParsePrefix(value)
		if e != nil {
			ip, err := netip.ParseAddr(value)
			if err != nil || ip.Zone() != "" {
				return nil, fmt.Errorf("trusted_proxies requires IP addresses or CIDRs")
			}
			ip = ip.Unmap()
			prefix = netip.PrefixFrom(ip, ip.BitLen())
		}
		if prefix.Bits() == 0 || prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("trusted_proxies requires explicit proxy ranges, using ordinary IPv4 or IPv6 CIDRs")
		}
		out = append(out, prefix.Masked())
	}
	return out, nil
}

func (s *Server) trustedProxy(ip netip.Addr) bool {
	for _, prefix := range s.trustedProxies {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// Traverse X-Forwarded-For from the direct peer towards the client, stopping at
// the first untrusted hop. A client-supplied prefix cannot override that hop.
// Malformed or oversized chains fall back to the direct peer's shared budget.
func (s *Server) loginClientIP(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return "unknown"
	}
	peer, e := netip.ParseAddr(host)
	if e != nil || peer.Zone() != "" {
		return "unknown"
	}
	peer = peer.Unmap()
	if !s.trustedProxy(peer) {
		return peer.String()
	}
	raw := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	parts := strings.Split(raw, ",")
	if raw == "" || len(raw) > 2048 || len(parts) > 32 {
		return peer.String()
	}
	chain := make([]netip.Addr, len(parts))
	for i, part := range parts {
		ip, e := netip.ParseAddr(strings.TrimSpace(part))
		if e != nil || ip.Zone() != "" {
			return peer.String()
		}
		chain[i] = ip.Unmap()
	}
	client := peer
	for i := len(chain) - 1; i >= 0 && s.trustedProxy(client); i-- {
		client = chain[i]
	}
	return client.String()
}
