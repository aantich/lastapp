package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/agent"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

// Regression fixtures use metadata-only database resources and never run a worker.
func agentSecurityFixture(t *testing.T) (*Server, testClient, access.Principal, string, string, string) {
	t.Helper()
	s, c, _, owner, _ := foundationFixture(t)
	tenant := registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "review-" + tenant, "display_name": "MCP review fixture", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	allowed, denied := registry.NewID(), registry.NewID()
	for i, target := range []string{allowed, denied} {
		if _, e := s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode,managed_enabled,provisioning_credentials_enc) VALUES(?,'Review target',?,33317,'disabled',1,X'00')", target, fmt.Sprintf("127.0.0.%d", i+2)); e != nil {
			t.Fatal(e)
		}
	}
	grant := registry.NewID()
	perms, _ := json.Marshal([]string{"read", "production.deploy", "resources.write", "entities.draft", "databases.provision"})
	limits, _ := json.Marshal(access.AgentLimits{TargetIDs: []string{allowed}, MaxDatabases: 1})
	if _, e := s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'review-client','Review client',?,'tenant',?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", grant, owner, tenant, perms, limits, s.mcpURL()); e != nil {
		t.Fatal(e)
	}
	g, e := s.ReadAgentGrant(context.Background(), s.DB, grant)
	if e != nil {
		t.Fatal(e)
	}
	return s, c.as(owner), access.Principal{ID: owner, Agent: g}, tenant, allowed, denied
}

func TestAgentRetryTargetGuard(t *testing.T) {
	s, _, p, tenant, _, denied := agentSecurityFixture(t)
	record := registry.NewID()
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc,ownership_token) VALUES(?,?,?,'la_review_never_provisioned','managed','shared','failed',X'00',UNHEX(REPEAT('01',16)))", record, tenant, denied); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(agent.DatabaseInput{Action: "retry", ID: record, Revision: 1, RequestKey: registry.NewID()})
	result, e := agentTool(s, "lastapp_database_provision")(context.Background(), p, raw)
	var ae *registry.APIError
	if e != nil && (!errors.As(e, &ae) || ae.Code != "needs_authorization") {
		t.Fatal("retry failed for the wrong reason", e)
	}
	if e == nil {
		row, e := registry.QueryRow(context.Background(), s.DB, "SELECT state,agent_grant_id FROM database_resources WHERE id=?", record)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.DB.Exec("UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
			t.Fatal(e)
		}
		allowed := s.AgentJobAllowed(context.Background(), s.DB, registry.Str(row["agent_grant_id"]), p.ID, access.Resource{Tenant: tenant}, "databases.provision", denied) == nil
		t.Fatalf("unapproved retry accepted: state=%v, grant_id=%v, still runnable after revocation=%v; result=%v", row["state"], row["agent_grant_id"], allowed, result)
	}
}

func TestAgentLargeEntityResponse(t *testing.T) {
	s, c, p, tenant, allowed, _ := agentSecurityFixture(t)
	resource, space := registry.NewID(), registry.NewID()
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'review_metadata_only','registered','shared','ready',X'00')", resource, tenant, allowed); e != nil {
		t.Fatal(e)
	}
	c.want("POST", "/v1/tenants/"+tenant+"/spaces", registry.Row{"id": space, "slug": "review", "display_name": "Review", "initial_manager_user_id": p.ID, "database": registry.Row{"resource_id": resource}}, 0, 201)
	d := entity.Definition{Format: 1, Key: "big", Label: "Big", Rules: []entity.Rule{}, Permissions: []entity.Permission{}, Projection: []string{"choice_0"}}
	for field := 0; field < 32; field++ {
		f := entity.Field{Key: fmt.Sprintf("choice_%d", field), Label: "Choice", Type: "choice"}
		for i := 0; i < 100; i++ {
			f.Values = append(f.Values, strings.Repeat("v", 140)+fmt.Sprintf("%03d", i))
		}
		d.Fields = append(d.Fields, f)
	}
	if diags := d.Validate(); len(diags) > 0 {
		t.Fatal("invalid probe entity", diags[0])
	}
	encoded, _ := json.Marshal(d)
	if len(encoded) >= entity.MaxDefinitionBytes {
		t.Fatal("probe entity exceeds supported limit")
	}
	requestKey := registry.NewID()
	raw, _ := json.Marshal(agent.EntityInput{Target: &agent.Target{Kind: "space", ID: space}, Definition: &d, RequestKey: requestKey})
	result, e := agentTool(s, "lastapp_entity_save")(context.Background(), p, raw)
	if e != nil {
		t.Fatal(e)
	}
	receipt := result.(registry.Row)
	if receipt["committed"] != true || receipt["response_error"] == nil {
		t.Fatal("missing committed receipt and response limit", receipt)
	}
	key := agent.StableID(p, requestKey)
	for _, section := range []string{"definition", "json_schema", "create_schema", "update_schema", "typescript"} {
		args := agent.ReadInput{Resource: "entity", ID: key, Target: &agent.Target{Kind: "space", ID: space}, Section: section, Version: "draft"}
		if section != "definition" {
			args.Field = "choice_0"
		}
		b, _ := json.Marshal(args)
		v, err := agentTool(s, "lastapp_get")(context.Background(), p, b)
		if err != nil {
			t.Fatal(section, err)
		}
		if v.(registry.Row)["value"] == nil {
			t.Fatal("missing bounded section", section)
		}
	}
	replay, err := agentTool(s, "lastapp_entity_save")(context.Background(), p, raw)
	if err != nil || replay.(registry.Row)["replayed"] != true {
		t.Fatal("large committed write could not be replayed", err)
	}
}

func TestAgentTeamRecipientDiscovery(t *testing.T) {
	s, c, p, tenant, allowed, _ := agentSecurityFixture(t)
	resource, space, team := registry.NewID(), registry.NewID(), registry.NewID()
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'review_metadata_only','registered','shared','ready',X'00')", resource, tenant, allowed); e != nil {
		t.Fatal(e)
	}
	c.want("POST", "/v1/tenants/"+tenant+"/spaces", registry.Row{"id": space, "slug": "review", "display_name": "Review", "initial_manager_user_id": p.ID, "database": registry.Row{"resource_id": resource}}, 0, 201)
	c.want("POST", "/v1/tenants/"+tenant+"/teams", registry.Row{"id": team, "slug": "marketing", "display_name": "Marketing"}, 0, 201)
	if _, e := s.DB.Exec("UPDATE agent_grants SET scope_kind='space',scope_id=? WHERE id=?", space, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	g, e := s.ReadAgentGrant(context.Background(), s.DB, p.Agent.ID)
	if e != nil {
		t.Fatal(e)
	}
	p.Agent = g
	// Team discovery uses the same shared, scoped API as the console.
	scope, res, e := roles.ScopeOf(tenant, "", space, "")
	if e != nil {
		t.Fatal(e)
	}
	direct, e := s.authority.TeamRecipients(context.Background(), op.ActorFrom(context.Background(), p, registry.NewID()), scope, res, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(direct)
	if !strings.Contains(string(encoded), team) {
		t.Fatal("probe setup did not expose the normal team's recipient option")
	}
	viaRecipients, e := agentList(t, s, context.Background(), p, agent.ReadInput{Resource: "recipient", RecipientType: "team"})
	if e != nil {
		t.Fatal(e)
	}
	viaTeams, e := agentList(t, s, context.Background(), p, agent.ReadInput{Resource: "team"})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(viaRecipients)
	b, _ := json.Marshal(viaTeams)
	if !strings.Contains(string(a), team) && !strings.Contains(string(b), team) {
		t.Fatal("authorized Marketing team available in shared API but undiscoverable through either MCP recipient/team collection")
	}
}

func TestAgentRetryBindingAndQuota(t *testing.T) {
	s, _, p, tenant, target, _ := agentSecurityFixture(t)
	ctx := context.Background()
	makeResource := func(mode string) string {
		t.Helper()
		key := registry.NewID()
		var ownership any
		if mode == "managed" {
			ownership = make([]byte, 16)
		}
		if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc,ownership_token) VALUES(?,?,?,?,?,'shared','failed',X'00',?)", key, tenant, target, "fixture_"+strings.ReplaceAll(key, "-", ""), mode, ownership); e != nil {
			t.Fatal(e)
		}
		return key
	}
	retry := func(key string, revision int64) error {
		t.Helper()
		raw, _ := json.Marshal(agent.DatabaseInput{Action: "retry", ID: key, Revision: revision, RequestKey: registry.NewID()})
		_, e := agentTool(s, "lastapp_database_provision")(ctx, p, raw)
		return e
	}
	first, second, registered := makeResource("managed"), makeResource("managed"), makeResource("registered")
	if e := retry(registered, 1); e == nil {
		t.Fatal("agent retried registered credentials")
	}
	if e := retry(first, 1); e != nil {
		t.Fatal(e)
	}
	v, e := registry.QueryRow(ctx, s.DB, "SELECT agent_grant_id,agent_user_id FROM database_resources WHERE id=?", first)
	if e != nil {
		t.Fatal(e)
	}
	if registry.Str(v["agent_grant_id"]) != p.Agent.ID || registry.Str(v["agent_user_id"]) != p.ID {
		t.Fatal("retry lost current authorization", v)
	}
	if e = retry(second, 1); e == nil {
		t.Fatal("retry bypassed allocation cap")
	}
	if _, e = s.DB.Exec("UPDATE database_resources SET state='failed' WHERE id=?", first); e != nil {
		t.Fatal(e)
	}
	if e = retry(first, 2); e != nil {
		t.Fatal("same resource consumed another quota slot", e)
	}
	if _, e = s.DB.Exec("UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.AgentJobAllowed(ctx, s.DB, p.Agent.ID, p.ID, access.Resource{Tenant: tenant}, "databases.provision", target); e == nil {
		t.Fatal("revoked retry remains runnable")
	}
	if _, e = s.DB.Exec("UPDATE database_resources SET state='failed' WHERE id=?", first); e != nil {
		t.Fatal(e)
	}
	// A human takes ownership explicitly; no stale agent binding can strand it.
	_, e = s.storage.RetryDatabase(ctx, op.Actor{Principal: access.Principal{ID: p.ID}, RequestID: registry.NewID()}, tenant, first, 3)
	if e != nil {
		t.Fatal(e)
	}
	v, e = registry.QueryRow(ctx, s.DB, "SELECT agent_grant_id,agent_user_id FROM database_resources WHERE id=?", first)
	if e != nil {
		t.Fatal(e)
	}
	if v["agent_grant_id"] != nil || v["agent_user_id"] != nil {
		t.Fatal("human retry retained revoked delegation", v)
	}
}

// agentTool returns the adapter of a catalog tool by name, as the MCP server
// runs it after the grant check.
func agentTool(s *Server, name string) func(context.Context, access.Principal, json.RawMessage) (any, error) {
	for _, spec := range s.agent.Tools() {
		if spec.Name == name {
			return spec.Run
		}
	}
	panic("unknown agent tool " + name)
}

// agentList lists through the lastapp_list tool.
func agentList(t *testing.T, s *Server, ctx context.Context, p access.Principal, in agent.ReadInput) (registry.Row, error) {
	t.Helper()
	v, e := agentTool(s, "lastapp_list")(ctx, p, agentTestJSON(t, in))
	if e != nil {
		return nil, e
	}
	return v.(registry.Row), nil
}

func agentFixtureToken(t *testing.T, s *Server, p access.Principal) string {
	t.Helper()
	access := registry.Token(32)
	if _, e := s.DB.Exec("INSERT INTO agent_token_sessions(id,grant_id,grant_revision,family_id,access_hash,refresh_hash,access_expires_at) VALUES(?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 15 MINUTE))", registry.NewID(), p.Agent.ID, p.Agent.Revision, registry.NewID(), registry.TokenHash(access), registry.TokenHash(registry.Token(48))); e != nil {
		t.Fatal(e)
	}
	return access
}

func TestAgentTokenRevisionBinding(t *testing.T) {
	s, _, p, _, _, _ := agentSecurityFixture(t)
	accessToken := agentFixtureToken(t, s, p)
	r := httptest.NewRequest("POST", s.mcpURL(), nil)
	r.Header.Set("Authorization", "Bearer "+accessToken)
	snapshot, e := s.agent.Principal(r)
	if e != nil {
		t.Fatal(e)
	}
	if snapshot.Agent.Revision != p.Agent.Revision {
		t.Fatal("token loaded an unrelated revision")
	}
	if _, e = s.DB.Exec("UPDATE agent_grants SET permissions_json='[\"read\",\"access.manage\"]',revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	if access.AgentHas(snapshot.Agent, "access.manage") {
		t.Fatal("previously loaded principal changed with grant")
	}
	if e = s.CheckAgent(context.Background(), s.DB, snapshot, access.Resource{Tenant: p.Agent.Tenant}, false); e == nil {
		t.Fatal("stale snapshot remained authorized")
	}
	if _, e = s.agent.Principal(r); e == nil {
		t.Fatal("old token upgraded to changed grant")
	}
}

func TestAgentBootstrapIsolation(t *testing.T) {
	s, _, p, _, _, _ := agentSecurityFixture(t)
	access := agentFixtureToken(t, s, p)
	held, e := s.DB.Conn(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	const lock = "lastapp:agent-client-registration"
	var got int
	if e = held.QueryRowContext(context.Background(), "SELECT GET_LOCK(?,0)", lock).Scan(&got); e != nil || got != 1 {
		t.Fatal(e)
	}
	defer data.ReleasePinnedLock(held, lock)
	done := make(chan struct{}, agent.BootstrapConcurrent)
	for i := 0; i < agent.BootstrapConcurrent; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			r := httptest.NewRequest("POST", "/oauth/register", strings.NewReader(`{"redirect_uris":["http://localhost/cb"]}`))
			r.Header.Set("Content-Type", "application/json")
			s.agent.RegisterClient(httptest.NewRecorder(), r)
		}()
	}
	defer func() {
		for i := 0; i < agent.BootstrapConcurrent; i++ {
			<-done
		}
	}()
	deadline := time.Now().Add(time.Second)
	for len(s.agent.BootstrapSlots) < agent.BootstrapConcurrent && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(s.agent.BootstrapSlots) != agent.BootstrapConcurrent {
		t.Fatal("bootstrap fixture did not occupy its budget")
	}
	r := httptest.NewRequest("POST", s.mcpURL(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"regression","version":"1"}}}`))
	r.Header.Set("Authorization", "Bearer "+access)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	s.agent.MCPHandler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal("anonymous bootstrap blocked authenticated MCP", w.Code)
	}
	r = httptest.NewRequest("POST", "/oauth/register", strings.NewReader(`{}`))
	w = httptest.NewRecorder()
	s.agent.RegisterClient(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatal("anonymous admission budget was not bounded", w.Code)
	}
}

func TestAgentUnapprovedClientRetention(t *testing.T) {
	s, _, p, _, _, _ := agentSecurityFixture(t)
	ctx := context.Background()
	old := time.Now().UTC().Add(-agent.UnapprovedClientLifetime - time.Minute)
	args := []any{}
	tuples := []string{}
	for i := 0; i < agent.MaxClients; i++ {
		tuples = append(tuples, "(?,?,?,?)")
		args = append(args, "lastapp-client-"+registry.NewID(), `{}`, old, old)
	}
	if _, e := s.DB.Exec("INSERT INTO agent_oauth_clients(id,metadata_json,created_at,last_used_at) VALUES "+strings.Join(tuples, ","), args...); e != nil {
		t.Fatal(e)
	}
	// An approved and an in-progress client must survive cleanup, even with old timestamps.
	approved, pending := "lastapp-client-"+registry.NewID(), "lastapp-client-"+registry.NewID()
	for _, key := range []string{approved, pending} {
		metadata, _ := json.Marshal(config.AgentClient{ID: key, Name: "Protected", Redirects: []string{"http://localhost/cb"}})
		if _, e := s.DB.Exec("INSERT INTO agent_oauth_clients(id,metadata_json,created_at,last_used_at) VALUES(?,?,?,?)", key, metadata, old, old); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.DB.Exec("UPDATE agent_grants SET client_id=? WHERE id=?", approved, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("INSERT INTO agent_oauth_requests(id,client_id,client_name,redirect_uri,challenge,state_value,scopes,expires_at,resource_uri) VALUES(?,?,'Pending','http://localhost/cb',?,'','read',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 MINUTE),?)", registry.NewID(), pending, strings.Repeat("a", 43), s.mcpURL()); e != nil {
		t.Fatal(e)
	}
	if _, e := s.agent.ResolveClient(ctx, pending); e == nil {
		t.Fatal("unapproved registration could be kept alive indefinitely with authorize requests")
	}
	r := httptest.NewRequest("POST", "/oauth/register", strings.NewReader(`{"redirect_uris":["http://localhost/cb"]}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.agent.RegisterClient(w, r)
	if w.Code != 201 {
		t.Fatal("expired anonymous quota blocked a new registration", w.Code)
	}
	if e := agent.CleanupClients(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{approved, pending} {
		var n int
		if e := s.DB.QueryRow("SELECT COUNT(*) FROM agent_oauth_clients WHERE id=?", key).Scan(&n); e != nil || n != 1 {
			t.Fatal("cleanup evicted approved/in-progress identity", e)
		}
	}
	if _, e := s.agent.ResolveClient(ctx, approved); e != nil {
		t.Fatal("approved client could not reconnect", e)
	}
}

// TestActorIsTheAgentSource pins the Actor as the only source of an
// operation's delegated call: an Actor that omits the agent call already in
// its context fails instead of running with delegation checks but no receipt.
func TestActorIsTheAgentSource(t *testing.T) {
	r := &op.Runner{Access: &access.Checker{}}
	call := &access.AgentCall{Operation: "lastapp_create", Key: registry.NewID()}
	mcp := context.WithValue(context.Background(), access.AgentContextKey{}, call)
	for name, a := range map[string]op.Actor{
		"missing agent": {Principal: access.Principal{ID: registry.NewID()}, RequestID: registry.NewID()},
		"other agent":   {Principal: access.Principal{ID: registry.NewID()}, RequestID: registry.NewID(), Agent: &access.AgentCall{Operation: "lastapp_create"}},
	} {
		if e := r.Mutate(mcp, a, "apps.create", access.Resource{Tenant: registry.NewID()}, func(*sql.Tx) error { t.Fatal(name, "mutation ran"); return nil }); !errors.Is(e, op.ErrActorAgentMismatch) {
			t.Fatal(name, "mutation accepted a mismatched actor", e)
		}
		if e := r.Authorize(mcp, a, "apps.read", access.Resource{Tenant: registry.NewID()}); !errors.Is(e, op.ErrActorAgentMismatch) {
			t.Fatal(name, "read authorization accepted a mismatched actor", e)
		}
		if _, e := r.Scopes(mcp, a, registry.NewID(), "apps.read"); !errors.Is(e, op.ErrActorAgentMismatch) {
			t.Fatal(name, "scope lookup accepted a mismatched actor", e)
		}
	}
	// The Actor's own call is what the authorization checks see.
	ctx, e := op.WithAgentCall(context.Background(), call)
	if e != nil || access.AgentCallFrom(ctx) != call {
		t.Fatal("actor agent call was not exposed", e)
	}
	if ctx, e = op.WithAgentCall(mcp, call); e != nil || access.AgentCallFrom(ctx) != call {
		t.Fatal("matching actor was rejected", e)
	}
	if ctx, e = op.WithAgentCall(context.Background(), nil); e != nil || access.AgentCallFrom(ctx) != nil {
		t.Fatal("human actor gained an agent call", e)
	}
}
