package control

// Golden contracts for refactors: the generated MCP tool schemas and the JSON
// key/type shape of representative console and MCP responses. A refactor must
// leave both files byte-identical. Regenerate deliberately with
//
//	go test ./internal/control -run Golden -update-golden
//
// (the response golden also needs LASTAPP_TEST_CONFIG) and review the diff.

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/registry"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite golden contract files")

func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	file := filepath.Join("testdata", name)
	if *updateGolden {
		if e := os.MkdirAll("testdata", 0o755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(file, got, 0o644); e != nil {
			t.Fatal(e)
		}
		return
	}
	want, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(want, got) {
		wantLines, gotLines := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
		for i := 0; i < max(len(wantLines), len(gotLines)); i++ {
			var w, g string
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if w != g {
				t.Fatalf("%s differs at line %d:\nwant: %s\ngot:  %s\n(run with -update-golden only for an intended contract change)", file, i+1, w, g)
			}
		}
	}
}

func TestAgentToolSchemaGolden(t *testing.T) {
	s := &Server{}
	tools := []registry.Row{}
	for _, tool := range s.agentTools() {
		schema, e := agentInputSchema(tool.Input)
		if e != nil {
			t.Fatal(e)
		}
		tools = append(tools, registry.Row{"name": tool.Name, "description": tool.Description, "read_only": tool.Read, "permissions": tool.Permissions, "input_schema": schema})
	}
	b, e := json.MarshalIndent(registry.Row{"instructions": agentInstructions, "tools": tools}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	compareGolden(t, "mcp_tools.golden.json", append(b, '\n'))
}

// jsonShape records every key path of v with its JSON type (and the value of
// error codes and messages). Array elements share the path suffix "[]";
// differing element shapes are merged.
func jsonShape(v any, path string, out map[string]map[string]bool) {
	kind := "null"
	switch x := v.(type) {
	case map[string]any:
		kind = "object"
		for k, child := range x {
			jsonShape(child, path+"."+k, out)
		}
	case []any:
		kind = "array"
		for _, child := range x {
			jsonShape(child, path+"[]", out)
		}
	case string:
		kind = "string"
		if strings.HasSuffix(path, ".error.code") || strings.HasSuffix(path, ".error.message") {
			kind += "=" + x // error codes and messages are part of the contract
		}
	case float64, json.Number:
		kind = "number"
	case bool:
		kind = "bool"
	}
	if out[path] == nil {
		out[path] = map[string]bool{}
	}
	out[path][kind] = true
}

type shapeRecorder struct {
	t      *testing.T
	h      http.Handler
	shapes map[string][]string
}

func (r *shapeRecorder) record(label string, status int, body any) {
	r.t.Helper()
	b, e := json.Marshal(body)
	if e != nil {
		r.t.Fatal(e)
	}
	var v any
	if e = json.Unmarshal(b, &v); e != nil {
		r.t.Fatal(e)
	}
	paths := map[string]map[string]bool{}
	jsonShape(v, "", paths)
	lines := []string{}
	for path, kinds := range paths {
		list := []string{}
		for kind := range kinds {
			list = append(list, kind)
		}
		sort.Strings(list)
		if path == "" {
			path = "$"
		}
		lines = append(lines, path+" "+strings.Join(list, "|"))
	}
	sort.Strings(lines)
	key := fmt.Sprintf("%s -> %d", label, status)
	if _, ok := r.shapes[key]; ok {
		r.t.Fatalf("duplicate golden label %s", key)
	}
	r.shapes[key] = lines
}

func (r *shapeRecorder) http(c testClient, label, method, path string, body any, revision int64, status int) registry.Row {
	r.t.Helper()
	got, v := c.req(method, path, body, revision)
	if got != status {
		r.t.Fatalf("%s %s: got %d want %d: %v", method, path, got, status, v)
	}
	r.record(label, got, v)
	return v
}

// mcp calls a tool through the real /mcp endpoint and records the structured
// tool result, so the adapter's success, error and receipt shapes are covered.
func (r *shapeRecorder) mcp(s *Server, access, label, tool string, args any, ok bool) registry.Row {
	r.t.Helper()
	body := agentTestJSON(r.t, registry.Row{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": registry.Row{"name": tool, "arguments": args}})
	req := httptest.NewRequest("POST", s.mcpURL(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, req)
	var envelope struct {
		Result struct {
			Structured registry.Row `json:"structuredContent"`
		} `json:"result"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &envelope); e != nil || envelope.Result.Structured == nil {
		r.t.Fatalf("%s: invalid MCP response %d %s", label, w.Code, w.Body.String())
	}
	v := envelope.Result.Structured
	if v["ok"] != ok {
		r.t.Fatalf("%s: ok=%v want %v: %v", label, v["ok"], ok, v)
	}
	r.record("mcp "+label, w.Code, v)
	return v
}

func TestResponseShapeGolden(t *testing.T) {
	s, c, operator, owner, joe := foundationFixture(t)
	rec := &shapeRecorder{t: t, h: c.h, shapes: map[string][]string{}}
	oc := c.as(owner)

	// Platform: plans, Tenants and Tenant administrators.
	rec.http(c, "GET /admin/v1/plans", "GET", "/admin/v1/plans", nil, 0, 200)
	plan := registry.NewID()
	planBody := registry.Row{"id": plan, "code": "golden", "display_name": "Golden", "allow_shared_database": true, "allow_dedicated_database": false, "allow_managed_database": true, "allow_registered_database": true, "allow_dedicated_compute": false, "default_layout": "shared", "max_spaces": 5, "max_apps": nil, "max_databases": 3}
	rec.http(c, "POST /admin/v1/plans", "POST", "/admin/v1/plans", planBody, 0, 201)
	planBody["display_name"] = "Golden plan"
	rec.http(c, "PATCH /admin/v1/plans/{id}", "PATCH", "/admin/v1/plans/"+plan, planBody, 1, 200)
	rec.http(c, "PATCH /admin/v1/plans/{id} stale", "PATCH", "/admin/v1/plans/"+plan, planBody, 1, 412)
	rec.http(c, "PATCH /admin/v1/plans/{id} without If-Match", "PATCH", "/admin/v1/plans/"+plan, planBody, 0, 428)
	tenant := registry.NewID()
	rec.http(c, "POST /admin/v1/tenants", "POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "golden-" + tenant[:8], "display_name": "Golden", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	rec.http(c, "GET /admin/v1/tenants", "GET", "/admin/v1/tenants?q=golden-"+tenant[:8], nil, 0, 200)
	rec.http(c, "PATCH /admin/v1/tenants/{id}", "PATCH", "/admin/v1/tenants/"+tenant, registry.Row{"display_name": "Golden Tenant"}, 1, 200)
	rec.http(c, "GET /admin/v1/tenants/{tenant}/administrators", "GET", "/admin/v1/tenants/"+tenant+"/administrators", nil, 0, 200)
	rec.http(c, "POST /admin/v1/tenants/{tenant}/administrators bad", "POST", "/admin/v1/tenants/"+tenant+"/administrators", registry.Row{}, 0, 400)
	base := "/v1/tenants/" + tenant
	rec.http(oc, "GET /v1/tenants/{tenant}", "GET", base, nil, 0, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}", "PATCH", base, registry.Row{"display_name": "Golden company"}, 2, 200)
	rec.http(oc, "GET /admin/v1/tenants as owner", "GET", "/admin/v1/tenants", nil, 0, 403)

	// Memberships and teams.
	rec.http(oc, "POST /v1/tenants/{tenant}/memberships", "POST", base+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	rec.http(oc, "GET /v1/tenants/{tenant}/memberships", "GET", base+"/memberships", nil, 0, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/memberships/{user}", "PATCH", base+"/memberships/"+joe, registry.Row{"state": "active"}, 1, 200)
	team := registry.NewID()
	rec.http(oc, "POST /v1/tenants/{tenant}/teams", "POST", base+"/teams", registry.Row{"id": team, "slug": "golden", "display_name": "Golden"}, 0, 201)
	rec.http(oc, "GET /v1/tenants/{tenant}/teams", "GET", base+"/teams", nil, 0, 200)
	rec.http(oc, "GET /v1/tenants/{tenant}/teams/{id}", "GET", base+"/teams/"+team, nil, 0, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/teams/{id}", "PATCH", base+"/teams/"+team, registry.Row{"display_name": "Golden team"}, 1, 200)
	rec.http(oc, "POST /v1/tenants/{tenant}/teams/{id}/members", "POST", base+"/teams/"+team+"/members", registry.Row{"user_id": joe}, 2, 200)
	rec.http(oc, "GET /v1/tenants/{tenant}/teams/{id}/members", "GET", base+"/teams/"+team+"/members", nil, 0, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/teams/{id}/members/{user}", "PATCH", base+"/teams/"+team+"/members/"+joe, registry.Row{"state": "disabled"}, 3, 200)
	rec.http(oc, "DELETE /v1/tenants/{tenant}/teams/{id}/members/{user}", "DELETE", base+"/teams/"+team+"/members/"+joe, nil, 4, 200)

	// Spaces, Apps and micro-apps on a metadata-only registered database.
	target, resource := registry.NewID(), registry.NewID()
	if _, e := s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode) VALUES (?,'Golden target','127.0.0.2',33317,'disabled')", target); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'golden_metadata_only','registered','shared','ready',X'00')", resource, tenant, target); e != nil {
		t.Fatal(e)
	}
	space := registry.NewID()
	rec.http(oc, "POST /v1/tenants/{tenant}/spaces", "POST", base+"/spaces", registry.Row{"id": space, "slug": "golden", "display_name": "Golden", "database": registry.Row{"resource_id": resource}, "initial_manager_user_id": owner}, 0, 201)
	rec.http(oc, "GET /v1/tenants/{tenant}/spaces", "GET", base+"/spaces", nil, 0, 200)
	rec.http(oc, "GET /v1/tenants/{tenant}/spaces/{space}", "GET", base+"/spaces/"+space, nil, 0, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/spaces/{id}", "PATCH", base+"/spaces/"+space, registry.Row{"display_name": "Golden space"}, 1, 200)
	app := registry.NewID()
	rec.http(oc, "POST /v1/tenants/{tenant}/spaces/{space}/apps", "POST", base+"/spaces/"+space+"/apps", registry.Row{"id": app, "slug": "golden", "display_name": "Golden", "initial_manager_user_id": owner}, 0, 201)
	rec.http(oc, "GET /v1/tenants/{tenant}/apps", "GET", base+"/apps", nil, 0, 200)
	rec.http(oc, "GET /v1/tenants/{tenant}/apps?space_id", "GET", base+"/apps?space_id="+space, nil, 0, 200)
	rec.http(oc, "GET /v1/tenants/{tenant}/apps/{app}", "GET", base+"/apps/"+app, nil, 0, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/apps/{id}", "PATCH", base+"/apps/"+app, registry.Row{"display_name": "Golden app"}, 1, 200)
	microapp := registry.NewID()
	rec.http(oc, "POST /v1/tenants/{tenant}/apps/{app}/microapps", "POST", base+"/apps/"+app+"/microapps", registry.Row{"id": microapp, "slug": "golden", "display_name": "Golden", "artifact_ref": "golden.js", "navigation_order": 2}, 0, 201)
	rec.http(oc, "GET /v1/tenants/{tenant}/microapps", "GET", base+"/microapps?app_id="+app, nil, 0, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/microapps/{id}", "PATCH", base+"/microapps/"+microapp, registry.Row{"display_name": "Golden micro-app", "artifact_ref": "", "navigation_order": 1}, 1, 200)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/microapps/{id} invalid", "PATCH", base+"/microapps/"+microapp, registry.Row{"compute_mode": "shared"}, 2, 400)
	rec.http(oc, "PATCH /v1/tenants/{tenant}/microapps/{id} unknown field", "PATCH", base+"/microapps/"+microapp, registry.Row{"plan_id": plan}, 2, 400)

	// MCP: a platform connection for plans and a Tenant connection for the
	// Tenant inventory, memberships and teams.
	grant := func(user, kind, tenantID string, permissions ...string) string {
		t.Helper()
		id := registry.NewID()
		perms, _ := json.Marshal(permissions)
		if _, e := s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'golden','Golden client',?,?,?,'{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", id, user, registry.Nullable(tenantID), kind, perms, s.mcpURL()); e != nil {
			t.Fatal(e)
		}
		g, e := s.ReadAgentGrant(t.Context(), s.DB, id)
		if e != nil {
			t.Fatal(e)
		}
		return agentFixtureToken(t, s, access.Principal{ID: user, Agent: g})
	}
	platform := grant(operator, "platform", "", "read", "platform.manage")
	key := registry.NewID()
	plan2 := registry.NewID()
	create := registry.Row{"target": registry.Row{"kind": "platform"}, "request_key": key, "plan": registry.Row{"id": plan2, "code": "golden-mcp", "display_name": "Golden MCP", "allow_shared_database": true, "allow_dedicated_database": true, "allow_managed_database": true, "allow_registered_database": false, "allow_dedicated_compute": true, "default_layout": "dedicated", "max_spaces": nil, "max_apps": 4, "max_databases": nil}}
	rec.mcp(s, platform, "lastapp_create plan", "lastapp_create", create, true)
	rec.mcp(s, platform, "lastapp_create plan replay", "lastapp_create", create, true)
	rec.mcp(s, platform, "lastapp_list plan", "lastapp_list", registry.Row{"resource": "plan", "target": registry.Row{"kind": "platform"}, "q": "golden"}, true)
	rec.mcp(s, platform, "lastapp_get plan", "lastapp_get", registry.Row{"resource": "plan", "target": registry.Row{"kind": "platform"}, "id": plan2}, true)
	planPatch := create["plan"].(registry.Row)
	planPatch["display_name"] = "Golden MCP plan"
	rec.mcp(s, platform, "lastapp_update plan", "lastapp_update", registry.Row{"resource": "plan", "id": plan2, "target": registry.Row{"kind": "platform"}, "expected_revision": 1, "request_key": registry.NewID(), "plan": planPatch}, true)
	rec.mcp(s, platform, "lastapp_update plan stale", "lastapp_update", registry.Row{"resource": "plan", "id": plan2, "target": registry.Row{"kind": "platform"}, "expected_revision": 1, "request_key": registry.NewID(), "plan": planPatch}, false)
	rec.mcp(s, platform, "lastapp_list tenant", "lastapp_list", registry.Row{"resource": "tenant", "target": registry.Row{"kind": "platform"}, "q": "golden-" + tenant[:8]}, true)
	rec.mcp(s, platform, "lastapp_get tenant from platform", "lastapp_get", registry.Row{"resource": "tenant", "target": registry.Row{"kind": "platform"}, "id": tenant}, true)

	owned := grant(owner, "tenant", tenant, "read", "resources.write", "access.manage", "lifecycle.manage", "production.deploy")
	readOnly := grant(owner, "tenant", tenant, "read", "access.manage")
	microapp2 := registry.NewID()
	rec.mcp(s, owned, "lastapp_create microapp", "lastapp_create", registry.Row{"target": registry.Row{"kind": "app", "id": app}, "request_key": registry.NewID(), "microapp": registry.Row{"id": microapp2, "slug": "golden-mcp", "display_name": "Golden MCP", "navigation_order": 3}}, true)
	rec.mcp(s, readOnly, "lastapp_create microapp without resources.write", "lastapp_create", registry.Row{"target": registry.Row{"kind": "app", "id": app}, "request_key": registry.NewID(), "microapp": registry.Row{"id": registry.NewID(), "slug": "denied", "display_name": "Denied"}}, false)
	rec.mcp(s, owned, "lastapp_list microapp", "lastapp_list", registry.Row{"resource": "microapp", "target": registry.Row{"kind": "app", "id": app}}, true)
	rec.mcp(s, owned, "lastapp_get microapp", "lastapp_get", registry.Row{"resource": "microapp", "id": microapp2}, true)
	rec.mcp(s, owned, "lastapp_update microapp", "lastapp_update", registry.Row{"resource": "microapp", "id": microapp2, "expected_revision": 1, "request_key": registry.NewID(), "display_name": "Golden MCP micro-app", "artifact_ref": "mcp.js", "navigation_order": 4}, true)
	rec.mcp(s, owned, "lastapp_update microapp unsupported field", "lastapp_update", registry.Row{"resource": "microapp", "id": microapp2, "expected_revision": 2, "request_key": registry.NewID(), "plan_id": plan}, false)
	rec.mcp(s, owned, "lastapp_lifecycle microapp", "lastapp_lifecycle", registry.Row{"resource": "microapp", "id": microapp2, "state": "archived", "expected_revision": 2, "request_key": registry.NewID()}, true)
	rec.mcp(s, owned, "lastapp_lifecycle microapp stale", "lastapp_lifecycle", registry.Row{"resource": "microapp", "id": microapp2, "state": "active", "expected_revision": 2, "request_key": registry.NewID()}, false)
	rec.mcp(s, owned, "lastapp_list space", "lastapp_list", registry.Row{"resource": "space"}, true)
	rec.mcp(s, owned, "lastapp_get space", "lastapp_get", registry.Row{"resource": "space", "id": space}, true)
	rec.mcp(s, owned, "lastapp_update space", "lastapp_update", registry.Row{"resource": "space", "id": space, "expected_revision": 2, "request_key": registry.NewID(), "display_name": "Golden MCP space"}, true)
	rec.mcp(s, owned, "lastapp_list app", "lastapp_list", registry.Row{"resource": "app", "target": registry.Row{"kind": "space", "id": space}}, true)
	rec.mcp(s, owned, "lastapp_get app", "lastapp_get", registry.Row{"resource": "app", "id": app}, true)
	rec.mcp(s, owned, "lastapp_update app", "lastapp_update", registry.Row{"resource": "app", "id": app, "expected_revision": 2, "request_key": registry.NewID(), "compute_mode": "shared"}, true)
	rec.mcp(s, owned, "lastapp_get tenant from tenant", "lastapp_get", registry.Row{"resource": "tenant", "id": tenant}, true)
	rec.mcp(s, owned, "lastapp_update tenant", "lastapp_update", registry.Row{"resource": "tenant", "id": tenant, "expected_revision": 3, "request_key": registry.NewID(), "display_name": "Golden MCP company"}, true)
	team2 := registry.NewID()
	rec.mcp(s, owned, "lastapp_create team", "lastapp_create", registry.Row{"request_key": registry.NewID(), "team": registry.Row{"id": team2, "slug": "golden-mcp", "display_name": "Golden MCP"}}, true)
	rec.mcp(s, owned, "lastapp_list team", "lastapp_list", registry.Row{"resource": "team"}, true)
	rec.mcp(s, owned, "lastapp_get team", "lastapp_get", registry.Row{"resource": "team", "id": team2}, true)
	rec.mcp(s, owned, "lastapp_update team", "lastapp_update", registry.Row{"resource": "team", "id": team2, "expected_revision": 1, "request_key": registry.NewID(), "display_name": "Golden MCP team"}, true)
	rec.mcp(s, owned, "lastapp_membership team add", "lastapp_membership", registry.Row{"action": "add", "target": registry.Row{"kind": "team", "id": team2}, "user_id": joe, "expected_revision": 2, "request_key": registry.NewID()}, true)
	rec.mcp(s, owned, "lastapp_list team_member", "lastapp_list", registry.Row{"resource": "team_member", "target": registry.Row{"kind": "team", "id": team2}}, true)
	rec.mcp(s, owned, "lastapp_membership tenant set_state", "lastapp_membership", registry.Row{"action": "set_state", "user_id": joe, "state": "active", "expected_revision": 2, "request_key": registry.NewID()}, true)
	rec.mcp(s, owned, "lastapp_list tenant_member", "lastapp_list", registry.Row{"resource": "tenant_member"}, true)
	rec.mcp(s, owned, "lastapp_lifecycle team", "lastapp_lifecycle", registry.Row{"resource": "team", "id": team2, "state": "disabled", "expected_revision": 3, "request_key": registry.NewID()}, true)

	labels := make([]string, 0, len(rec.shapes))
	for label := range rec.shapes {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	var out bytes.Buffer
	for _, label := range labels {
		out.WriteString(label + "\n")
		for _, line := range rec.shapes[label] {
			out.WriteString("  " + line + "\n")
		}
	}
	compareGolden(t, "response_shapes.golden.txt", out.Bytes())
}
