package control

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aantich/lastapp/internal/delivery"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// The real worker leader serves Tenants independently: Tenant A's older
// deployment (waiting for an entity publication whose storage is still
// pending) and older workspace (waiting for its storage) do not stop Tenant
// B's deployment and workspace from being processed. Before per-Tenant
// fairness both steps always took the single oldest job and idled while it
// waited, so B never moved.
func TestWorkerTenantFairness(t *testing.T) {
	s, c, _, owner, _ := foundationFixture(t)
	oc := c.as(owner)
	ctx := context.Background()
	s.Config.WorkspaceDatabaseTarget = ""
	type tenantJobs struct{ tenant, space, target, resource, workspace, backing, deployment string }
	setup := func(port int) tenantJobs {
		t.Helper()
		v := tenantJobs{tenant: registry.NewID(), space: registry.NewID(), target: registry.NewID(), resource: registry.NewID(), workspace: registry.NewID(), deployment: registry.NewID()}
		c.want("POST", "/admin/v1/tenants", registry.Row{"id": v.tenant, "slug": "fair-" + v.tenant, "display_name": "Fairness", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
		// A closed local port: nothing here is ever really provisioned.
		if _, e := s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode,managed_enabled,provisioning_credentials_enc) VALUES(?,'Fairness','127.0.0.1',?,'disabled',1,X'00')", v.target, port); e != nil {
			t.Fatal(e)
		}
		if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'fairness_fixture','registered','shared','ready',X'00')", v.resource, v.tenant, v.target); e != nil {
			t.Fatal(e)
		}
		oc.want("POST", "/v1/tenants/"+v.tenant+"/spaces", registry.Row{"id": v.space, "slug": "prod", "display_name": "Production", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": v.resource}}, 0, 201)
		ws := oc.want("POST", "/v1/spaces/"+v.space+"/workspaces", delivery.WorkspaceCreateInput{ID: v.workspace, Name: "Fairness workspace"}, 0, 200)
		v.backing = registry.Str(ws["database_resource_id"])
		return v
	}
	a, b := setup(1), setup(2)

	// Tenant A: storage that never becomes ready (its target is disabled, so
	// provisioning skips it), an older waiting workspace and an older
	// deployment waiting for a queued publication on that storage.
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	// Fixture rows only: the deployments' releases and Apps and the circular
	// entity/version references are irrelevant to job selection.
	if _, e = conn.ExecContext(ctx, "SET SESSION foreign_key_checks=0"); e != nil {
		t.Fatal(e)
	}
	definition := entity.Templates()[0]
	draft, _ := json.Marshal(definition)
	entityID, version := registry.NewID(), registry.NewID()
	for _, q := range []struct {
		query string
		args  []any
	}{
		{"UPDATE database_targets SET state='disabled' WHERE id=?", []any{a.target}},
		{"UPDATE database_resources SET state='pending' WHERE id=?", []any{a.resource}},
		{"UPDATE development_workspaces SET created_at=created_at-INTERVAL 1 HOUR WHERE id=?", []any{a.workspace}},
		{"INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,draft_json,pending_version_id) VALUES(?,?,?,?,?,?,?)", []any{entityID, a.tenant, a.space, definition.Key, definition.Label, draft, version}},
		{"INSERT INTO entity_versions(id,tenant_id,space_id,entity_id,definition_json,definition_hash,created_by,state) VALUES(?,?,?,?,?,?,?,'queued')", []any{version, a.tenant, a.space, entityID, draft, entity.Hash(definition), owner}},
		// Tenant B's workspace storage is ready; its deployment is newer.
		{"UPDATE database_resources SET state='ready' WHERE id=?", []any{b.backing}},
	} {
		if _, e := conn.ExecContext(ctx, q.query, q.args...); e != nil {
			t.Fatal(e)
		}
	}
	plan := func(entities []registry.Row, versions registry.Row) []byte {
		p, _ := json.Marshal(registry.Row{"snapshot": registry.Row{"entities": entities, "roles": []any{}, "apps": []any{}, "microapps": []any{}}, "base_hash": "", "versions": versions})
		return p
	}
	insert := "INSERT INTO release_deployments(id,tenant_id,space_id,release_id,target_app_id,created_by,plan_json,state,created_at) VALUES(?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6)-INTERVAL ? MINUTE)"
	if _, e = conn.ExecContext(ctx, insert, a.deployment, a.tenant, a.space, registry.NewID(), registry.NewID(), owner, plan([]registry.Row{{"id": entityID, "state": registry.StateActive, "definition": definition}}, registry.Row{entityID: version}), "publishing", 60); e != nil {
		t.Fatal(e)
	}
	if _, e = conn.ExecContext(ctx, insert, b.deployment, b.tenant, b.space, registry.NewID(), registry.NewID(), owner, plan([]registry.Row{}, registry.Row{}), "queued", 0); e != nil {
		t.Fatal(e)
	}

	state := func(table, id string) string {
		t.Helper()
		var v string
		if e := s.DB.QueryRow("SELECT state FROM "+table+" WHERE id=?", id).Scan(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	leader, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); s.worker.Run(leader) }()
	defer func() {
		stop()
		<-done
	}()
	deadline := time.Now().Add(15 * time.Second)
	for state("release_deployments", b.deployment) == "queued" || state("development_workspaces", b.workspace) == "preparing" {
		if time.Now().After(deadline) {
			t.Fatalf("Tenant B blocked behind Tenant A: deployment %s, workspace %s", state("release_deployments", b.deployment), state("development_workspaces", b.workspace))
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := state("release_deployments", a.deployment); got != "publishing" {
		t.Fatalf("Tenant A deployment %s, want still waiting in publishing", got)
	}
	if got := state("development_workspaces", a.workspace); got != "preparing" {
		t.Fatalf("Tenant A workspace %s, want still waiting in preparing", got)
	}
	if got := state("entity_versions", version); got != "queued" {
		t.Fatalf("Tenant A publication %s, want queued behind its storage", got)
	}
}
