package control

import (
	"context"
	"slices"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/aantich/lastapp/internal/testdb"
)

// Used only against an explicitly supplied, disposable local CI server. Secrets
// are read/written in private fixture files, never echoed into command arguments.
func TestPrepareFixture(t *testing.T) {
	files := testdb.Load(t, "LASTAPP_PREPARE_FIXTURE_CONFIG")
	cfg := files.Config
	if cfg.RuntimeAccountHost != "%" {
		t.Fatal("expected disposable loopback CI configuration")
	}
	s, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	ctx := context.Background()
	if e = Migrate(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	state := files.State
	// Random token() fixture passwords contain only URL-safe characters.
	for _, v := range []struct{ name, password string }{{"lastapp_control", state.Registry}, {"lastapp_provisioner", state.Provisioner}} {
		if !generatedPassword(v.password) {
			t.Fatal("invalid fixture secret")
		}
		if _, e = s.DB.Exec("CREATE USER '" + v.name + "'@'%' IDENTIFIED BY '" + v.password + "'"); e != nil {
			t.Fatal(e)
		}
	}
	statements := []string{
		"GRANT SELECT,INSERT ON `lastapp_control`.* TO 'lastapp_control'@'%'",
		"GRANT CREATE USER ON *.* TO 'lastapp_provisioner'@'%'",
		"GRANT CREATE,ALTER,INDEX,REFERENCES,SELECT,INSERT,UPDATE,DELETE ON `la\\_%`.* TO 'lastapp_provisioner'@'%' WITH GRANT OPTION",
	}
	// The runtime grants come from the embedded migrations (the same source as
	// `lastapp runtime-grants`); confirm they name exactly the migrated tables.
	tables, e := s.DB.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema='lastapp_control' AND table_type='BASE TABLE' AND table_name<>?", goose.TableName())
	if e != nil {
		t.Fatal(e)
	}
	var migrated []string
	for tables.Next() {
		var table string
		if e = tables.Scan(&table); e != nil {
			t.Fatal(e)
		}
		migrated = append(migrated, table)
	}
	if e = tables.Err(); e != nil {
		t.Fatal(e)
	}
	tables.Close()
	parsed, e := registryTables()
	if e != nil {
		t.Fatal(e)
	}
	// Compare in Go byte order: the server's collation orders "apps" and
	// "app_builds" differently.
	slices.Sort(parsed)
	slices.Sort(migrated)
	if !slices.Equal(parsed, migrated) {
		t.Fatalf("migration-derived tables %v differ from migrated tables %v", parsed, migrated)
	}
	grants, e := RuntimeGrantStatements("lastapp_control", "%")
	if e != nil {
		t.Fatal(e)
	}
	statements = append(statements, grants...)
	for _, statement := range statements {
		if _, e = s.DB.Exec(statement); e != nil {
			t.Fatal(e)
		}
	}
	// Seed with a TCP DSN so development target placement follows the mapped port.
	if e = s.SeedDevelopment(ctx, state.Provisioner); e != nil {
		t.Fatal(e)
	}
}

// RuntimeGrantStatements (printed by `lastapp runtime-grants` for
// scripts/dev.py) grants every migrated table except the append-only set.
func TestRuntimeGrantStatements(t *testing.T) {
	tables, e := registryTables()
	if e != nil {
		t.Fatal(e)
	}
	statements, e := RuntimeGrantStatements("lastapp_foundation", "127.0.0.1")
	if e != nil {
		t.Fatal(e)
	}
	got := map[string]bool{}
	for _, statement := range statements {
		got[statement] = true
	}
	mutable := 0
	for _, table := range tables {
		want := "GRANT UPDATE,DELETE ON `lastapp_foundation`.`" + table + "` TO 'lastapp_control'@'127.0.0.1'"
		if got[want] == appendOnlyRegistryTables[table] {
			t.Errorf("runtime UPDATE,DELETE grant for %s = %v, want %v", table, got[want], !appendOnlyRegistryTables[table])
		}
		if !appendOnlyRegistryTables[table] {
			mutable++
		}
	}
	if len(statements) != mutable || mutable < 20 {
		t.Errorf("got %d grants for %d mutable tables", len(statements), mutable)
	}
	for _, table := range []string{"users", "sessions", "release_deployments"} {
		if !got["GRANT UPDATE,DELETE ON `lastapp_foundation`.`"+table+"` TO 'lastapp_control'@'127.0.0.1'"] {
			t.Errorf("missing runtime grant for %s", table)
		}
	}
	for _, bad := range [][2]string{{"x`; DROP", "%"}, {"lastapp", "a'b"}, {"", "%"}, {"lastapp", ""}} {
		if _, e := RuntimeGrantStatements(bad[0], bad[1]); e == nil {
			t.Errorf("accepted database %q host %q", bad[0], bad[1])
		}
	}
}

func generatedPassword(v string) bool {
	if len(v) < 32 {
		return false
	}
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
