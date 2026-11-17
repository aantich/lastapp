package control

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

func TestAgentToolContracts(t *testing.T) {
	s := &Server{}
	tools := s.agentTools()
	if len(tools) != 16 {
		t.Fatal("unexpected tool surface", len(tools))
	}
	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			schema, e := agentInputSchema(tool.Input)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = schema.Resolve(nil); e != nil {
				t.Fatal(e)
			}
			if len(tool.Description) < 100 {
				t.Fatal("missing agent guidance")
			}
		})
	}
	schema, e := agentInputSchema(agentType[agentEntityInput]())
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := schema.Resolve(nil)
	if e != nil {
		t.Fatal(e)
	}
	var instance any
	encoded, _ := json.Marshal(agentEntityInput{Definition: &entity.Templates()[2], RequestKey: registry.NewID()})
	if e = json.Unmarshal(encoded, &instance); e != nil {
		t.Fatal(e)
	}
	if e = resolved.Validate(instance); e != nil {
		t.Fatal("default/calculated expression pointers are not representable", e)
	}
	for _, raw := range []string{"http://evil.example/callback", "https://u:p@example.com/cb", "file:///tmp/cb", "https://example.com/cb#fragment"} {
		if validAgentRedirect(raw) {
			t.Fatal("unsafe redirect", raw)
		}
	}
	if !agentRedirectMatches("http://127.0.0.1:8790/callback", "http://127.0.0.1:12345/callback") || agentRedirectMatches("http://127.0.0.1:8790/callback", "http://localhost:12345/callback") {
		t.Fatal("loopback redirect mismatch")
	}
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.64.0.2", "::ffff:127.0.0.1", "2001:db8::1", "64:ff9b::7f00:1"} {
		if publicAgentIP(ip) {
			t.Fatal("SSRF address accepted", ip)
		}
	}
}

type agentRoundTripper struct{ token string }

func (a agentRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+a.token)
	return http.DefaultTransport.RoundTrip(clone)
}

func TestAgentManagementMCP(t *testing.T) {
	s, c, _, owner, joe := foundationFixture(t)
	oc := c.as(owner)
	tenant := registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "agent-" + tenant, "display_name": "Agent fixture", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	base := "/v1/tenants/" + tenant
	oc.want("POST", base+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	target, db := registry.NewID(), registry.NewID()
	if _, e := s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode) VALUES (?,'Agent fixture','127.0.0.2',33317,'disabled')", target); e != nil {
		t.Fatal(e)
	}
	enc, e := s.sealer.Seal("database", db, tenant, databases.Credentials{Username: "fixture", Password: registry.Token(32)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES (?,?,?,?,'registered','shared','ready',?)", db, tenant, target, "agent_fixture_"+strings.ReplaceAll(db, "-", ""), enc); e != nil {
		t.Fatal(e)
	}
	space, sibling := registry.NewID(), registry.NewID()
	for i, id := range []string{space, sibling} {
		oc.want("POST", base+"/spaces", registry.Row{"id": id, "slug": []string{"sales", "private"}[i], "display_name": []string{"Sales", "Private"}[i], "initial_manager_user_id": owner, "database": registry.Row{"resource_id": db}}, 0, 201)
	}
	app := registry.NewID()
	oc.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": app, "slug": "sales", "display_name": "Sales", "initial_manager_user_id": owner}, 0, 201)

	verifier := registry.Token(48)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	redirect := "http://127.0.0.1:8790/callback"
	oauthRequest := func(scopes string) string {
		t.Helper()
		q := url.Values{"response_type": {"code"}, "client_id": {"lastapp-development"}, "redirect_uri": {redirect}, "resource": {s.mcpURL()}, "code_challenge_method": {"S256"}, "code_challenge": {challenge}, "state": {"csrf-client-state"}, "scope": {scopes}}
		r := httptest.NewRequest("GET", s.Config.PublicURL+"/oauth/authorize?"+q.Encode(), nil)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		if w.Code != 302 {
			t.Fatal("OAuth authorize", w.Code, w.Body.String())
		}
		u, _ := url.Parse(w.Header().Get("Location"))
		return u.Query().Get("request")
	}
	tokenRequest := func(form url.Values) (int, registry.Row) {
		t.Helper()
		r := httptest.NewRequest("POST", s.Config.PublicURL+"/oauth/token", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		out := registry.Row{}
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatalf("token response %d: %v", w.Code, e)
		}
		return w.Code, out
	}
	connect := func(permissions []string, scopeKind, scopeID string) (*mcp.ClientSession, string, string, string) {
		t.Helper()
		request := oauthRequest(strings.Join(permissions, " "))
		limits := registry.Row{"target_ids": []string{}, "max_databases": 0}
		if slices.Contains(permissions, "databases.provision") {
			limits = registry.Row{"target_ids": []string{"20000000-0000-4000-8000-000000000001"}, "max_databases": 1}
		}
		approved := oc.want("POST", "/v1/agent-access/requests/"+request, registry.Row{"tenant_id": tenant, "scope_kind": scopeKind, "scope_id": scopeID, "permissions": permissions, "limits": limits, "days": 1}, 0, 200)
		u, _ := url.Parse(registry.Str(approved["redirect_url"]))
		code := u.Query().Get("code")
		form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {"lastapp-development"}, "redirect_uri": {redirect}, "resource": {s.mcpURL()}, "code_verifier": {verifier}}
		bad, _ := url.ParseQuery(form.Encode())
		bad.Set("code_verifier", registry.Token(48))
		if status, _ := tokenRequest(bad); status != 400 {
			t.Fatal("bad PKCE accepted")
		}
		status, tokens := tokenRequest(form)
		if status != 200 {
			t.Fatal("exchange", status, tokens)
		}
		if status, _ = tokenRequest(form); status != 400 {
			t.Fatal("authorization code replay")
		}
		server := httptest.NewServer(c.h)
		t.Cleanup(server.Close)
		client := mcp.NewClient(&mcp.Implementation{Name: "lastapp-integration", Version: "1"}, nil)
		session, e := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: agentRoundTripper{registry.Str(tokens["access_token"])}}}, nil)
		if e != nil {
			t.Fatal("MCP connect", e)
		}
		t.Cleanup(func() { session.Close() })
		var grant string
		if e = s.DB.QueryRow("SELECT g.id FROM agent_token_sessions t JOIN agent_grants g ON g.id=t.grant_id WHERE t.access_hash=?", registry.TokenHash(registry.Str(tokens["access_token"]))).Scan(&grant); e != nil {
			t.Fatal(e)
		}
		return session, grant, registry.Str(tokens["access_token"]), registry.Str(tokens["refresh_token"])
	}
	call := func(session *mcp.ClientSession, name string, args any, wantOK bool) registry.Row {
		t.Helper()
		result, e := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if e != nil {
			t.Fatal(name, e)
		}
		b, _ := json.Marshal(result.StructuredContent)
		out := registry.Row{}
		if e = json.Unmarshal(b, &out); e != nil {
			t.Fatal(name, e)
		}
		if out["ok"] != wantOK {
			t.Fatalf("%s: %s", name, b)
		}
		return out
	}
	permissions := []string{"production.deploy", "read", "resources.write", "entities.draft", "entities.check", "entities.publish", "access.manage", "lifecycle.manage"}
	session, grant, accessToken, refresh := connect(permissions, "space", space)
	otherEndpoint := &Server{DB: s.DB, Config: s.Config, agentTokenSlots: make(chan struct{}, agentTokenConcurrent)}
	otherEndpoint.Config.PublicURL = "http://127.0.0.1:8999"
	otherEndpoint.Checker = &access.Checker{Audience: otherEndpoint.mcpURL()}
	foreignRequest := httptest.NewRequest("POST", otherEndpoint.mcpURL(), strings.NewReader(`{}`))
	foreignRequest.Header.Set("Authorization", "Bearer "+accessToken)
	if _, e := otherEndpoint.agentPrincipal(foreignRequest); e == nil {
		t.Fatal("MCP audience crossed endpoint boundary")
	}
	foreignForm := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {"lastapp-development"}, "resource": {otherEndpoint.mcpURL()}}
	foreignRequest = httptest.NewRequest("POST", otherEndpoint.Config.PublicURL+"/oauth/token", strings.NewReader(foreignForm.Encode()))
	foreignRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	foreignResponse := httptest.NewRecorder()
	otherEndpoint.agentToken(foreignResponse, foreignRequest)
	if foreignResponse.Code != 400 {
		t.Fatal("refresh audience crossed endpoint boundary", foreignResponse.Code)
	}
	tools, e := session.ListTools(context.Background(), nil)
	expectedTools := 0
	for _, spec := range s.agentTools() {
		if agentToolVisible(spec, &access.AgentGrant{Permissions: permissions}) {
			expectedTools++
		}
	}
	if e != nil || len(tools.Tools) != expectedTools {
		t.Fatal("tool discovery", e, len(tools.Tools))
	}
	contextResult := call(session, "lastapp_context", registry.Row{}, true)
	if !strings.Contains(registry.Str(contextResult), "shared_components") {
		t.Fatal("missing creation context")
	}
	// Reuse existing entities and identify full drafts without touching records.
	definition := entity.Templates()[0]
	definition.Fields[0].Default = &entity.Expr{Literal: "Default company"}
	definition.Permissions = []entity.Permission{}
	saved := call(session, "lastapp_entity_save", registry.Row{"target": registry.Row{"kind": "space", "id": space}, "definition": definition, "request_key": registry.NewID()}, true)
	value := saved["data"].(map[string]any)
	entityID := registry.Str(value["id"])
	inventory := call(session, "lastapp_list", registry.Row{"resource": "entity", "target": registry.Row{"kind": "space", "id": space}}, true)
	if !strings.Contains(registry.Str(inventory), entityID) || !strings.Contains(registry.Str(inventory), "draft_fields") {
		t.Fatal("entity context missing")
	}
	call(session, "lastapp_get", registry.Row{"resource": "entity", "id": entityID}, true)
	checked := call(session, "lastapp_entity_check", registry.Row{"entity_id": entityID, "cases": []registry.Row{{"operation": "create", "values": registry.Row{}}}}, true)
	if !strings.Contains(registry.Str(checked), "Default company") {
		t.Fatal("synthetic default evaluation failed")
	}
	call(session, "lastapp_list", registry.Row{"resource": "entity", "target": registry.Row{"kind": "space", "id": sibling}}, false)
	call(session, "lastapp_get", registry.Row{"resource": "space", "id": sibling}, false)
	key, id := registry.NewID(), registry.NewID()
	args := registry.Row{"target": registry.Row{"kind": "app", "id": app}, "request_key": key, "microapp": registry.Row{"id": id, "slug": "deals", "display_name": "Deals", "artifact_ref": "", "navigation_order": 0}}
	call(session, "lastapp_create", args, true)
	replay := call(session, "lastapp_create", args, true)
	if !strings.Contains(registry.Str(replay), "replayed") {
		t.Fatal("write retry was not reconciled")
	}
	args["microapp"].(registry.Row)["display_name"] = "Different"
	call(session, "lastapp_create", args, false)
	call(session, "lastapp_update", registry.Row{"resource": "microapp", "id": id, "display_name": "Changed", "expected_revision": 99, "request_key": registry.NewID()}, false)
	call(session, "lastapp_create", registry.Row{"target": registry.Row{"kind": "tenant"}, "request_key": registry.NewID(), "space": registry.Row{"id": registry.NewID(), "slug": "outside", "display_name": "Outside", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": db}}}, false)
	role, assignment := registry.NewID(), registry.NewID()
	call(session, "lastapp_create", registry.Row{"request_key": registry.NewID(), "role": registry.Row{"id": role, "code": "reviewer", "display_name": "Reviewer"}}, true)
	call(session, "lastapp_get", registry.Row{"resource": "role", "id": role}, true)
	call(session, "lastapp_role_assignment", registry.Row{"action": "grant", "id": assignment, "role_id": role, "user_id": joe, "request_key": registry.NewID()}, true)
	call(session, "lastapp_role_assignment", registry.Row{"action": "revoke", "id": assignment, "expected_revision": 1, "request_key": registry.NewID()}, true)
	call(session, "lastapp_lifecycle", registry.Row{"resource": "role", "id": role, "state": "disabled", "expected_revision": 1, "request_key": registry.NewID()}, true)
	call(session, "lastapp_lifecycle", registry.Row{"resource": "role", "id": role, "state": "active", "expected_revision": 2, "request_key": registry.NewID()}, true)
	call(session, "lastapp_app_access_update", registry.Row{"app_id": app, "access": entity.AppAccess{Origin: "http://127.0.0.22:8789", Inherit: true}, "expected_revision": 1, "request_key": registry.NewID()}, true)
	publishArgs := registry.Row{"entity_id": entityID, "expected_revision": 1, "request_key": registry.NewID()}
	publication := call(session, "lastapp_entity_publish", publishArgs, true)["data"].(map[string]any)
	call(session, "lastapp_entity_publish", publishArgs, true)
	call(session, "lastapp_get", registry.Row{"resource": "publication", "entity_id": entityID, "id": publication["id"]}, true)
	readOnly, _, _, _ := connect([]string{"read"}, "space", space)
	listed, e := readOnly.ListTools(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tool := range listed.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatal("read-only grant exposed a mutation", tool.Name)
		}
	}
	appOnly, _, _, _ := connect([]string{"read", "access.manage"}, "app", app)
	call(appOnly, "lastapp_app_access_update", registry.Row{"app_id": app, "access": entity.AppAccess{Origin: "http://127.0.0.23:8789", Inherit: true}, "expected_revision": 2, "request_key": registry.NewID()}, false)
	call(appOnly, "lastapp_get", registry.Row{"resource": "entity", "id": entityID, "target": registry.Row{"kind": "space", "id": space}}, false)
	// MCP credentials and cookies are intentionally different audiences.
	for _, path := range []string{"/v1/tenants/" + tenant + "/spaces", "/data/v1/schema"} {
		r := httptest.NewRequest("GET", s.Config.PublicURL+path, nil)
		r.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal("token crossed audience", path, w.Code)
		}
	}
	r := httptest.NewRequest("POST", s.Config.PublicURL+"/mcp", strings.NewReader(`{}`))
	r.Header.Set("Mcp-Session-Id", grant)
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("session header accepted as identity")
	}
	r = httptest.NewRequest("POST", s.Config.PublicURL+"/mcp", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+accessToken)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("unsafe origin accepted")
	}
	// Expanding a grant invalidates old access; refresh receives only human-approved changes.
	oc.want("PATCH", "/v1/agent-access/"+grant, registry.Row{"permissions": []string{"read"}, "limits": registry.Row{"target_ids": []string{}, "max_databases": 0}, "days": 1}, 1, 200)
	if _, e = s.publisher.Job(context.Background(), s.DB); e != nil {
		t.Fatal(e)
	}
	var publicationState string
	if e := s.DB.QueryRow("SELECT state FROM entity_versions WHERE id=?", publication["id"]).Scan(&publicationState); e != nil {
		t.Fatal(e)
	}
	if publicationState != "failed" {
		t.Fatal("revoked queued publication started", publicationState)
	}
	failedAudit, err := registry.QueryRow(context.Background(), s.DB, "SELECT actor_user_id,details_json FROM audit_events WHERE action='entity.publication_failed' AND resource_id=?", entityID)
	if err != nil || registry.Str(failedAudit["actor_user_id"]) != owner || !strings.Contains(registry.Str(failedAudit["details_json"]), grant) {
		t.Fatal("revoked publication lost audit identity", err)
	}

	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {"lastapp-development"}, "resource": {s.mcpURL()}}
	status, tokens := tokenRequest(form)
	if status != 200 || tokens["scope"] != "read" {
		t.Fatal("refresh", status, tokens)
	}
	if status, _ = tokenRequest(form); status != 400 {
		t.Fatal("refresh replay accepted")
	}
	if status, _ = tokenRequest(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {registry.Str(tokens["refresh_token"])}, "client_id": {"lastapp-development"}, "resource": {s.mcpURL()}}); status != 400 {
		t.Fatal("refresh replay did not revoke family")
	}
	oc.want("PATCH", "/v1/agent-access/"+grant, registry.Row{"state": "revoked"}, 2, 200)
	// A grant cannot be minted by another user into a Tenant they do not administer.
	request := oauthRequest("read resources.write")
	c.as(joe).want("POST", "/v1/agent-access/requests/"+request, registry.Row{"tenant_id": tenant, "scope_kind": "space", "scope_id": sibling, "permissions": []string{"read", "resources.write"}, "days": 1}, 0, 403)
	var count int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM microapp_installations WHERE id=?", id).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatal("duplicate mutation")
	}
	// Complete a managed database + entity publication through MCP, using only
	// owned disposable resources and the installed MariaDB (no version matrix).
	var state struct {
		Provisioner string `json:"provisioner_password"`
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(os.Getenv("LASTAPP_TEST_CONFIG")), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &state); e != nil {
		t.Fatal(e)
	}
	provisioner, e := s.sealer.Seal("target", "20000000-0000-4000-8000-000000000001", "", databases.Credentials{Username: "lastapp_provisioner", Password: state.Provisioner})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode,managed_enabled,provisioning_credentials_enc) VALUES ('20000000-0000-4000-8000-000000000001','Managed fixture','127.0.0.1',33317,'disabled',1,?)", provisioner); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		resources, _ := registry.QueryRows(context.Background(), s.DB, "SELECT id,database_name,LOWER(HEX(ownership_token)) AS ownership_hex FROM database_resources WHERE management_mode='managed'")
		for _, r := range resources {
			var comment string
			name := registry.Str(r["database_name"])
			if err := s.DB.QueryRow("SELECT SCHEMA_COMMENT FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", name).Scan(&comment); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Error("cleanup lookup failed", err)
				}
				continue
			}
			if generatedRE.MatchString(name) && comment == "lastapp:"+registry.Str(r["id"])+":"+registry.Str(r["ownership_hex"]) {
				if _, err := s.DB.Exec("DROP DATABASE `" + name + "`"); err != nil {
					t.Error("managed database cleanup failed", err)
				}
				if _, err := s.DB.Exec("DROP USER IF EXISTS '" + name + "'@'" + s.Config.RuntimeAccountHost + "'"); err != nil {
					t.Error("managed account cleanup failed", err)
				}
			}
		}
	})
	tenantAgent, tenantGrant, _, _ := connect(append(permissions, "databases.provision"), "tenant", "")
	narrowed := call(tenantAgent, "lastapp_list", registry.Row{"resource": "space", "target": registry.Row{"kind": "space", "id": space}}, true)["data"].(map[string]any)
	if len(narrowed["items"].([]any)) != 1 {
		t.Fatal("explicit read target did not narrow a broad delegation")
	}
	managed := registry.NewID()
	call(tenantAgent, "lastapp_database_provision", registry.Row{"action": "provision", "id": managed, "target_id": "20000000-0000-4000-8000-000000000001", "layout": "shared", "request_key": registry.NewID()}, true)
	call(tenantAgent, "lastapp_database_provision", registry.Row{"action": "provision", "id": registry.NewID(), "target_id": "20000000-0000-4000-8000-000000000001", "layout": "shared", "request_key": registry.NewID()}, false)
	call(tenantAgent, "lastapp_database_provision", registry.Row{"action": "provision", "id": registry.NewID(), "target_id": target, "layout": "shared", "request_key": registry.NewID()}, false)
	newSpace := registry.NewID()
	call(tenantAgent, "lastapp_create", registry.Row{"request_key": registry.NewID(), "space": registry.Row{"id": newSpace, "slug": "managed", "display_name": "Managed", "database": registry.Row{"resource_id": managed}}}, true)
	newApp := registry.NewID()
	call(tenantAgent, "lastapp_create", registry.Row{"target": registry.Row{"kind": "space", "id": newSpace}, "request_key": registry.NewID(), "app": registry.Row{"id": newApp, "slug": "crm", "display_name": "CRM"}}, true)
	team := registry.NewID()
	call(tenantAgent, "lastapp_create", registry.Row{"request_key": registry.NewID(), "team": registry.Row{"id": team, "slug": "marketing", "display_name": "Marketing"}}, true)
	call(tenantAgent, "lastapp_membership", registry.Row{"target": registry.Row{"kind": "team", "id": team}, "action": "add", "user_id": joe, "expected_revision": 1, "request_key": registry.NewID()}, true)
	call(tenantAgent, "lastapp_membership", registry.Row{"target": registry.Row{"kind": "team", "id": team}, "action": "set_state", "user_id": joe, "state": "disabled", "expected_revision": 2, "request_key": registry.NewID()}, true)
	call(tenantAgent, "lastapp_membership", registry.Row{"target": registry.Row{"kind": "team", "id": team}, "action": "remove", "user_id": joe, "expected_revision": 3, "request_key": registry.NewID()}, true)
	liveDefinition := entity.Templates()[2]
	liveDefinition.Permissions = []entity.Permission{{Key: "viewer", Label: "View deals", Operation: "read", Relationship: "member", Fields: []string{"name", "amount", "stage"}}}
	created := call(tenantAgent, "lastapp_entity_save", registry.Row{"target": registry.Row{"kind": "space", "id": newSpace}, "definition": liveDefinition, "request_key": registry.NewID()}, true)["data"].(map[string]any)
	job := call(tenantAgent, "lastapp_entity_publish", registry.Row{"entity_id": created["id"], "expected_revision": 1, "request_key": registry.NewID()}, true)["data"].(map[string]any)
	if started, e := s.publisher.Job(context.Background(), s.DB); e != nil || started {
		t.Fatal("publication must wait for pending Space storage", started, e)
	}
	var queued string
	if e := s.DB.QueryRow("SELECT state FROM entity_versions WHERE id=?", job["id"]).Scan(&queued); e != nil {
		t.Fatal(e)
	}
	if queued != "queued" {
		t.Fatal("waiting publication was failed", queued)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.RunWorker(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		v := call(tenantAgent, "lastapp_get", registry.Row{"resource": "publication", "entity_id": created["id"], "id": job["id"]}, true)["data"].(map[string]any)
		if v["state"] == "published" {
			contracts := call(tenantAgent, "lastapp_get", registry.Row{"resource": "app_contracts", "id": newApp}, true)["data"].(map[string]any)
			if len(contracts["entities"].([]any)) != 0 {
				t.Fatal("App contract inspection bypassed the App ceiling under broad Tenant authority")
			}
			condition := entity.Expr{Op: "eq", Args: []entity.Expr{{Field: "stage"}, {Literal: "qualified"}}}
			call(tenantAgent, "lastapp_app_access_update", registry.Row{"app_id": newApp, "access": entity.AppAccess{Origin: "http://127.0.0.24:8789", Ceiling: []entity.Gate{{Permission: entity.PermissionID(registry.Str(created["id"]), "viewer"), Fields: []string{"name"}, When: &condition}}}, "expected_revision": 1, "request_key": registry.NewID()}, true)
			contracts = call(tenantAgent, "lastapp_get", registry.Row{"resource": "app_contracts", "id": newApp}, true)["data"].(map[string]any)
			entities := contracts["entities"].([]any)
			if len(entities) != 1 {
				t.Fatal("approved entity missing from App context")
			}
			fields := entities[0].(map[string]any)["definition"].(map[string]any)["fields"].([]any)
			if len(fields) != 1 || strings.Contains(registry.Str(contracts), "qualified") {
				t.Fatal("App context leaked hidden fields or gate expressions", contracts)
			}
			for _, section := range []string{"definition", "json_schema", "create_schema", "update_schema", "typescript"} {
				part := call(tenantAgent, "lastapp_get", registry.Row{"target": registry.Row{"kind": "app", "id": newApp}, "resource": "entity", "id": created["id"], "section": section}, true)["data"].(map[string]any)
				if strings.Contains(registry.Str(part), "qualified") || strings.Contains(registry.Str(part), `"amount"`) || part["value"] == nil {
					t.Fatal("selected App contract leaked hidden structure/policy", section)
				}
			}
			call(tenantAgent, "lastapp_get", registry.Row{"target": registry.Row{"kind": "app", "id": newApp}, "resource": "entity", "id": created["id"], "section": "json_schema", "field": "amount"}, false)
			inventory := call(tenantAgent, "lastapp_list", registry.Row{"target": registry.Row{"kind": "app", "id": newApp}, "resource": "entity", "q": liveDefinition.Label}, true)["data"].(map[string]any)
			if len(inventory["items"].([]any)) != 1 {
				t.Fatal("App entity inventory search lost labels")
			}
			publishedAudit, err := registry.QueryRow(context.Background(), s.DB, "SELECT actor_user_id,details_json FROM audit_events WHERE action='entity.published' AND resource_id=? ORDER BY id DESC LIMIT 1", created["id"])
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(registry.Str(publishedAudit["details_json"]), tenantGrant) {
				t.Errorf("agent publication audit lost delegation attribution: actor_user_id=%v, details=%v", publishedAudit["actor_user_id"], publishedAudit["details_json"])
			}
			provisionAudit, err := registry.QueryRow(context.Background(), s.DB, "SELECT actor_user_id,details_json FROM audit_events WHERE action='database.ready' AND resource_id=? ORDER BY id DESC LIMIT 1", managed)
			if err != nil {
				t.Fatal(err)
			}
			if registry.Str(provisionAudit["actor_user_id"]) != owner || !strings.Contains(registry.Str(provisionAudit["details_json"]), tenantGrant) {
				t.Errorf("agent provisioning audit lost both actor and delegation attribution: actor_user_id=%v, details=%v", provisionAudit["actor_user_id"], provisionAudit["details_json"])
			}
			return
		}
		if v["state"] == "failed" {
			t.Fatal("live MCP publication", v)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("live MCP publication timed out")
}
