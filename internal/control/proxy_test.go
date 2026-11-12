package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginClientIP(t *testing.T) {
	proxies, e := parseTrustedProxies([]string{"10.0.0.0/24", "2001:db8:1::/48"})
	if e != nil {
		t.Fatal(e)
	}
	s := &Server{trustedProxies: proxies}
	for _, tc := range []struct{ name, peer, forwarded, want string }{
		{"direct ignores spoof", "198.51.100.1:1234", "203.0.113.99", "198.51.100.1"},
		{"proxy", "10.0.0.1:1234", "198.51.100.2", "198.51.100.2"},
		{"chain ignores injected prefix", "10.0.0.1:1234", "203.0.113.99, 198.51.100.2, 10.0.0.2", "198.51.100.2"},
		{"ipv6", "[2001:db8:1::1]:1234", "2001:db8:2::5", "2001:db8:2::5"},
		{"mapped peer", "[::ffff:10.0.0.1]:1234", "::ffff:198.51.100.3", "198.51.100.3"},
		{"missing", "10.0.0.1:1234", "", "10.0.0.1"},
		{"invalid", "10.0.0.1:1234", "unknown, 198.51.100.1", "10.0.0.1"},
		{"port", "10.0.0.1:1234", "198.51.100.1:5678", "10.0.0.1"},
		{"zone", "10.0.0.1:1234", "fe80::1%en0", "10.0.0.1"},
		{"too many hops", "10.0.0.1:1234", strings.Repeat("10.0.0.2,", 32) + "198.51.100.1", "10.0.0.1"},
		{"oversized", "10.0.0.1:1234", strings.Repeat(" ", 2049) + "198.51.100.1", "10.0.0.1"},
		{"invalid peer", "invalid", "198.51.100.1", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/auth/login/password", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			if got := s.loginClientIP(r); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	for _, value := range []string{"*", "proxy.example", "0.0.0.0/0", "::/0", "::ffff:10.0.0.0/120", "fe80::1%en0", ""} {
		if _, e := parseTrustedProxies([]string{value}); e == nil {
			t.Errorf("unsafe proxy configuration accepted: %q", value)
		}
	}
	if _, e := parseTrustedProxies(make([]string, 65)); e == nil {
		t.Fatal("unbounded proxy configuration")
	}
	if _, e := parseTrustedProxies([]string{"127.0.0.1", "::1"}); e != nil {
		t.Fatal("exact proxies rejected", e)
	}
}

func TestPasswordLoginProxyBudgets(t *testing.T) {
	s, c, _, _, _ := foundationFixture(t)
	var e error
	s.trustedProxies, e = parseTrustedProxies([]string{"10.0.0.1"})
	if e != nil {
		t.Fatal(e)
	}
	login := func(peer, forwarded, email string) int {
		// Length rejection follows rate charging. Avoid slow PBKDF2 work so the
		// burst assertions remain independent of machine speed and race overhead.
		b, _ := json.Marshal(Row{"email": email, "password": strings.Repeat("x", 1025)})
		r := httptest.NewRequest("POST", "/auth/login/password", bytes.NewReader(b))
		r.RemoteAddr = peer
		r.Header.Set("Origin", s.Config.PublicURL)
		r.Header.Set("X-Forwarded-For", forwarded)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w.Code
	}
	for i := 1; i <= 13; i++ {
		if got := login("10.0.0.1:1234", fmt.Sprintf("198.51.100.%d", i), fmt.Sprintf("client-%d@example.test", i)); got != 401 {
			t.Fatalf("client %d: %d", i, got)
		}
	}
	// Rotating direct spoofed forwarding headers cannot evade a peer's budget.
	for i := 1; i <= 13; i++ {
		want := 401
		if i == 13 {
			want = 429
		}
		if got := login("203.0.113.1:1234", fmt.Sprintf("192.0.2.%d", i), fmt.Sprintf("spoof-%d@example.test", i)); got != want {
			t.Fatalf("spoof %d: got %d want %d", i, got, want)
		}
	}
	// A distributed attack against one email still spends its shared email budget.
	for i := 1; i <= 9; i++ {
		want := 401
		if i == 9 {
			want = 429
		}
		if got := login("10.0.0.1:1234", fmt.Sprintf("192.0.2.%d", i), "target@example.test"); got != want {
			t.Fatalf("email attempt %d: got %d want %d", i, got, want)
		}
	}
}
