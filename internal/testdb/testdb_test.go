package testdb

import (
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/config"
)

// The development stack's DSNs (scripts/dev.py) are refused before any
// connection; only the marker check of a disposable server may pass.
func TestGuardRefusesDevelopmentData(t *testing.T) {
	disposable := "root@tcp(127.0.0.1:13319)/lastapp_control"
	for name, v := range map[string]struct{ runtime, admin, want string }{
		"development port":        {"lastapp_control:x@tcp(127.0.0.1:33317)/lastapp_foundation", "root:x@tcp(127.0.0.1:33317)/lastapp_foundation", "development MariaDB"},
		"development registry":    {"lastapp_control:x@tcp(127.0.0.1:13319)/lastapp_foundation", disposable, "development registry"},
		"admin on development":    {disposable, "root:x@tcp(127.0.0.1:33317)/lastapp_control", "development MariaDB"},
		"development unix socket": {disposable, "root@unix(/repo/.local/mariadb.sock)/lastapp_foundation", "not the test server"},
	} {
		f := Files{Config: config.Config{Development: true, RegistryDSN: v.runtime}, Admin: config.Config{Development: true, RegistryDSN: v.admin}}
		if e := guard(f); e == nil || !strings.Contains(e.Error(), v.want) {
			t.Errorf("%s: guard = %v, want %q", name, e, v.want)
		}
	}
	f := Files{Config: config.Config{RegistryDSN: disposable}, Admin: config.Config{Development: true, RegistryDSN: disposable}}
	if e := guard(f); e == nil {
		t.Error("accepted a non-development configuration")
	}
}
