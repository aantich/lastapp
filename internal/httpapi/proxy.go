package httpapi

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedProxies are the configured reverse-proxy ranges.
type TrustedProxies []netip.Prefix

// ParseTrustedProxies validates the trusted_proxies configuration.
func ParseTrustedProxies(values []string) (TrustedProxies, error) {
	if len(values) > 64 {
		return nil, fmt.Errorf("trusted_proxies supports at most 64 IP addresses or CIDRs")
	}
	out := make(TrustedProxies, 0, len(values))
	for _, value := range values {
		prefix, e := netip.ParsePrefix(value)
		if e != nil {
			ip, err := netip.ParseAddr(value)
			if err != nil {
				return nil, fmt.Errorf("trusted_proxies requires IP addresses or CIDRs: %w", err)
			}
			if ip.Zone() != "" {
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

func (t TrustedProxies) trusted(ip netip.Addr) bool {
	for _, prefix := range t {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP traverses X-Forwarded-For from the direct peer towards the client,
// stopping at the first untrusted hop. A client-supplied prefix cannot override
// that hop. Malformed or oversized chains fall back to the direct peer's shared
// budget.
func (t TrustedProxies) ClientIP(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return "unknown"
	}
	peer, e := netip.ParseAddr(host)
	if e != nil || peer.Zone() != "" {
		return "unknown"
	}
	peer = peer.Unmap()
	if !t.trusted(peer) {
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
	for i := len(chain) - 1; i >= 0 && t.trusted(client); i-- {
		client = chain[i]
	}
	return client.String()
}
