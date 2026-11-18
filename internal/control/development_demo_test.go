package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/registry"
)

// disposableRegistry opens a Server on a new, migrated registry beside the
// local development registry whose configuration env names. Its builds go to a
// test directory. Cleanup drops the managed databases and accounts it created
// and the registry. It also returns an administrative Server on the local
// registry and the local provisioner password the development seed needs.
func disposableRegistry(t *testing.T, env, prefix string) (*Server, *Server, string) {
	t.Helper()
	configPath := os.Getenv(env)
	if configPath == "" {
		t.Skip("set " + env + " to local .local/config.json")
	}
	cfg, e := LoadConfig(configPath)
	if e != nil {
		t.Fatal(e)
	}
	if !cfg.Development {
		t.Fatal("requires local development configuration")
	}
	adminCfg, e := LoadConfig(filepath.Join(filepath.Dir(configPath), "migrate.json"))
	if e != nil {
		t.Fatal(e)
	}
	admin, e := Open(adminCfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { admin.DB.Close() })
	name := prefix + strings.ReplaceAll(registry.NewID(), "-", "")
	if _, e = admin.DB.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		t.Fatal(e)
	}
	cfg.RegistryDSN = adminCfg.RegistryDSN[:strings.LastIndex(adminCfg.RegistryDSN, "/")+1] + name
	cfg.ArtifactDirectory = t.TempDir()
	s, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		resources, _ := registry.QueryRows(context.Background(), s.DB, "SELECT id,database_name,LOWER(HEX(ownership_token)) AS ownership_hex FROM database_resources WHERE management_mode='managed'")
		for _, r := range resources {
			database := registry.Str(r["database_name"])
			var comment string
			if err := admin.DB.QueryRow("SELECT SCHEMA_COMMENT FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", database).Scan(&comment); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Error("cleanup lookup failed", err)
				}
				continue
			}
			if databases.GeneratedName.MatchString(database) && comment == "lastapp:"+registry.Str(r["id"])+":"+registry.Str(r["ownership_hex"]) {
				if _, err := admin.DB.Exec("DROP DATABASE `" + database + "`"); err != nil {
					t.Error("database cleanup failed", err)
				}
				if _, err := admin.DB.Exec("DROP USER IF EXISTS '" + database + "'@'" + cfg.RuntimeAccountHost + "'"); err != nil {
					t.Error("account cleanup failed", err)
				}
			}
		}
		s.Close()
		if _, err := admin.DB.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Error("registry cleanup failed", err)
		}
	})
	if e = Migrate(context.Background(), s.DB); e != nil {
		t.Fatal(e)
	}
	var state struct {
		Provisioner string `json:"provisioner_password"`
	}
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(configPath), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &state); e != nil {
		t.Fatal(e)
	}
	return s, admin, state.Provisioner
}

// The demo definitions satisfy the entity contract before any registry check.
func TestNorthstarDemoDefinitions(t *testing.T) {
	for _, d := range demoEntities() {
		if diagnostics := d.Validate(); len(diagnostics) > 0 {
			t.Errorf("%s: %v", d.Key, diagnostics)
		}
	}
}

// The development seed fills Northstar through real operations: published CRM
// entities, a ready workspace and an active production deployment per App, the
// second of which promotes a workspace backend change. A second run is a no-op.
func TestNorthstarDemoSeed(t *testing.T) {
	s, _, provisioner := disposableRegistry(t, "LASTAPP_TEST_CONFIG", "lastapp_demo_")
	ctx := context.Background()
	if e := s.SeedDevelopment(ctx, provisioner); e != nil {
		t.Fatal(e)
	}
	var owner string
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='tenant-admin'").Scan(&owner); e != nil {
		t.Fatal(e)
	}
	for _, id := range demoEntityIDs {
		row, e := registry.QueryRow(ctx, s.DB, "SELECT e.pending_version_id,v.definition_json FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.id=? AND e.space_id=? AND v.state='published'", id, demoSpace)
		if e != nil || row["pending_version_id"] != nil {
			t.Fatal("entity not published", id, e, row)
		}
		d, e := entities.DefOf(row["definition_json"])
		if e != nil {
			t.Fatal(e)
		}
		if _, promoted := d.Field(demoForecastField); promoted != (id == demoDeal) {
			t.Fatal("forecast field not promoted by the Forecasting release only", d.Key)
		}
	}
	var state, name, wsOwner string
	if e := s.DB.QueryRow("SELECT state,display_name,owner_user_id FROM development_workspaces WHERE id=? AND production_space_id=?", demoWorkspace, demoSpace).Scan(&state, &name, &wsOwner); e != nil || state != "ready" || name != "crm-pricing-exp" || wsOwner != owner {
		t.Fatal("workspace", state, name, wsOwner, e)
	}
	for _, app := range []string{demoDealReview, demoForecasting} {
		var deployments, active, selected, by int
		if e := s.DB.QueryRow("SELECT COUNT(*),COALESCE(SUM(d.state='active'),0),COALESCE(SUM(a.active_release_id=d.release_id),0),COALESCE(SUM(d.created_by=?),0) FROM release_deployments d JOIN apps a ON a.id=d.target_app_id WHERE d.target_app_id=?", owner, app).Scan(&deployments, &active, &selected, &by); e != nil || deployments != 1 || active != 1 || selected != 1 || by != 1 {
			t.Fatal("deployment", app, deployments, active, selected, by, e)
		}
	}
	counts := func() string {
		t.Helper()
		var parts []string
		for _, table := range []string{"audit_events", "entity_definitions", "entity_versions", "development_workspaces", "app_builds", "app_releases", "release_deployments", "artifact_uploads", "database_resources"} {
			var n int
			if e := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil {
				t.Fatal(e)
			}
			parts = append(parts, fmt.Sprintf("%s=%d", table, n))
		}
		return strings.Join(parts, " ")
	}
	before := counts()
	if e := s.SeedDevelopment(ctx, provisioner); e != nil {
		t.Fatal(e)
	}
	if after := counts(); after != before {
		t.Fatalf("second seed changed the registry:\n%s\n%s", before, after)
	}
}
