package control

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
		"GRANT CREATE,SELECT,INSERT,UPDATE,DELETE ON `la\\_%`.* TO 'lastapp_provisioner'@'%' WITH GRANT OPTION",
	}
	for _, table := range []string{"users", "sessions", "plans", "tenants", "tenant_memberships", "database_targets", "database_resources", "spaces", "apps", "microapp_installations", "groups", "group_memberships", "local_accounts", "roles", "role_permissions", "access_grants"} {
		statements = append(statements, "GRANT UPDATE,DELETE ON `lastapp_control`."+table+" TO 'lastapp_control'@'%'")
	}
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
