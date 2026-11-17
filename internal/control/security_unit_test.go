package control

import (
	"context"
	"crypto/tls"
	"testing"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/access"
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

func TestPlatformBypassRejectsUnknownDomain(t *testing.T) {
	s := &Server{}
	if e := s.Authorize(context.Background(), nil, access.Principal{Platform: true}, "business.contact.read", access.Resource{}, true); e == nil {
		t.Fatal("platform bypass accepts an unknown action domain")
	}
}
