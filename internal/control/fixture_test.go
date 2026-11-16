package control

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// Used only against an explicitly supplied, disposable local CI server. Secrets
// are read/written in private fixture files, never echoed into command arguments.
func TestPrepareFixture(t *testing.T) {
	file := os.Getenv("LASTAPP_PREPARE_FIXTURE_CONFIG")
	if file == "" {
		t.Skip("explicit disposable fixture configuration required")
	}
	cfg, e := LoadConfig(file)
	if e != nil {
		t.Fatal(e)
	}
	if !cfg.Development || cfg.RuntimeAccountHost != "%" {
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
	var state struct {
		Registry    string `json:"registry_password"`
		Provisioner string `json:"provisioner_password"`
		Legacy      string `json:"legacy_password"`
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(file), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &state); e != nil {
		t.Fatal(e)
	}
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
	// Derive mutable tables from the migrated schema so a new table is not
	// silently left without runtime grants.
	tables, e := s.DB.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema='lastapp_control' AND table_type='BASE TABLE' AND table_name<>? ORDER BY table_name", goose.TableName())
	if e != nil {
		t.Fatal(e)
	}
	for tables.Next() {
		var table string
		if e = tables.Scan(&table); e != nil {
			t.Fatal(e)
		}
		if !appendOnlyRegistryTables[table] {
			statements = append(statements, "GRANT UPDATE,DELETE ON `lastapp_control`.`"+table+"` TO 'lastapp_control'@'%'")
		}
	}
	if e = tables.Err(); e != nil {
		t.Fatal(e)
	}
	tables.Close()
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

// scripts/dev.py grants the local runtime account in Python; keep its explicit
// table list equal to the migrated tables minus appendOnlyRegistryTables.
func TestDevelopmentRuntimeGrantsCoverRegistryTables(t *testing.T) {
	want := map[string]bool{}
	created := regexp.MustCompile(`(?i)CREATE TABLE\s+(?:IF NOT EXISTS\s+)?` + "`?" + `([a-z0-9_]+)`)
	names, e := fs.Glob(migrationFiles, "migrations/*.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range names {
		b, e := migrationFiles.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range created.FindAllStringSubmatch(string(b), -1) {
			want[m[1]] = !appendOnlyRegistryTables[m[1]]
		}
	}
	for table := range appendOnlyRegistryTables {
		if _, ok := want[table]; !ok {
			t.Fatalf("append-only table %s is not in the migrations", table)
		}
	}
	script, e := os.ReadFile(filepath.Join("..", "..", "scripts", "dev.py"))
	if e != nil {
		t.Fatal(e)
	}
	list := regexp.MustCompile(`for table in \[([^\]]*)\]`).FindSubmatch(script)
	if list == nil {
		t.Fatal("runtime grant table list not found in scripts/dev.py")
	}
	got := map[string]bool{}
	for _, v := range strings.Split(string(list[1]), ",") {
		got[strings.Trim(strings.TrimSpace(v), "'\"")] = true
	}
	for table, mutable := range want {
		if mutable != got[table] {
			t.Errorf("scripts/dev.py runtime UPDATE,DELETE grant for %s = %v, want %v", table, got[table], mutable)
		}
		delete(got, table)
	}
	for table := range got {
		t.Errorf("scripts/dev.py grants unknown table %s", table)
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
