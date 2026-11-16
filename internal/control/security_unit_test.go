package control

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/registry"
)

func TestProductionRegistryTransport(t *testing.T) {
	for _, suffix := range []string{"", "?tls=false", "?tls=preferred", "?tls=skip-verify"} {
		cfg, e := mariadb.ParseDSN("u:p@tcp(db.example:3306)/registry" + suffix)
		if e != nil {
			t.Fatal(e)
		}
		if registryTransport(cfg, false) == nil {
			t.Fatalf("accepted insecure mode %q", suffix)
		}
	}
	cfg, e := mariadb.ParseDSN("u:p@tcp(db.example:3306)/registry?tls=true")
	if e != nil {
		t.Fatal(e)
	}
	if e = registryTransport(cfg, false); e != nil || cfg.TLS.InsecureSkipVerify || cfg.TLS.MinVersion < tls.VersionTLS12 {
		t.Fatal("verified TLS rejected", e)
	}
	cfg, e = mariadb.ParseDSN("u@unix(/tmp/private.sock)/registry")
	if e != nil {
		t.Fatal(e)
	}
	if e = registryTransport(cfg, false); e != nil {
		t.Fatal(e)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOIDCTransportAndOrigins(t *testing.T) {
	called := false
	transport := secureTransport{roundTripFunc(func(r *http.Request) (*http.Response, error) { called = true; return nil, errors.New("test") })}
	r := httptest.NewRequest("GET", "http://issuer.example/keys", nil)
	if _, e := transport.RoundTrip(r); e == nil || called {
		t.Fatal("insecure OIDC request reached transport")
	}
	s := &Server{Config: config.Config{PublicURL: "https://admin.example"}}
	for _, endpoint := range []string{"http://issuer.example/token", "https://user:password@issuer.example/token", "https://issuer.example/token#fragment"} {
		if secureEndpoint(endpoint) {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	redirect := httptest.NewRequest("GET", "http://issuer.example/redirect", nil)
	if s.oidcClient().CheckRedirect(redirect, []*http.Request{r}) == nil {
		t.Fatal("OIDC HTTPS downgrade allowed")
	}
	if s.allowedOrigin("https://app.example") {
		t.Fatal("untrusted app accepted as admin origin")
	}
	s.Config.Development = true
	s.Config.DevelopmentUIOrigin = "http://127.0.0.1:5173"
	if !s.allowedOrigin(s.Config.DevelopmentUIOrigin) || s.allowedOrigin("http://127.0.0.1:5174") {
		t.Fatal("development allowlist is not exact")
	}
	if s.cookieName("lastapp_session") != "__Host-lastapp_session" {
		t.Fatal("production cookie lacks host prefix")
	}
	for _, origin := range []string{"https://admin.example/#fragment", "https://admin.example?query", "https://user@admin.example", "javascript:alert(1)"} {
		if config.ValidOrigin(origin, false) {
			t.Fatalf("accepted malformed origin %s", origin)
		}
	}
}

func TestAdministrativeLimiterIsBounded(t *testing.T) {
	var l requestLimiter
	now := time.Now()
	for i := 0; i < 120; i++ {
		if !l.allow("user", now) {
			t.Fatal("initial burst rejected")
		}
	}
	if l.allow("user", now) || !l.allow("user", now.Add(time.Second)) {
		t.Fatal("limiter did not enforce/refill")
	}
	for i := 0; i < 10000; i++ {
		l.allow(fmt.Sprint(i), now)
	}
	if len(l.entries) > 10000 || l.allow("overflow", now) {
		t.Fatal("limiter map grew without bound")
	}
	if !l.allow("new", now.Add(2*time.Minute)) {
		t.Fatal("idle entries were not reclaimed")
	}
}

func TestRedactedDiagnostics(t *testing.T) {
	secret := "password-in-server-message"
	fields := fmt.Sprint(diagnostic(&mariadb.MySQLError{Number: 1142, Message: secret}))
	if strings.Contains(fields, secret) || !strings.Contains(fields, "1142") {
		t.Fatal("diagnostics leak secrets or omit SQL number")
	}
	wrapped := fmt.Errorf("registry connection failed: %w", fmt.Errorf("ping: %w", &mariadb.MySQLError{Number: 1045, Message: secret}))
	fields = fmt.Sprint(diagnostic(wrapped))
	if strings.Contains(fields, secret) || strings.Contains(fields, "registry connection") || !strings.Contains(fields, "1045") || !strings.Contains(fields, "*mariadb.MySQLError") {
		t.Fatalf("wrapped diagnostics leak messages or lose the chain: %s", fields)
	}
	if fields = fmt.Sprint(diagnostic(fmt.Errorf("query: %w", context.DeadlineExceeded))); !strings.Contains(fields, "deadline_exceeded") {
		t.Fatalf("diagnostics omit the context state: %s", fields)
	}
	s := &Server{}
	if e := s.Authorize(context.Background(), nil, access.Principal{Platform: true}, "business.contact.read", access.Resource{}, true); e == nil {
		t.Fatal("platform bypass accepts an unknown action domain")
	}
}

func TestPasswordWorkIsBounded(t *testing.T) {
	s := &Server{passwordSlots: make(chan struct{}, 2)}
	first, e := s.passwordWork()
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.passwordWork()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.passwordWork(); e == nil {
		t.Fatal("password work is not bounded")
	}
	first()
	third, e := s.passwordWork()
	if e != nil {
		t.Fatal("password capacity was not released", e)
	}
	second()
	third()
}

func TestWrappedErrorResponses(t *testing.T) {
	secret := "user:hunter2@tcp(db)/registry"
	respond := func(err error) (int, string) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/v1/x", nil)
		r.Header.Set("X-Request-ID", "rid")
		writeError(w, r, err)
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
