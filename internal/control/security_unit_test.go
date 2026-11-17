package control

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/httpapi"
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
	if httpapi.AllowedOrigin(&s.Config, "https://app.example") {
		t.Fatal("untrusted app accepted as admin origin")
	}
	s.Config.Development = true
	s.Config.DevelopmentUIOrigin = "http://127.0.0.1:5173"
	if !httpapi.AllowedOrigin(&s.Config, s.Config.DevelopmentUIOrigin) || httpapi.AllowedOrigin(&s.Config, "http://127.0.0.1:5174") {
		t.Fatal("development allowlist is not exact")
	}
	if httpapi.CookieName(s.Config.PublicURL, "lastapp_session") != "__Host-lastapp_session" {
		t.Fatal("production cookie lacks host prefix")
	}
	for _, origin := range []string{"https://admin.example/#fragment", "https://admin.example?query", "https://user@admin.example", "javascript:alert(1)"} {
		if config.ValidOrigin(origin, false) {
			t.Fatalf("accepted malformed origin %s", origin)
		}
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

func TestPlatformBypassRejectsUnknownDomain(t *testing.T) {
	s := &Server{}
	if e := s.Authorize(context.Background(), nil, access.Principal{Platform: true}, "business.contact.read", access.Resource{}, true); e == nil {
		t.Fatal("platform bypass accepts an unknown action domain")
	}
}
