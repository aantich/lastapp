package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aantich/lastapp/internal/artifact"
	"github.com/aantich/lastapp/internal/authz"
)

type releaseFixture struct {
	s                                                     *Server
	c                                                     testClient
	p                                                     Principal
	tenant, space, workspace, backing, app, productionApp string
	release                                               Row
}

// Metadata-only workspaces suffice for management boundaries; the full release
// flow test separately provisions storage and executes real business-record APIs.
func newReleaseFixture(t *testing.T) releaseFixture {
	t.Helper()
	s, c, p, tenant, target, _ := agentSecurityFixture(t)
	ctx := context.Background()
	resource, space, productionApp := newID(), newID(), newID()
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'release_metadata_fixture','registered','shared','ready',X'00')", resource, tenant, target); e != nil {
		t.Fatal(e)
	}
	c.want("POST", "/v1/tenants/"+tenant+"/spaces", Row{"id": space, "slug": "source", "display_name": "Source", "initial_manager_user_id": p.ID, "database": Row{"resource_id": resource}}, 0, 201)
	c.want("POST", "/v1/tenants/"+tenant+"/spaces/"+space+"/apps", Row{"id": productionApp, "slug": "client", "display_name": "Client", "initial_manager_user_id": p.ID}, 0, 201)
	permissions, _ := json.Marshal([]string{"read", "workspaces.write", "workspaces.test", "releases.write", "resources.write"})
	if _, e := s.DB.Exec("UPDATE agent_grants SET scope_kind='space',scope_id=?,permissions_json=?,limits_json='{}',revision=revision+1 WHERE id=?", space, permissions, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	var e error
	p.Agent, e = s.readAgentGrant(ctx, s.DB, p.Agent.ID)
	if e != nil {
		t.Fatal(e)
	}
	ws := c.want("POST", "/v1/spaces/"+space+"/workspaces", workspaceCreateInput{ID: newID(), Name: "Private human workspace"}, 0, 200)
	if _, e = s.DB.Exec("UPDATE development_workspaces SET state='ready' WHERE id=?", ws["id"]); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("UPDATE database_resources SET state='ready' WHERE id=?", ws["database_resource_id"]); e != nil {
		t.Fatal(e)
	}
	var app string
	if e = s.DB.QueryRow("SELECT development_id FROM workspace_items WHERE workspace_id=? AND kind='app' AND production_id=?", ws["id"], productionApp).Scan(&app); e != nil {
		t.Fatal(e)
	}
	s.Config.ArtifactDirectory = t.TempDir()
	ticket := c.want("POST", "/v1/apps/"+app+"/upload-tickets", Row{}, 0, 201)
	upload, _ := json.Marshal(Row{"entrypoint": "index.html", "files": []artifact.File{{Path: "index.html", Content: []byte("<!doctype html><h1>Release fixture</h1>")}}})
	r := httptest.NewRequest("POST", "/artifact/v1/upload", bytes.NewReader(upload))
	r.Header.Set("Authorization", "Bearer "+str(ticket["upload_token"]))
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	var build Row
	if e = json.Unmarshal(w.Body.Bytes(), &build); e != nil || w.Code != 201 {
		t.Fatal("upload", w.Code, e)
	}
	rel := c.want("POST", "/v1/apps/"+app+"/releases", releaseCreateInput{Name: "Private snapshot", Build: str(build["id"]), Manifest: releaseManifest{Microapps: map[string]string{}, Dependencies: map[string]string{}}}, 1, 200)
	return releaseFixture{s, c, p, tenant, space, str(ws["id"]), str(ws["backing_space_id"]), app, productionApp, rel}
}

func TestReleaseWorkspaceOwnership(t *testing.T) {
	f := newReleaseFixture(t)
	ctx := context.Background()
	path := "/v1/releases/" + str(f.release["id"])
	f.c.want("GET", path, nil, 0, 200)
	read := func(p Principal, want int) {
		t.Helper()
		raw, _ := json.Marshal(agentReadInput{Resource: "release", ID: str(f.release["id"])})
		result, e := f.s.toolAgentGet(ctx, p, raw)
		if want == 200 {
			if e != nil || result.(Release).Snapshot.value() == nil {
				t.Fatal("authorized release inspection", e)
			}
			return
		}
		var ae *APIError
		if !errors.As(e, &ae) || ae.Status != want {
			t.Fatalf("release inspection: want %d, got %v", want, e)
		}
	}
	read(f.p, 404) // Same human, but this delegation does not own the workspace.
	var joe string
	if e := f.s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='outsider'").Scan(&joe); e != nil {
		t.Fatal(e)
	}
	f.c.want("POST", "/v1/tenants/"+f.tenant+"/memberships", Row{"user_id": joe}, 0, 201)
	f.c.want("POST", "/v1/tenants/"+f.tenant+"/spaces/"+f.space+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": authz.SpaceManagerRole}, 0, 201)
	f.c.as(joe).want("GET", path, nil, 0, 404)
	if _, e := f.s.DB.Exec("UPDATE development_workspaces SET owner_agent_grant_id=? WHERE id=?", f.p.Agent.ID, f.workspace); e != nil {
		t.Fatal(e)
	}
	read(f.p, 200)
	other := f.p
	grant := newID()
	permissions, _ := json.Marshal(f.p.Agent.Permissions)
	if _, e := f.s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,scope_id,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'other-client','Other client',?,'space',?,?,'{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", grant, f.p.ID, f.tenant, f.space, permissions, f.s.mcpURL()); e != nil {
		t.Fatal(e)
	}
	var e error
	other.Agent, e = f.s.readAgentGrant(ctx, f.s.DB, grant)
	if e != nil {
		t.Fatal(e)
	}
	read(other, 404)
	// Only a completed deployment makes a snapshot production history. A queued
	// or failed job is not permission to inspect another author's private work.
	job := newID()
	if _, e = f.s.DB.Exec("INSERT INTO release_deployments(id,tenant_id,space_id,release_id,target_app_id,created_by,plan_json) VALUES(?,?,?,?,?,?,'{}')", job, f.tenant, f.space, f.release["id"], f.productionApp, f.p.ID); e != nil {
		t.Fatal(e)
	}
	for _, state := range []string{"queued", "failed", "active"} {
		if _, e = f.s.DB.Exec("UPDATE release_deployments SET state=? WHERE id=?", state, job); e != nil {
			t.Fatal(e)
		}
		want := 404
		if state == "active" {
			want = 200
		}
		read(other, want)
		f.c.as(joe).want("GET", path, nil, 0, want)
	}
	ws := f.c.want("GET", "/v1/workspaces/"+f.workspace, nil, 0, 200)
	f.c.want("POST", "/v1/workspaces/"+f.workspace+"/archive", Row{}, integer(ws["revision"]), 200)
	read(other, 200)
	f.c.as(joe).want("GET", path, nil, 0, 200)
	// Production history never authorizes the backing workspace itself.
	if e = f.s.workspaceGuard(ctx, f.s.DB, other, f.backing); e == nil {
		t.Fatal("release history opened a private workspace")
	}
}

func TestWorkspaceAppQuota(t *testing.T) {
	f := newReleaseFixture(t)
	if _, e := f.s.DB.Exec("UPDATE plans p JOIN tenants t ON t.plan_id=p.id SET p.max_apps=1 WHERE t.id=?", f.tenant); e != nil {
		t.Fatal(e)
	}
	create := func(space, id, slug string, want int) Row {
		t.Helper()
		return f.c.want("POST", "/v1/tenants/"+f.tenant+"/spaces/"+space+"/apps", Row{"id": id, "slug": slug, "display_name": slug, "initial_manager_user_id": f.p.ID}, 0, want)
	}
	create(f.space, newID(), "over-production-quota", 409)
	dev := create(f.backing, newID(), "development", 201)
	devPath := "/v1/tenants/" + f.tenant + "/apps/" + str(dev["id"])
	f.c.want("PATCH", devPath, Row{"state": "archived"}, 1, 200)
	f.c.want("PATCH", devPath, Row{"state": "active"}, 2, 200)
	rel := f.c.want("POST", "/v1/apps/"+f.app+"/releases", releaseCreateInput{Name: "Exceeds production capacity", Build: str(f.release["build_id"]), Manifest: releaseManifest{Microapps: map[string]string{}, Dependencies: map[string]string{}}}, 1, 200)
	rejected := f.c.want("POST", "/v1/releases/"+str(rel["id"])+"/deploy", Row{}, 1, 409)
	if !strings.Contains(str(rejected), "production App quota") {
		t.Fatal("deployment rejected for wrong reason", rejected)
	}
	// The independent workspace cap still bounds all stored Apps, even archived
	// ones. Restoring one of those slots must remain possible at that cap.
	for i := 2; i < maxWorkspaceApps; i++ {
		if _, e := f.s.DB.Exec("INSERT INTO apps(id,tenant_id,space_id,slug,display_name,compute_mode) VALUES(?,?,?,?,?,'shared')", newID(), f.tenant, f.backing, fmt.Sprintf("cap-%d", i), "Cap fixture"); e != nil {
			t.Fatal(e)
		}
	}
	create(f.backing, newID(), "over-workspace-cap", 409)
	f.c.want("PATCH", devPath, Row{"state": "archived"}, 3, 200)
	f.c.want("PATCH", devPath, Row{"state": "active"}, 4, 200)
	prodPath := "/v1/tenants/" + f.tenant + "/apps/" + f.productionApp
	f.c.want("PATCH", prodPath, Row{"state": "archived"}, 1, 200)
	create(f.space, newID(), "replacement", 201)
	f.c.want("PATCH", prodPath, Row{"state": "active"}, 2, 409)
}

func TestReleasePreviewConcurrentConfiguration(t *testing.T) {
	f := newReleaseFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, e := f.s.DB.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if e = lockTenant(ctx, tx, f.tenant); e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "/v1/releases/"+str(f.release["id"])+"/preview", strings.NewReader("{}"))
	r = r.WithContext(ctx)
	r.SetPathValue("release", str(f.release["id"]))
	r.Header.Set("If-Match", "1")
	done := make(chan error, 1)
	go func() { done <- f.s.releasePreview(httptest.NewRecorder(), r, Principal{ID: f.p.ID}) }()
	// Wait for the real preview transaction to contend on the Tenant lock. This
	// puts the concurrent change after any unlocked preflight in the old code.
	for {
		var waiting int
		e = f.s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE DB=DATABASE() AND ID<>CONNECTION_ID() AND LOWER(INFO) LIKE '%select id from tenants%' AND LOWER(INFO) LIKE '%for update%'").Scan(&waiting)
		if e != nil {
			t.Fatal(e)
		}
		if waiting > 0 {
			break
		}
		select {
		case early := <-done:
			t.Fatalf("preview did not wait for concurrent mutation: %v", early)
		case <-ctx.Done():
			t.Fatal("preview did not reach the Tenant lock")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO roles(id,tenant_id,space_id,owner_key,scope_kind,domain,code,display_name) VALUES(?,?,?,?,'space','data','new-role','Concurrent role')", newID(), f.tenant, f.backing, "s:"+f.backing); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-done:
	case <-ctx.Done():
		t.Fatal("preview did not resume")
	}
	var ae *APIError
	if !errors.As(e, &ae) || ae.Status != 409 || !strings.Contains(ae.Message, "configuration changed") {
		t.Fatalf("preview accepted a changed snapshot or failed for the wrong reason: %v", e)
	}
	var active int
	if e = f.s.DB.QueryRow("SELECT COUNT(*) FROM apps WHERE id=? AND active_release_id IS NOT NULL", f.app).Scan(&active); e != nil || active != 0 {
		t.Fatal("rejected preview changed client selection", active, e)
	}
}
