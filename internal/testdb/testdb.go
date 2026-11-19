// Package testdb locates the disposable test MariaDB for database-backed Go
// tests and refuses everything else. scripts/testdb.py starts that server,
// writes per-run secrets into a temporary directory and names its config.json
// in LASTAPP_TEST_CONFIG. Each layer keeps its own fixtures; this package only
// answers "which server, with which credentials" and guarantees it is never
// the development data (scripts/dev.py: registry lastapp_foundation on port
// 33317).
package testdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/config"
)

const (
	// ConfigEnv names the runtime config.json written by scripts/testdb.py.
	ConfigEnv = "LASTAPP_TEST_CONFIG"
	// Marker is the database scripts/testdb.py creates on every disposable
	// server it starts. A server without it is not a test server.
	Marker = "lastapp_disposable_test"
	// The development stack (scripts/dev.py, docker-compose service mariadb).
	developmentPort     = "33317"
	developmentRegistry = "lastapp_foundation"
)

// Files are the per-run settings of the disposable server: the runtime
// configuration, the administrator configuration (migrate.json) and the
// generated account passwords (state.json), all from one temporary directory.
type Files struct {
	Path   string
	Config config.Config
	Admin  config.Config
	State  struct {
		Registry    string `json:"registry_password"`
		Provisioner string `json:"provisioner_password"`
		Legacy      string `json:"legacy_password"`
	}
}

// Load returns the test configuration named by env (normally ConfigEnv). It
// skips the test when env is unset, and fails it when either DSN or the server
// itself could be the development data.
func Load(t testing.TB, env string) Files {
	t.Helper()
	f := Files{Path: os.Getenv(env)}
	if f.Path == "" {
		t.Skip("set " + env + ": run database tests through scripts/testdb.py (make integration)")
	}
	dir := filepath.Dir(f.Path)
	for _, v := range []struct {
		name string
		into any
	}{{filepath.Base(f.Path), &f.Config}, {"migrate.json", &f.Admin}, {"state.json", &f.State}} {
		b, e := os.ReadFile(filepath.Join(dir, v.name))
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(b, v.into); e != nil {
			t.Fatal(v.name, e)
		}
	}
	if e := guard(f); e != nil {
		t.Fatal("refusing to run against " + f.Path + ": " + e.Error() + ". Database tests use only the disposable server of scripts/testdb.py (make integration).")
	}
	return f
}

// AdminDSN is the administrator account of the test server with no default
// database, for fixtures that create and drop their own databases.
func (f Files) AdminDSN(t testing.TB) *mariadb.Config {
	t.Helper()
	cfg, e := mariadb.ParseDSN(f.Admin.RegistryDSN)
	if e != nil {
		t.Fatal("invalid administrator DSN")
	}
	cfg.DBName = ""
	cfg.ParseTime = true
	return cfg
}

var marked sync.Map // administrator DSN -> error of its marker check

func guard(f Files) error {
	if !f.Config.Development || !f.Admin.Development {
		return fmt.Errorf("not a development-mode configuration")
	}
	for _, dsn := range []string{f.Config.RegistryDSN, f.Admin.RegistryDSN} {
		cfg, e := mariadb.ParseDSN(dsn)
		if e != nil {
			return fmt.Errorf("invalid registry DSN")
		}
		if cfg.Net != "tcp" {
			return fmt.Errorf("registry is reached over %q, not the test server's TCP port", cfg.Net)
		}
		if _, port, e := net.SplitHostPort(cfg.Addr); e != nil || port == developmentPort {
			return fmt.Errorf("registry port %q is the development MariaDB", cfg.Addr)
		}
		if cfg.DBName == developmentRegistry {
			return fmt.Errorf("registry %s is the development registry", cfg.DBName)
		}
	}
	if v, ok := marked.Load(f.Admin.RegistryDSN); ok {
		e, _ := v.(error)
		return e
	}
	e := checkMarker(f.Admin.RegistryDSN)
	marked.Store(f.Admin.RegistryDSN, e)
	return e
}

func checkMarker(dsn string) error {
	cfg, e := mariadb.ParseDSN(dsn)
	if e != nil {
		return e
	}
	cfg.DBName = ""
	connector, e := mariadb.NewConnector(cfg)
	if e != nil {
		return e
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	var n int
	if e = db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", Marker).Scan(&n); e != nil {
		return fmt.Errorf("cannot check the disposable-server marker: %w", e)
	}
	if n != 1 {
		return fmt.Errorf("server has no %s marker database, so it is not a disposable test server", Marker)
	}
	return nil
}
