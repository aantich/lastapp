package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/artifact"
	"github.com/aantich/lastapp/internal/delivery"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

// Delivery contracts (workspaces, builds, releases, deployments, starters): console responses and the MCP workspace/release
// tools, including stale, forbidden and not-found failures. The deployment
// worker runs synchronously so job states are deterministic.
func TestDeliveryShapeGolden(t *testing.T) {
	s, oc, p, tenant, _, _ := agentSecurityFixture(t)
	ctx := context.Background()
	s.Config.ArtifactDirectory = t.TempDir()
	rec := newShapeRecorder(t, oc.h)
	var joe string
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='outsider'").Scan(&joe); e != nil {
		t.Fatal(e)
	}
	jc := oc.as(joe)
	oc.want("POST", "/v1/tenants/"+tenant+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	// Production storage is real so the deployment worker completes; workspace
	// databases stay metadata-only (marked ready, never provisioned).
	resource, _ := businessDatabaseFixture(t, s, tenant, true)
	space, prodApp := registry.NewID(), registry.NewID()
	oc.want("POST", "/v1/tenants/"+tenant+"/spaces", registry.Row{"id": space, "slug": "source", "display_name": "Source", "initial_manager_user_id": p.ID, "database": registry.Row{"resource_id": resource}}, 0, 201)
	oc.want("POST", "/v1/tenants/"+tenant+"/spaces/"+space+"/apps", registry.Row{"id": prodApp, "slug": "client", "display_name": "Client", "initial_manager_user_id": p.ID}, 0, 201)
	ready := func(ws registry.Row) {
		t.Helper()
		if _, e := s.DB.Exec("UPDATE development_workspaces SET state='ready' WHERE id=?", ws["id"]); e != nil {
			t.Fatal(e)
		}
		if _, e := s.DB.Exec("UPDATE database_resources SET state='ready' WHERE id=?", ws["database_resource_id"]); e != nil {
			t.Fatal(e)
		}
	}
	devApp := func(workspace any) string {
		t.Helper()
		var app string
		if e := s.DB.QueryRow("SELECT development_id FROM workspace_items WHERE workspace_id=? AND kind='app' AND production_id=?", workspace, prodApp).Scan(&app); e != nil {
			t.Fatal(e)
		}
		return app
	}
	upload := func(label, token string, files []artifact.File, status int) registry.Row {
		t.Helper()
		b, _ := json.Marshal(registry.Row{"entrypoint": "index.html", "files": files})
		r := httptest.NewRequest("POST", "/artifact/v1/upload", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+token)
		w := rec.raw(label, r, status)
		var v registry.Row
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return v
	}
	files := []artifact.File{{Path: "index.html", Content: []byte("<!doctype html><h1>Delivery golden</h1>")}}

	// Console: workspace lifecycle.
	rec.http(oc, "GET /v1/spaces/{space}/workspaces empty", "GET", "/v1/spaces/"+space+"/workspaces", nil, 0, 200)
	ws := rec.http(oc, "POST /v1/spaces/{space}/workspaces", "POST", "/v1/spaces/"+space+"/workspaces", delivery.WorkspaceCreateInput{ID: registry.NewID(), Name: "Golden workspace"}, 0, 200)
	wid := registry.Str(ws["id"])
	rec.http(oc, "POST /v1/spaces/{space}/workspaces invalid", "POST", "/v1/spaces/"+space+"/workspaces", registry.Row{"id": "not-an-id"}, 0, 400)
	rec.http(oc, "POST /v1/spaces/{space}/workspaces on backing space", "POST", "/v1/spaces/"+registry.Str(ws["backing_space_id"])+"/workspaces", delivery.WorkspaceCreateInput{ID: registry.NewID(), Name: "Nested"}, 0, 400)
	rec.http(oc, "GET /v1/workspaces/{workspace} provisioning", "GET", "/v1/workspaces/"+wid, nil, 0, 200)
	rec.http(jc, "GET /v1/workspaces/{workspace} other user", "GET", "/v1/workspaces/"+wid, nil, 0, 404)
	rec.http(oc, "POST /v1/workspaces/{workspace}/retry not failed", "POST", "/v1/workspaces/"+wid+"/retry", registry.Row{}, registry.Integer(ws["revision"]), 409)
	rec.http(oc, "POST /v1/workspaces/{workspace}/archive stale", "POST", "/v1/workspaces/"+wid+"/archive", registry.Row{}, registry.Integer(ws["revision"])+5, 412)
	rec.http(oc, "POST /v1/workspaces/{workspace}/archive without If-Match", "POST", "/v1/workspaces/"+wid+"/archive", registry.Row{}, 0, 428)
	ready(ws)
	rec.http(oc, "GET /v1/workspaces/{workspace} ready", "GET", "/v1/workspaces/"+wid, nil, 0, 200)
	rec.http(oc, "GET /v1/spaces/{space}/workspaces", "GET", "/v1/spaces/"+space+"/workspaces", nil, 0, 200)
	app := devApp(wid)

	// Starters and launch.
	rec.http(oc, "GET /v1/apps/{app}/starter", "GET", "/v1/apps/"+app+"/starter", nil, 0, 200)
	rec.http(oc, "GET /v1/apps/{app}/starter production", "GET", "/v1/apps/"+prodApp+"/starter", nil, 0, 404)
	zip := httptest.NewRequest("GET", "/v1/apps/"+app+"/starter.zip", nil)
	zip.AddCookie(sessionCookie(oc))
	rec.raw("GET /v1/apps/{app}/starter.zip", zip, 200)
	verifier, nonce := registry.Token(32), registry.Token(32)
	launch := delivery.DevelopmentLaunchInput{Challenge: base64.RawURLEncoding.EncodeToString(registry.TokenHash(verifier)), Nonce: nonce, Origin: "http://localhost:5490"}
	rec.http(oc, "POST /v1/apps/{app}/development-launch unapproved origin", "POST", "/v1/apps/"+app+"/development-launch", launch, 0, 400)
	oc.want("PATCH", "/v1/apps/"+app+"/data-access", entity.AppAccess{Origin: "http://localhost:5490", Inherit: true}, 1, 200)
	rec.http(oc, "POST /v1/apps/{app}/development-launch human session", "POST", "/v1/apps/"+app+"/development-launch", launch, 0, 400)

	// Builds.
	ticket := rec.http(oc, "POST /v1/apps/{app}/upload-tickets", "POST", "/v1/apps/"+app+"/upload-tickets", registry.Row{}, 0, 201)
	rec.http(oc, "POST /v1/apps/{app}/upload-tickets production", "POST", "/v1/apps/"+prodApp+"/upload-tickets", registry.Row{}, 0, 404)
	build := upload("POST /artifact/v1/upload", registry.Str(ticket["upload_token"]), files, 201)
	upload("POST /artifact/v1/upload replay", registry.Str(ticket["upload_token"]), files, 200)
	upload("POST /artifact/v1/upload unsafe path", registry.Str(ticket["upload_token"]), []artifact.File{{Path: "../secret", Content: []byte("x")}}, 400)
	upload("POST /artifact/v1/upload invalid ticket", registry.Token(32), files, 401)
	rec.http(oc, "GET /v1/apps/{app}/builds", "GET", "/v1/apps/"+app+"/builds", nil, 0, 200)
	rec.http(jc, "GET /v1/apps/{app}/builds other user", "GET", "/v1/apps/"+app+"/builds", nil, 0, 404)

	// Releases, preview and production deployment.
	capture := delivery.ReleaseCreateInput{Name: "1.0.0", Build: registry.Str(build["id"]), Manifest: delivery.ReleaseManifest{Microapps: map[string]string{}, Dependencies: map[string]string{"react": "19.2.0"}}}
	rec.http(oc, "POST /v1/apps/{app}/releases stale", "POST", "/v1/apps/"+app+"/releases", capture, 9, 412)
	rec.http(oc, "POST /v1/apps/{app}/releases without If-Match", "POST", "/v1/apps/"+app+"/releases", capture, 0, 428)
	rel := rec.http(oc, "POST /v1/apps/{app}/releases", "POST", "/v1/apps/"+app+"/releases", capture, 2, 200)
	relPath := "/v1/releases/" + registry.Str(rel["id"])
	rec.http(oc, "GET /v1/apps/{app}/releases", "GET", "/v1/apps/"+app+"/releases", nil, 0, 200)
	rec.http(oc, "GET /v1/releases/{release}", "GET", relPath, nil, 0, 200)
	rec.http(jc, "GET /v1/releases/{release} other user", "GET", relPath, nil, 0, 403)
	rec.http(oc, "GET /v1/releases/{release} missing", "GET", "/v1/releases/"+registry.NewID(), nil, 0, 404)
	rec.http(oc, "POST /v1/releases/{release}/preview stale", "POST", relPath+"/preview", registry.Row{}, 9, 412)
	rec.http(oc, "POST /v1/releases/{release}/preview", "POST", relPath+"/preview", registry.Row{}, 2, 200)
	rec.http(oc, "GET /v1/apps/{app}/deployment none", "GET", "/v1/apps/"+prodApp+"/deployment", nil, 0, 200)
	rec.http(oc, "POST /v1/releases/{release}/deploy stale", "POST", relPath+"/deploy", registry.Row{}, 9, 412)
	job := rec.http(oc, "POST /v1/releases/{release}/deploy", "POST", relPath+"/deploy", registry.Row{}, 1, 202)
	rec.http(oc, "POST /v1/releases/{release}/deploy while queued", "POST", relPath+"/deploy", registry.Row{}, 1, 409)
	jobPath := "/v1/deployments/" + registry.Str(job["id"])
	rec.http(oc, "GET /v1/deployments/{deployment} queued", "GET", jobPath, nil, 0, 200)
	rec.http(jc, "GET /v1/deployments/{deployment} other user", "GET", jobPath, nil, 0, 403)
	rec.http(oc, "GET /v1/apps/{app}/deployment queued", "GET", "/v1/apps/"+prodApp+"/deployment", nil, 0, 200)
	rec.http(jc, "GET /v1/apps/{app}/deployment other user", "GET", "/v1/apps/"+prodApp+"/deployment", nil, 0, 403)
	for range 8 {
		if e := s.delivery.DeploymentJob(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
		if _, e := s.publisher.Job(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
	}
	deployed := rec.http(oc, "GET /v1/deployments/{deployment} finished", "GET", jobPath, nil, 0, 200)
	if deployed["state"] != "active" {
		t.Fatal("deployment did not complete", deployed)
	}
	rec.http(oc, "GET /v1/apps/{app}/deployment finished", "GET", "/v1/apps/"+prodApp+"/deployment", nil, 0, 200)
	ws = oc.want("GET", "/v1/workspaces/"+wid, nil, 0, 200)
	rec.http(oc, "POST /v1/workspaces/{workspace}/archive", "POST", "/v1/workspaces/"+wid+"/archive", registry.Row{}, registry.Integer(ws["revision"]), 200)

	// MCP: a Space-scoped delegation with development and deploy authority, a
	// sibling delegation of the same user, and one without production.deploy.
	permissions, _ := json.Marshal([]string{"read", "workspaces.write", "workspaces.test", "releases.write", "resources.write", "production.deploy"})
	if _, e := s.DB.Exec("UPDATE agent_grants SET scope_kind='space',scope_id=?,permissions_json=?,limits_json='{}',revision=revision+1 WHERE id=?", space, permissions, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	sibling := func(client string, permissions ...string) access.Principal {
		t.Helper()
		id := registry.NewID()
		b, _ := json.Marshal(permissions)
		if _, e := s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,scope_id,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,?,'Golden sibling',?,'space',?,?,'{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", id, p.ID, client, tenant, space, b, s.mcpURL()); e != nil {
			t.Fatal(e)
		}
		g, e := s.ReadAgentGrant(ctx, s.DB, id)
		if e != nil {
			t.Fatal(e)
		}
		return access.Principal{ID: p.ID, Agent: g}
	}
	var e error
	if p.Agent, e = s.ReadAgentGrant(ctx, s.DB, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	agent := agentFixtureToken(t, s, p)
	other := agentFixtureToken(t, s, sibling("golden-other", "read", "workspaces.write", "workspaces.test", "releases.write"))
	noDeploy := other
	key := registry.NewID()
	create := agentWorkspaceInput{Action: "create", Space: space, Name: "Golden agent workspace", RequestKey: key}
	aws := rec.mcp(s, agent, "lastapp_workspace create", "lastapp_workspace", create, true)
	rec.mcp(s, agent, "lastapp_workspace create replay", "lastapp_workspace", create, true)
	rec.mcp(s, agent, "lastapp_workspace create without space", "lastapp_workspace", agentWorkspaceInput{Action: "create", RequestKey: registry.NewID()}, false)
	rec.mcp(s, agent, "lastapp_workspace unknown action", "lastapp_workspace", agentWorkspaceInput{Action: "delete", RequestKey: registry.NewID()}, false)
	aid := registry.Str(mcpData(aws)["id"])
	rec.mcp(s, agent, "lastapp_get workspace", "lastapp_get", agentReadInput{Resource: "workspace", ID: aid}, true)
	rec.mcp(s, other, "lastapp_get workspace sibling", "lastapp_get", agentReadInput{Resource: "workspace", ID: aid}, false)
	rec.mcp(s, agent, "lastapp_list workspace", "lastapp_list", agentReadInput{Resource: "workspace", Target: &agentTarget{Kind: "space", ID: space}}, true)
	rec.mcp(s, agent, "lastapp_workspace retry not failed", "lastapp_workspace", agentWorkspaceInput{Action: "retry", Workspace: aid, Revision: 1, RequestKey: registry.NewID()}, false)
	rec.mcp(s, agent, "lastapp_workspace archive stale", "lastapp_workspace", agentWorkspaceInput{Action: "archive", Workspace: aid, Revision: 9, RequestKey: registry.NewID()}, false)
	ready(mcpData(aws))
	aapp := devApp(aid)
	rec.mcp(s, agent, "lastapp_get starter", "lastapp_get", agentReadInput{Resource: "starter", ID: aapp}, true)
	oc.want("PATCH", "/v1/apps/"+aapp+"/data-access", entity.AppAccess{Origin: "http://localhost:5491", Inherit: true}, 1, 200)
	launch.Origin = "http://localhost:5491"
	rec.mcp(s, agent, "lastapp_workspace launch", "lastapp_workspace", agentWorkspaceInput{Action: "launch", App: aapp, Launch: &launch, RequestKey: registry.NewID()}, true)
	rec.mcp(s, other, "lastapp_workspace launch sibling", "lastapp_workspace", agentWorkspaceInput{Action: "launch", App: aapp, Launch: &launch, RequestKey: registry.NewID()}, false)
	aticket := rec.mcp(s, agent, "lastapp_release upload_ticket", "lastapp_release", agentReleaseInput{Action: "upload_ticket", App: aapp, RequestKey: registry.NewID()}, true)
	rec.mcp(s, agent, "lastapp_release upload_ticket production", "lastapp_release", agentReleaseInput{Action: "upload_ticket", App: prodApp, RequestKey: registry.NewID()}, false)
	abuild := upload("POST /artifact/v1/upload agent ticket", registry.Str(mcpData(aticket)["upload_token"]), files, 201)
	rec.mcp(s, agent, "lastapp_list build", "lastapp_list", agentReadInput{Resource: "build", Target: &agentTarget{Kind: "app", ID: aapp}}, true)
	acapture := capture
	acapture.Build = registry.Str(abuild["id"])
	rec.mcp(s, agent, "lastapp_release capture stale", "lastapp_release", agentReleaseInput{Action: "capture", App: aapp, Capture: &acapture, Revision: 9, RequestKey: registry.NewID()}, false)
	rec.mcp(s, agent, "lastapp_release capture missing parameters", "lastapp_release", agentReleaseInput{Action: "capture", App: aapp, Revision: 2, RequestKey: registry.NewID()}, false)
	arel := rec.mcp(s, agent, "lastapp_release capture", "lastapp_release", agentReleaseInput{Action: "capture", App: aapp, Capture: &acapture, Revision: 2, RequestKey: registry.NewID()}, true)
	arelID := registry.Str(mcpData(arel)["id"])
	rec.mcp(s, agent, "lastapp_list release", "lastapp_list", agentReadInput{Resource: "release", Target: &agentTarget{Kind: "app", ID: aapp}}, true)
	rec.mcp(s, agent, "lastapp_get release", "lastapp_get", agentReadInput{Resource: "release", ID: arelID}, true)
	rec.mcp(s, other, "lastapp_get release sibling", "lastapp_get", agentReadInput{Resource: "release", ID: arelID}, false)
	rec.mcp(s, agent, "lastapp_release preview", "lastapp_release", agentReleaseInput{Action: "preview", Release: arelID, Revision: 2, RequestKey: registry.NewID()}, true)
	prod := oc.want("GET", "/v1/tenants/"+tenant+"/apps/"+prodApp, nil, 0, 200)
	rec.mcp(s, noDeploy, "lastapp_release deploy without production.deploy", "lastapp_release", agentReleaseInput{Action: "deploy", Release: arelID, Revision: registry.Integer(prod["revision"]), RequestKey: registry.NewID()}, false)
	rec.mcp(s, agent, "lastapp_release deploy stale", "lastapp_release", agentReleaseInput{Action: "deploy", Release: arelID, Revision: registry.Integer(prod["revision"]) + 9, RequestKey: registry.NewID()}, false)
	ajob := rec.mcp(s, agent, "lastapp_release deploy", "lastapp_release", agentReleaseInput{Action: "deploy", Release: arelID, Revision: registry.Integer(prod["revision"]), RequestKey: registry.NewID()}, true)
	rec.mcp(s, agent, "lastapp_get deployment queued", "lastapp_get", agentReadInput{Resource: "deployment", ID: registry.Str(mcpData(ajob)["id"])}, true)
	rec.mcp(s, agent, "lastapp_get app_deployment", "lastapp_get", agentReadInput{Resource: "app_deployment", ID: aapp}, true)
	rec.mcp(s, agent, "lastapp_get deployment missing", "lastapp_get", agentReadInput{Resource: "deployment", ID: registry.NewID()}, false)
	aws2 := rec.mcp(s, agent, "lastapp_get workspace before archive", "lastapp_get", agentReadInput{Resource: "workspace", ID: aid}, true)
	rec.mcp(s, agent, "lastapp_workspace archive", "lastapp_workspace", agentWorkspaceInput{Action: "archive", Workspace: aid, Revision: registry.Integer(mcpData(aws2)["revision"]), RequestKey: registry.NewID()}, true)
	rec.golden("delivery_shapes.golden.txt")
}

// mcpData returns the operation result inside a structured MCP tool result.
func mcpData(v registry.Row) registry.Row {
	if d, ok := v["data"].(map[string]any); ok {
		return d
	}
	return v
}

func sessionCookie(c testClient) *http.Cookie {
	return &http.Cookie{Name: httpapi.CookieName(c.s.Config.PublicURL, "lastapp_session"), Value: c.session}
}
