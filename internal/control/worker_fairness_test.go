package control

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aantich/lastapp/internal/delivery"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// workerFixture is a registry with Tenants whose storage targets are closed
// local ports, so nothing in it is ever really provisioned, and the real
// worker leader of its Server.
type workerFixture struct {
	t     *testing.T
	s     *Server
	c, oc testClient
	owner string
}

type tenantJobs struct{ tenant, space, target, resource, workspace, backing, deployment string }

func newWorkerFixture(t *testing.T) *workerFixture {
	s, c, _, owner, _ := foundationFixture(t)
	s.Config.WorkspaceDatabaseTarget = ""
	return &workerFixture{t: t, s: s, c: c, oc: c.as(owner), owner: owner}
}

// tenant creates a Tenant with a production Space on its own target (the
// given closed port) and one preparing workspace.
func (f *workerFixture) tenant(port int) tenantJobs {
	t := f.t
	t.Helper()
	v := tenantJobs{tenant: registry.NewID(), space: registry.NewID(), target: registry.NewID(), resource: registry.NewID(), deployment: registry.NewID()}
	f.c.want("POST", "/admin/v1/tenants", registry.Row{"id": v.tenant, "slug": "fair-" + v.tenant, "display_name": "Fairness", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": f.owner}, 0, 201)
	if _, e := f.s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode,managed_enabled,provisioning_credentials_enc) VALUES(?,'Fairness','127.0.0.1',?,'disabled',1,X'00')", v.target, port); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'fairness_fixture','registered','shared','ready',X'00')", v.resource, v.tenant, v.target); e != nil {
		t.Fatal(e)
	}
	f.oc.want("POST", "/v1/tenants/"+v.tenant+"/spaces", registry.Row{"id": v.space, "slug": "prod", "display_name": "Production", "initial_manager_user_id": f.owner, "database": registry.Row{"resource_id": v.resource}}, 0, 201)
	v.workspace, v.backing = f.workspace(v.space)
	return v
}

// workspace creates a preparing workspace and returns it with its storage.
func (f *workerFixture) workspace(space string) (string, string) {
	f.t.Helper()
	id := registry.NewID()
	ws := f.oc.want("POST", "/v1/spaces/"+space+"/workspaces", delivery.WorkspaceCreateInput{ID: id, Name: "Fairness workspace " + id[:8]}, 0, 200)
	return id, registry.Str(ws["database_resource_id"])
}

func (f *workerFixture) exec(q registry.Queryer, query string, args ...any) {
	f.t.Helper()
	if _, e := q.ExecContext(context.Background(), query, args...); e != nil {
		f.t.Fatal(e)
	}
}

func (f *workerFixture) state(table, id string) string {
	f.t.Helper()
	var v string
	if e := f.s.DB.QueryRow("SELECT state FROM "+table+" WHERE id=?", id).Scan(&v); e != nil {
		f.t.Fatal(e)
	}
	return v
}

// lead runs the Server's worker leader until the returned stop is called.
func (f *workerFixture) lead() (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); f.s.worker.Run(ctx) }()
	return func() {
		cancel()
		<-done
	}
}

// waitUntil polls ok for up to 15 seconds and fails with why() otherwise.
func (f *workerFixture) waitUntil(ok func() bool, why func() string) {
	f.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			f.t.Fatal(why())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// The real worker leader serves Tenants independently: Tenant A's older
// deployment (waiting for an entity publication whose storage is still
// pending) and older workspace (waiting for its storage) do not stop Tenant
// B's deployment and workspace from being processed. Before per-Tenant
// fairness both steps always took the single oldest job and idled while it
// waited, so B never moved.
func TestWorkerTenantFairness(t *testing.T) {
	f := newWorkerFixture(t)
	ctx := context.Background()
	a, b := f.tenant(1), f.tenant(2)
	conn, e := f.s.DB.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	// Fixture rows only: the deployments' releases and Apps and the circular
	// entity/version references are irrelevant to job selection.
	f.exec(conn, "SET SESSION foreign_key_checks=0")
	// Tenant A: storage that never becomes ready (its target is disabled, so
	// provisioning skips it), an older waiting workspace and an older
	// deployment waiting for a queued publication on that storage.
	definition := entity.Templates()[0]
	draft, _ := json.Marshal(definition)
	entityID, version := registry.NewID(), registry.NewID()
	f.exec(conn, "UPDATE database_targets SET state='disabled' WHERE id=?", a.target)
	f.exec(conn, "UPDATE database_resources SET state='pending' WHERE id=?", a.resource)
	f.exec(conn, "UPDATE development_workspaces SET created_at=created_at-INTERVAL 1 HOUR WHERE id=?", a.workspace)
	f.exec(conn, "INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,draft_json,pending_version_id) VALUES(?,?,?,?,?,?,?)", entityID, a.tenant, a.space, definition.Key, definition.Label, draft, version)
	f.exec(conn, "INSERT INTO entity_versions(id,tenant_id,space_id,entity_id,definition_json,definition_hash,created_by,state) VALUES(?,?,?,?,?,?,?,'queued')", version, a.tenant, a.space, entityID, draft, entity.Hash(definition), f.owner)
	// Tenant B's workspace storage is ready; its deployment is newer.
	f.exec(conn, "UPDATE database_resources SET state='ready' WHERE id=?", b.backing)
	plan := func(entities []registry.Row, versions registry.Row) []byte {
		p, _ := json.Marshal(registry.Row{"snapshot": registry.Row{"entities": entities, "roles": []any{}, "apps": []any{}, "microapps": []any{}}, "base_hash": "", "versions": versions})
		return p
	}
	insert := "INSERT INTO release_deployments(id,tenant_id,space_id,release_id,target_app_id,created_by,plan_json,state,created_at) VALUES(?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6)-INTERVAL ? MINUTE)"
	f.exec(conn, insert, a.deployment, a.tenant, a.space, registry.NewID(), registry.NewID(), f.owner, plan([]registry.Row{{"id": entityID, "state": registry.StateActive, "definition": definition}}, registry.Row{entityID: version}), "publishing", 60)
	f.exec(conn, insert, b.deployment, b.tenant, b.space, registry.NewID(), registry.NewID(), f.owner, plan([]registry.Row{}, registry.Row{}), "queued", 0)

	stop := f.lead()
	defer stop()
	f.waitUntil(func() bool {
		return f.state("release_deployments", b.deployment) != "queued" && f.state("development_workspaces", b.workspace) != "preparing"
	}, func() string {
		return fmt.Sprintf("Tenant B blocked behind Tenant A: deployment %s, workspace %s", f.state("release_deployments", b.deployment), f.state("development_workspaces", b.workspace))
	})
	if got := f.state("release_deployments", a.deployment); got != "publishing" {
		t.Fatalf("Tenant A deployment %s, want still waiting in publishing", got)
	}
	if got := f.state("development_workspaces", a.workspace); got != "preparing" {
		t.Fatalf("Tenant A workspace %s, want still waiting in preparing", got)
	}
	if got := f.state("entity_versions", version); got != "queued" {
		t.Fatalf("Tenant A publication %s, want queued behind its storage", got)
	}
}

// Workspaces are independent, so within one Tenant an older workspace waiting
// for its storage does not hold back a newer one whose storage is ready.
// Taking only each Tenant's oldest workspace left the newer one preparing.
func TestWorkerWorkspacesIndependent(t *testing.T) {
	f := newWorkerFixture(t)
	a := f.tenant(1)
	newer, storage := f.workspace(a.space)
	// The older workspace's storage stays pending (its target is disabled).
	f.exec(f.s.DB, "UPDATE database_targets SET state='disabled' WHERE id=?", a.target)
	f.exec(f.s.DB, "UPDATE development_workspaces SET created_at=created_at-INTERVAL 1 HOUR WHERE id=?", a.workspace)
	f.exec(f.s.DB, "UPDATE database_resources SET state='ready' WHERE id=?", storage)
	stop := f.lead()
	defer stop()
	f.waitUntil(func() bool { return f.state("development_workspaces", newer) != "preparing" }, func() string {
		return "newer workspace blocked behind the Tenant's waiting workspace"
	})
	if got := f.state("development_workspaces", a.workspace); got != "preparing" {
		t.Fatalf("waiting workspace %s, want still preparing", got)
	}
}

// Background work cannot starve requests: the worker runs on its own capped
// registry pool. With every connection of the request pool checked out (as
// under request load), the worker still claims and finishes jobs, and its own
// pool is bounded. When the worker shared the request pool it stalled here.
func TestWorkerOwnPool(t *testing.T) {
	f := newWorkerFixture(t)
	ctx := context.Background()
	b := f.tenant(2)
	f.s.DB.SetMaxOpenConns(2)
	probe, e := f.s.DB.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer probe.Close()
	held, e := f.s.DB.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	f.exec(probe, "SET SESSION foreign_key_checks=0")
	f.exec(probe, "UPDATE database_resources SET state='ready' WHERE id=?", b.backing)
	f.exec(probe, "INSERT INTO release_deployments(id,tenant_id,space_id,release_id,target_app_id,created_by,plan_json,state) VALUES(?,?,?,?,?,?,'{\"snapshot\":{\"entities\":[]},\"versions\":{}}','queued')", b.deployment, b.tenant, b.space, registry.NewID(), registry.NewID(), f.owner)
	state := func(table, id string) string {
		var v string
		if e := probe.QueryRowContext(ctx, "SELECT state FROM "+table+" WHERE id=?", id).Scan(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	stop := f.lead()
	defer stop()
	f.waitUntil(func() bool {
		return state("release_deployments", b.deployment) != "queued" && state("development_workspaces", b.workspace) != "preparing"
	}, func() string {
		return "worker stalled while the request pool was fully in use"
	})
	if inUse := f.s.DB.Stats().InUse; inUse != 2 {
		t.Fatalf("request pool in use %d, want only the test's 2", inUse)
	}
	if f.s.worker.DB == f.s.DB || f.s.worker.DB.Stats().MaxOpenConnections != 1+2*len(f.s.worker.Steps) {
		t.Fatal("worker pool is not separate and capped")
	}
}
