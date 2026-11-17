package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/registry"
)

func TestAdministrativeLimiterIsBounded(t *testing.T) {
	var l Limiter
	now := time.Now()
	for i := 0; i < 120; i++ {
		if !l.Allow("user", now) {
			t.Fatal("initial burst rejected")
		}
	}
	if l.Allow("user", now) || !l.Allow("user", now.Add(time.Second)) {
		t.Fatal("limiter did not enforce/refill")
	}
	for i := 0; i < 10000; i++ {
		l.Allow(fmt.Sprint(i), now)
	}
	if len(l.entries) > 10000 || l.Allow("overflow", now) {
		t.Fatal("limiter map grew without bound")
	}
	if !l.Allow("new", now.Add(2*time.Minute)) {
		t.Fatal("idle entries were not reclaimed")
	}
}

func TestRedactedDiagnostics(t *testing.T) {
	secret := "password-in-server-message"
	fields := fmt.Sprint(Diagnostic(&mariadb.MySQLError{Number: 1142, Message: secret}))
	if strings.Contains(fields, secret) || !strings.Contains(fields, "1142") {
		t.Fatal("diagnostics leak secrets or omit SQL number")
	}
	wrapped := fmt.Errorf("registry connection failed: %w", fmt.Errorf("ping: %w", &mariadb.MySQLError{Number: 1045, Message: secret}))
	fields = fmt.Sprint(Diagnostic(wrapped))
	if strings.Contains(fields, secret) || strings.Contains(fields, "registry connection") || !strings.Contains(fields, "1045") || !strings.Contains(fields, "*mariadb.MySQLError") {
		t.Fatalf("wrapped diagnostics leak messages or lose the chain: %s", fields)
	}
	if fields = fmt.Sprint(Diagnostic(fmt.Errorf("query: %w", context.DeadlineExceeded))); !strings.Contains(fields, "deadline_exceeded") {
		t.Fatalf("diagnostics omit the context state: %s", fields)
	}
}

func TestWrappedErrorResponses(t *testing.T) {
	secret := "user:hunter2@tcp(db)/registry"
	respond := func(err error) (int, string) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/v1/x", nil)
		r.Header.Set("X-Request-ID", "rid")
		WriteError(w, r, err)
		return w.Code, w.Body.String()
	}
	if code, body := respond(fmt.Errorf("insert: %w", &mariadb.MySQLError{Number: 1062, Message: secret})); code != 409 || strings.Contains(body, secret) {
		t.Fatalf("wrapped SQL error mapped to %d: %s", code, body)
	}
	if code, body := respond(fmt.Errorf("outer: %w", registry.Bad("Visible validation message."))); code != 400 || !strings.Contains(body, "Visible validation message.") {
		t.Fatalf("wrapped API error mapped to %d: %s", code, body)
	}
	if code, body := respond(fmt.Errorf("connect %s: %w", secret, errors.New(secret))); code != 500 || strings.Contains(body, secret) || strings.Contains(body, "connect") {
		t.Fatalf("internal error response is not generic (%d): %s", code, body)
	}
}

func TestLoginClientIP(t *testing.T) {
	proxies, e := ParseTrustedProxies([]string{"10.0.0.0/24", "2001:db8:1::/48"})
	if e != nil {
		t.Fatal(e)
	}
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
			if got := proxies.ClientIP(r); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	for _, value := range []string{"*", "proxy.example", "0.0.0.0/0", "::/0", "::ffff:10.0.0.0/120", "fe80::1%en0", ""} {
		if _, e := ParseTrustedProxies([]string{value}); e == nil {
			t.Errorf("unsafe proxy configuration accepted: %q", value)
		}
	}
	if _, e := ParseTrustedProxies(make([]string, 65)); e == nil {
		t.Fatal("unbounded proxy configuration")
	}
	if _, e := ParseTrustedProxies([]string{"127.0.0.1", "::1"}); e != nil {
		t.Fatal("exact proxies rejected", e)
	}
}
