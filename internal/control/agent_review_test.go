package control

// Metadata fixtures never access business storage. Live worker audit coverage
// extends TestAgentManagementMCP, which owns and cleans its generated resources.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aantich/lastapp/internal/entity"
)

func agentSpaceFixture(t *testing.T) (*Server, testClient, Principal, string, string, string) {
	t.Helper()
	s, c, p, tenant, target, _ := agentSecurityFixture(t)
	resource, space, app := newID(), newID(), newID()
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'review2_metadata_only','registered','shared','ready',X'00')", resource, tenant, target); e != nil {
		t.Fatal(e)
	}
	c.want("POST", "/v1/tenants/"+tenant+"/spaces", Row{"id": space, "slug": "review2", "display_name": "Review two", "database": Row{"resource_id": resource}, "initial_manager_user_id": p.ID}, 0, 201)
	c.want("POST", "/v1/tenants/"+tenant+"/spaces/"+space+"/apps", Row{"id": app, "slug": "review2", "display_name": "Review two", "initial_manager_user_id": p.ID}, 0, 201)
	return s, c, p, tenant, space, app
}

func agentPublishedFixture(t *testing.T, s *Server, p Principal, tenant, space string, d entity.Definition) string {
	t.Helper()
	if ds := d.Validate(); len(ds) > 0 {
		t.Fatal("invalid fixture definition", ds[0])
	}
	eid, vid := newID(), newID()
	b, _ := json.Marshal(d)
	if len(b) > entity.MaxDefinitionBytes {
		t.Fatal("fixture exceeds definition limit")
	}
	if _, e := s.DB.Exec("INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,draft_json) VALUES(?,?,?,?,?,?)", eid, tenant, space, d.Key, d.Label, b); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("INSERT INTO entity_versions(id,tenant_id,space_id,entity_id,definition_json,definition_hash,created_by,state) VALUES(?,?,?,?,?,?,?,'published')", vid, tenant, space, eid, b, entity.Hash(d), p.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("UPDATE entity_definitions SET active_version_id=? WHERE id=?", vid, eid); e != nil {
		t.Fatal(e)
	}
	return eid
}

func TestAgentPlatformScopePermissionBoundary(t *testing.T) {
	s, c, operator, _, _ := foundationFixture(t)
	candidate := newID()
	if _, e := s.DB.Exec("INSERT INTO users(id,oidc_issuer,oidc_subject,display_name) VALUES(?,?,?,'Review candidate')", candidate, s.Config.OIDCIssuer, candidate); e != nil {
		t.Fatal(e)
	}
	grant := newID()
	if _, e := s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,scope_kind,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'review2','Review two','platform','[\"read\",\"access.manage\"]','{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", grant, operator, s.mcpURL()); e != nil {
		t.Fatal(e)
	}
	g, e := s.readAgentGrant(context.Background(), s.DB, grant)
	if e != nil {
		t.Fatal(e)
	}
	p := Principal{ID: operator, Platform: true, Agent: g}
	// The honest platform target must demand platform.manage.
	normal := agentAssignmentInput{Action: "grant", Target: &agentTarget{Kind: "platform"}, ID: newID(), Role: SuperRole, User: candidate, RequestKey: newID()}
	raw, _ := json.Marshal(normal)
	_, e = s.toolAgentAssignment(context.Background(), p, raw)
	var authority *agentAuthorityError
	if !errors.As(e, &authority) {
		t.Fatal("normal platform target did not require explicit platform authority", e)
	}
	// The API adapter must not infer platform scope from a missing Tenant ID.
	normal.Target = &agentTarget{Kind: "tenant"}
	normal.RequestKey = newID()
	args, _ := json.Marshal(normal)
	schema, e := agentInputSchema(agentType[agentAssignmentInput]())
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := schema.Resolve(nil)
	if e != nil {
		t.Fatal(e)
	}
	var instance any
	if e = json.Unmarshal(args, &instance); e != nil {
		t.Fatal(e)
	}
	if e = resolved.Validate(instance); e != nil {
		t.Fatal("probe input is not an accepted MCP schema", e)
	}
	access := agentFixtureToken(t, s, p)
	r := httptest.NewRequest("POST", s.mcpURL(), strings.NewReader(string(agentTestJSON(t, Row{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": Row{"name": "lastapp_role_assignment", "arguments": normal}}))))
	r.Header.Set("Authorization", "Bearer "+access)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	var count int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM access_grants WHERE role_id=? AND user_id=?", SuperRole, candidate).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 0 {
		t.Fatalf("MCP added a platform administrator without platform.manage via target.kind=tenant; HTTP %d", w.Code)
	}
	if w.Code != http.StatusOK {
		t.Fatal("protocol request was rejected before tool execution", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":false`) {
		t.Fatal("invalid scope did not return an explicit tool error")
	}
	// An explicit platform grant still works through the same operation.
	if _, e = s.DB.Exec("UPDATE agent_grants SET permissions_json='[\"read\",\"access.manage\",\"platform.manage\"]',revision=revision+1 WHERE id=?", grant); e != nil {
		t.Fatal(e)
	}
	p.Agent, e = s.readAgentGrant(context.Background(), s.DB, grant)
	if e != nil {
		t.Fatal(e)
	}
	normal.Target = &agentTarget{Kind: "platform"}
	normal.RequestKey = newID()
	if _, e = s.toolAgentAssignment(context.Background(), p, agentTestJSON(t, normal)); e != nil {
		t.Fatal("explicitly approved platform assignment failed", e)
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM access_grants WHERE role_id=? AND user_id=?", SuperRole, candidate).Scan(&count); e != nil || count != 1 {
		t.Fatal("approved assignment missing", e)
	}
}

func agentTestJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestAgentAppInventoryPaging(t *testing.T) {
	s, _, p, tenant, space, app := agentSpaceFixture(t)
	if _, e := s.DB.Exec("UPDATE apps SET data_access_json=? WHERE id=?", `{"inherit":true}`, app); e != nil {
		t.Fatal(e)
	}
	d := entity.Definition{Format: 1, Key: "inventory", Label: "Inventory", Rules: []entity.Rule{}}
	keys := []string{}
	for i := 0; i < 64; i++ {
		key := fmt.Sprintf("f%047d", i)
		keys = append(keys, key)
		d.Fields = append(d.Fields, entity.Field{Key: key, Label: strings.Repeat("F", 160), Type: "integer"})
	}
	d.Projection = []string{keys[0]}
	for i := 0; i < 64; i++ {
		d.Permissions = append(d.Permissions, entity.Permission{Key: fmt.Sprintf("read_%d", i), Label: strings.Repeat("P", 160), Operation: "read", Relationship: "member", Fields: keys})
	}
	var first string
	ids := []string{}
	for i := 0; i < 12; i++ {
		d.Key = fmt.Sprintf("inventory_%d", i)
		d.Label = d.Key
		eid := agentPublishedFixture(t, s, p, tenant, space, d)
		ids = append(ids, eid)
		if i == 0 {
			first = eid
		}
	}
	raw := agentTestJSON(t, agentReadInput{Resource: "entity", ID: first, Target: &agentTarget{Kind: "app", ID: app}, Section: "definition"})
	if _, e := s.toolAgentGet(context.Background(), p, raw); e != nil {
		t.Fatal("one valid App definition must be inspectable", e)
	}
	for _, in := range []agentReadInput{
		{Resource: "entity", Target: &agentTarget{Kind: "app", ID: app}, Limit: 1},
		{Resource: "entity", Target: &agentTarget{Kind: "app", ID: app}, Limit: 1, Q: "inventory_0"},
	} {
		v, e := s.agentList(context.Background(), p, in)
		if e != nil {
			t.Fatal("valid inventory failed before paging/search", e)
		}
		if len(v["items"].([]any)) != 1 {
			t.Fatal("inventory was not paged", v["pagination"])
		}
		page := v["pagination"].(map[string]any)
		if page["has_more"] != (in.Q == "") || v["complete"] != (in.Q != "") {
			t.Fatal("incorrect inventory completeness", page)
		}
	}
	seen := map[string]bool{}
	for page := 1; page <= 12; page++ {
		v, e := s.agentList(context.Background(), p, agentReadInput{Resource: "entity", Target: &agentTarget{Kind: "app", ID: app}, Limit: 1, Page: page})
		if e != nil {
			t.Fatal(e)
		}
		items := v["items"].([]any)
		if len(items) != 1 {
			t.Fatal("missing authorized page", page)
		}
		key := str(items[0].(map[string]any)["id"])
		if seen[key] {
			t.Fatal("duplicate paged entity", key)
		}
		seen[key] = true
	}
	// An App ceiling filters visibility before search and paging, including
	// when the delegating human administers the entire Tenant.
	access := entity.AppAccess{Ceiling: []entity.Gate{{Permission: entity.PermissionID(ids[2], "read_0"), Fields: keys[:1]}}}
	if _, e := s.DB.Exec("UPDATE apps SET data_access_json=? WHERE id=?", agentTestJSON(t, access), app); e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{"", "inventory_0"} {
		v, e := s.agentList(context.Background(), p, agentReadInput{Resource: "entity", Target: &agentTarget{Kind: "app", ID: app}, Limit: 1, Q: query})
		if e != nil {
			t.Fatal(e)
		}
		items := v["items"].([]any)
		want := 1
		if query != "" {
			want = 0
		}
		if len(items) != want || v["pagination"].(map[string]any)["has_more"] != false {
			t.Fatal("paging/search leaked hidden inventory", v)
		}
		if len(items) > 0 {
			row := items[0].(map[string]any)
			if str(row["id"]) != ids[2] || len(row["fields"].([]any)) != 1 {
				t.Fatal("inventory escaped App ceiling")
			}
		}
	}
}

func TestAgentLargeEntityCheck(t *testing.T) {
	s, _, p, tenant, space, _ := agentSpaceFixture(t)
	d := entity.Definition{Format: 1, Key: "large", Label: "Large", Rules: []entity.Rule{}, Permissions: []entity.Permission{}, Projection: []string{"choice_0"}}
	for field := 0; field < 48; field++ {
		f := entity.Field{Key: fmt.Sprintf("choice_%d", field), Label: "Choice", Type: "choice"}
		for i := 0; i < 100; i++ {
			f.Values = append(f.Values, strings.Repeat("v", 140)+fmt.Sprintf("%03d", i))
		}
		d.Fields = append(d.Fields, f)
	}
	eid := agentPublishedFixture(t, s, p, tenant, space, d)
	if _, e := s.DB.Exec("UPDATE agent_grants SET permissions_json='[\"read\",\"entities.check\"]' WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	g, e := s.readAgentGrant(context.Background(), s.DB, p.Agent.ID)
	if e != nil {
		t.Fatal(e)
	}
	p.Agent = g
	ctx := context.WithValue(context.Background(), agentContextKey{}, &AgentCall{Grant: g, Permission: []string{"entities.check"}, Operation: "lastapp_entity_check"})
	raw := agentTestJSON(t, agentCheckInput{Entity: eid, Cases: []agentCase{{Operation: "create", Values: map[string]any{"choice_0": d.Fields[0].Values[0]}}}})
	checked, e := s.toolAgentEntityCheck(ctx, p, raw)
	if e != nil {
		t.Fatal("large saved entity cannot be checked", e)
	}
	result := checked.(Row)
	if len(result["diagnostics"].([]any)) != 0 || len(result["cases"].([]Row)) != 1 || result["schema"] != nil || result["typescript"] != nil || str(result["candidate_hash"]) != entity.Hash(d) {
		t.Fatal("check omitted diagnostics/cases or returned heavy contracts")
	}
	candidate := d
	candidate.Fields = append([]entity.Field(nil), d.Fields...)
	candidate.Fields[0].Type = "invalid"
	checked, e = s.toolAgentEntityCheck(ctx, p, agentTestJSON(t, agentCheckInput{Entity: eid, Definition: &candidate}))
	if e != nil {
		t.Fatal(e)
	}
	if len(checked.(Row)["diagnostics"].([]any)) == 0 {
		t.Fatal("candidate validation was skipped")
	}
	stored, e := row(context.Background(), s.DB, "SELECT draft_json FROM entity_definitions WHERE id=?", eid)
	if e != nil {
		t.Fatal(e)
	}
	definition, e := defOf(stored["draft_json"])
	if e != nil || entity.Hash(definition) != entity.Hash(d) {
		t.Fatal("check persisted its candidate", e)
	}
}

func TestAgentBatchAdmission(t *testing.T) {
	s, _, p, tenant, space, _ := agentSpaceFixture(t)
	d := entity.Definition{Format: 1, Key: "batch", Label: "Batch", Rules: []entity.Rule{}, Permissions: []entity.Permission{}, Projection: []string{"choice_0"}}
	for field := 0; field < 8; field++ {
		f := entity.Field{Key: fmt.Sprintf("choice_%d", field), Label: "Choice", Type: "choice"}
		for i := 0; i < 100; i++ {
			f.Values = append(f.Values, strings.Repeat("v", 140)+fmt.Sprintf("%03d", i))
		}
		d.Fields = append(d.Fields, f)
	}
	eid := agentPublishedFixture(t, s, p, tenant, space, d)
	access := agentFixtureToken(t, s, p)
	calls := []Row{}
	for i := 0; i < agentMaxConcurrent+8; i++ {
		calls = append(calls, Row{"jsonrpc": "2.0", "id": i + 1, "method": "tools/call", "params": Row{"name": "lastapp_get", "arguments": Row{"resource": "entity", "id": eid, "section": "definition"}}})
	}
	raw := agentTestJSON(t, calls)
	if len(raw) > 1<<20 {
		t.Fatal("fixture exceeds request budget")
	}
	r := httptest.NewRequest("POST", s.mcpURL(), strings.NewReader(string(raw)))
	r.Header.Set("Authorization", "Bearer "+access)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	s.mcpHandler().ServeHTTP(w, r)
	if w.Code == 200 {
		var result []Row
		if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
			t.Fatal("invalid batch response", e)
		}
		succeeded := 0
		for _, one := range result {
			if v, ok := one["result"].(map[string]any); ok {
				if content, ok := v["structuredContent"].(map[string]any); ok && content["ok"] == true {
					succeeded++
				}
			}
		}
		t.Fatalf("one %d-byte HTTP request admitted %d/%d tool executions through one slot and returned %d bytes (configured context cap %d); omitted protocol header allowed legacy batching", len(raw), succeeded, len(calls), w.Body.Len(), agentMaxResponse)
	}
	if w.Code != 400 {
		t.Fatal("batch must be explicitly rejected", w.Code)
	}
	for _, version := range []string{"2025-03-26", "2025-11-25"} {
		r = httptest.NewRequest("POST", s.mcpURL(), strings.NewReader(string(raw)))
		r.Header.Set("Authorization", "Bearer "+access)
		r.Header.Set("MCP-Protocol-Version", version)
		w = httptest.NewRecorder()
		s.mcpHandler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("version-dependent batch bypass", version, w.Code)
		}
	}
	r = httptest.NewRequest("POST", s.mcpURL(), strings.NewReader(`{"jsonrpc":"2.0","params":"`+strings.Repeat("x", 1<<20)+`"}`))
	r.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	s.mcpHandler().ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatal("oversized request was not bounded", w.Code)
	}
}

func TestAgentSharedSpaceReuse(t *testing.T) {
	s, c, p, tenant, space, _ := agentSpaceFixture(t)
	var resource string
	if e := s.DB.QueryRow("SELECT database_resource_id FROM spaces WHERE id=?", space).Scan(&resource); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("UPDATE tenants SET default_shared_database_id=? WHERE id=?", resource, tenant); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("UPDATE agent_grants SET permissions_json='[\"read\",\"resources.write\"]',limits_json='{}' WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	g, e := s.readAgentGrant(context.Background(), s.DB, p.Agent.ID)
	if e != nil {
		t.Fatal(e)
	}
	p.Agent = g
	input := spaceCreateInput{namedInput: namedInput{ID: newID(), Slug: "reuse-direct", Name: "Reuse direct"}, Manager: p.ID}
	input.Database.Mode = "managed"
	input.Database.Layout = "shared"
	// Existing shared resource: the shared operation itself needs no allocation.
	c.want("POST", "/v1/tenants/"+tenant+"/spaces", input, 0, 201)
	input.ID = newID()
	input.Slug = "reuse-agent"
	input.Name = "Reuse agent"
	raw := agentTestJSON(t, agentCreateInput{Space: &input, RequestKey: newID()})
	if _, e = s.toolAgentCreate(context.Background(), p, raw); e != nil {
		t.Fatal("shared reuse demanded provisioning permission", e)
	}
	var count int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM database_resources WHERE tenant_id=?", tenant).Scan(&count); e != nil || count != 1 {
		t.Fatal("reuse allocated storage", e)
	}
	if _, e = s.DB.Exec("UPDATE tenants SET default_shared_database_id=NULL WHERE id=?", tenant); e != nil {
		t.Fatal(e)
	}
	input.ID = newID()
	input.Slug = "unapproved-allocation"
	_, e = s.toolAgentCreate(context.Background(), p, agentTestJSON(t, agentCreateInput{Space: &input, RequestKey: newID()}))
	var apiError *APIError
	if !errors.As(e, &apiError) || apiError.Code != "needs_authorization" {
		t.Fatal("new allocation did not enforce provisioning delegation", e)
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM database_resources WHERE tenant_id=?", tenant).Scan(&count); e != nil || count != 1 {
		t.Fatal("unauthorized allocation was not rolled back", e)
	}
}

func TestAgentDeniedDatabaseJobAudit(t *testing.T) {
	s, _, p, tenant, target, _ := agentSecurityFixture(t)
	record := newID()
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc,ownership_token,agent_grant_id,agent_user_id) VALUES(?,?,?,'review_denied_metadata_only','managed','shared','pending',X'00',UNHEX(REPEAT('01',16)),?,?)", record, tenant, target, p.Agent.ID, p.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); s.RunWorker(ctx) }()
	defer func() { cancel(); <-done }()
	for ctx.Err() == nil {
		v, e := row(ctx, s.DB, "SELECT state,last_error_code FROM database_resources WHERE id=?", record)
		if e != nil {
			t.Fatal(e)
		}
		if v["state"] == "failed" {
			if v["last_error_code"] != "agent_access_revoked" {
				t.Fatal("revoked job reached provisioning", v)
			}
			event, e := row(ctx, s.DB, "SELECT actor_user_id,details_json FROM audit_events WHERE resource_id=? AND action='database.failed'", record)
			if e != nil {
				t.Fatal(e)
			}
			details := event["details_json"].(map[string]any)
			if str(event["actor_user_id"]) != p.ID || str(details["agent_grant_id"]) != p.Agent.ID || str(details["agent_client_id"]) != p.Agent.Client {
				t.Fatal("denied job lost persistent attribution", event)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("revoked queued database job was not rejected")
}
