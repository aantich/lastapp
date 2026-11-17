package publisher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// storageFixture opens a fresh, empty business database on the local MariaDB
// named by LASTAPP_TEST_CONFIG (its migrate.json administrator account). The
// test owns the database and drops it on cleanup.
func storageFixture(t *testing.T) *sql.DB {
	t.Helper()
	file := os.Getenv("LASTAPP_TEST_CONFIG")
	if file == "" {
		t.Skip("set LASTAPP_TEST_CONFIG for the installed local MariaDB")
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(file), "migrate.json"))
	if e != nil {
		t.Fatal(e)
	}
	var c config.Config
	if e = json.Unmarshal(b, &c); e != nil {
		t.Fatal(e)
	}
	cfg, e := mariadb.ParseDSN(c.RegistryDSN)
	if e != nil {
		t.Fatal("invalid registry DSN")
	}
	cfg.DBName = ""
	cfg.ParseTime = true
	connector, e := mariadb.NewConnector(cfg)
	if e != nil {
		t.Fatal(e)
	}
	admin := sql.OpenDB(connector)
	name := "publisher_test_" + strings.ReplaceAll(registry.NewID(), "-", "")
	if _, e = admin.Exec("CREATE DATABASE `" + name + "`"); e != nil {
		admin.Close()
		t.Fatal(e)
	}
	cfg = cfg.Clone()
	cfg.DBName = name
	if connector, e = mariadb.NewConnector(cfg); e != nil {
		t.Fatal(e)
	}
	db := sql.OpenDB(connector)
	t.Cleanup(func() {
		db.Close()
		if _, e := admin.Exec("DROP DATABASE `" + name + "`"); e != nil {
			t.Error("fixture database cleanup failed", e)
		}
		admin.Close()
	})
	return db
}

func apiStatus(e error) int {
	var ae *registry.APIError
	if errors.As(e, &ae) {
		return ae.Status
	}
	return 0
}

// TestSupportTablesOwnership: support tables are created once under the
// registry marker and never adopted from another owner.
func TestSupportTablesOwnership(t *testing.T) {
	db, ctx := storageFixture(t), context.Background()
	s := &Publisher{}
	if e := s.ensureSupport(ctx, db, "lastapp:one"); e != nil {
		t.Fatal(e)
	}
	if e := s.ensureSupport(ctx, db, "lastapp:one"); e != nil {
		t.Fatal("support tables are not idempotent", e)
	}
	if e := s.ensureSupport(ctx, db, "lastapp:other"); apiStatus(e) != 409 {
		t.Fatal("support tables of another owner were adopted", e)
	}
}

// TestOpenGateFence: the gate opens only for the current attempt, and a retry
// after an already-opened gate (zero changed rows) is accepted.
func TestOpenGateFence(t *testing.T) {
	db, ctx := storageFixture(t), context.Background()
	if e := (&Publisher{}).ensureSupport(ctx, db, "lastapp:gate"); e != nil {
		t.Fatal(e)
	}
	attempt, other := registry.NewID(), registry.NewID()
	v := Version{ID: registry.NewID(), EntityID: registry.NewID(), TenantID: registry.NewID(), SpaceID: registry.NewID(), Attempt: &attempt}
	if _, e := db.Exec("INSERT INTO _lastapp_entity_state(entity_id,tenant_id,space_id,layout_hash,attempt_id) VALUES(?,?,?,'old',?)", v.EntityID, v.TenantID, v.SpaceID, attempt); e != nil {
		t.Fatal(e)
	}
	if e := OpenGate(ctx, db, v, "layout"); e != nil {
		t.Fatal(e)
	}
	if e := OpenGate(ctx, db, v, "layout"); e != nil {
		t.Fatal("reopening the same physical version failed", e)
	}
	// The same attempt cannot rewrite the open version's layout hash.
	if e := OpenGate(ctx, db, v, "other-layout"); apiStatus(e) != 409 {
		t.Fatal("the open version's layout hash was rewritten", e)
	}
	var layout string
	if e := db.QueryRow("SELECT layout_hash FROM _lastapp_entity_state WHERE entity_id=?", v.EntityID).Scan(&layout); e != nil || layout != "layout" {
		t.Fatal("rejected reopen changed the layout", layout, e)
	}
	var open bool
	var version string
	if e := db.QueryRow("SELECT gate_open,version_id FROM _lastapp_entity_state WHERE entity_id=?", v.EntityID).Scan(&open, &version); e != nil || !open || version != v.ID {
		t.Fatal("gate not opened on the version", open, version, e)
	}
	stale := v
	stale.Attempt = &other
	if e := OpenGate(ctx, db, stale, "layout"); apiStatus(e) != 409 {
		t.Fatal("a superseded attempt passed the storage fence", e)
	}
}

// TestReferenceConstraintsAndColumns: references get one foreign key, and the
// generated columns are verified against the definition.
func TestReferenceConstraintsAndColumns(t *testing.T) {
	db, ctx := storageFixture(t), context.Background()
	account, contact := registry.NewID(), registry.NewID()
	accountDef := entity.Definition{Format: 1, Key: "account", Label: "Account", Fields: []entity.Field{{Key: "name", Label: "Name", Type: "text", MaxLength: 80, Required: true}}}
	contactDef := entity.Definition{Format: 1, Key: "contact", Label: "Contact", Fields: []entity.Field{{Key: "name", Label: "Name", Type: "text", MaxLength: 80, Required: true}, {Key: "account", Label: "Account", Type: "reference", Target: account}, {Key: "active", Label: "Active", Type: "boolean"}}}
	for id, d := range map[string]entity.Definition{account: accountDef, contact: contactDef} {
		if _, e := db.Exec(data.SkeletonSQL(id, "lastapp:test:"+id, d)); e != nil {
			t.Fatal(e)
		}
	}
	for range 2 {
		if e := ensureReferenceConstraints(ctx, db, contact, contactDef); e != nil {
			t.Fatal(e)
		}
	}
	var keys int
	if e := db.QueryRow("SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND REFERENCED_TABLE_NAME IS NOT NULL AND COLUMN_NAME='f_account'", strings.Trim(data.Table(contact), "`")).Scan(&keys); e != nil || keys != 1 {
		t.Fatal("reference constraint missing or duplicated", keys, e)
	}
	if e := verifyEntityColumns(ctx, db, contact, contactDef); e != nil {
		t.Fatal(e)
	}
	required := contactDef
	required.Fields = append([]entity.Field{}, contactDef.Fields...)
	required.Fields[2].Required = true
	if e := verifyEntityColumns(ctx, db, contact, required); apiStatus(e) != 409 {
		t.Fatal("nullability drift was not detected", e)
	}
	if _, e := db.Exec("ALTER TABLE " + data.Table(account) + " MODIFY COLUMN `f_name` VARCHAR(80) COLLATE utf8mb4_bin NOT NULL"); e != nil {
		t.Fatal(e)
	}
	if e := verifyEntityColumns(ctx, db, account, accountDef); apiStatus(e) != 409 {
		t.Fatal("a non-binary text collation was accepted", e)
	}
}
