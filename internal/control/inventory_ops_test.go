package control

import (
	"context"
	"errors"
	"testing"

	"github.com/aantich/lastapp/internal/entity"
)

func wantProblem(t *testing.T, e error, status int, code string) {
	t.Helper()
	o := classify(e)
	if e == nil || o.Status != status || o.Code != code {
		t.Fatalf("got %v (%d %s), want %d %s", e, o.Status, o.Code, status, code)
	}
}

// agentCall is the delegated call an MCP adapter builds for one mutation.
func agentCall(p Principal, operation string, input any, permissions ...string) *AgentCall {
	return &AgentCall{Grant: p.Agent, Permission: permissions, Operation: operation, Key: newID(), Hash: entity.Hash(Row{"operation": operation, "input": input}), ResultRef: Row{"type": "test"}}
}

func TestInventoryOperations(t *testing.T) {
	s, _, p, tenant, _, app := agentSpaceFixture(t)
	ctx := context.Background()
	human := Actor{Principal: Principal{ID: p.ID}, RequestID: newID()}

	// Console actor: create, revision rules and audit attribution.
	in := microappCreateInput{namedInput: namedInput{ID: newID(), Slug: "ops", Name: "Ops"}, Order: 2}
	ref, e := s.inv.CreateMicroapp(ctx, human, tenant, app, in)
	if e != nil || ref.ID != in.ID || ref.Revision != 1 {
		t.Fatal("console create", ref, e)
	}
	var request, details string
	if e = s.DB.QueryRow("SELECT request_id,details_json FROM audit_events WHERE action='microapp.created' AND resource_id=?", in.ID).Scan(&request, &details); e != nil || request != human.RequestID || details != `{"app_id":"`+app+`"}` {
		t.Fatal("console audit attribution", request, details, e)
	}
	name := "Renamed"
	_, e = s.inv.UpdateMicroapp(ctx, human, tenant, in.ID, metadataPatch{Name: &name}, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.inv.UpdateMicroapp(ctx, human, tenant, in.ID, metadataPatch{Name: &name}, 7)
	wantProblem(t, e, 412, "stale_revision")
	if ref, e = s.inv.UpdateMicroapp(ctx, human, tenant, in.ID, metadataPatch{Name: &name}, 1); e != nil || ref.Revision != 2 {
		t.Fatal("console update", ref, e)
	}
	page, e := s.inv.Microapps(ctx, human, tenant, app, PageRequest{Limit: 1})
	if e != nil || len(page.Items) != 1 || page.Pagination.Limit != 1 || page.Pagination.HasMore {
		t.Fatal("console list", page, e)
	}
	_, e = s.inv.Microapps(ctx, human, tenant, app, PageRequest{Limit: 201})
	wantProblem(t, e, 400, "invalid_request")

	// Denied scope: an outsider and a non-platform plan author.
	var outsider string
	if e = s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='outsider'").Scan(&outsider); e != nil {
		t.Fatal(e)
	}
	_, e = s.inv.CreateMicroapp(ctx, Actor{Principal: Principal{ID: outsider}, RequestID: newID()}, tenant, app, microappCreateInput{namedInput: namedInput{ID: newID(), Slug: "denied", Name: "Denied"}})
	wantProblem(t, e, 404, "not_found")
	_, e = s.inv.Plans(ctx, human, PageRequest{})
	wantProblem(t, e, 403, "forbidden")

	// Agent actor: receipt, replay, conflicting reuse and grant attribution.
	agentIn := microappCreateInput{namedInput: namedInput{ID: newID(), Slug: "agent", Name: "Agent"}}
	call := agentCall(p, "lastapp_create", agentIn, "resources.write")
	agent := Actor{Principal: p, RequestID: newID(), Agent: call}
	if _, e = s.inv.CreateMicroapp(ctx, agent, tenant, app, agentIn); e != nil || !call.Committed {
		t.Fatal("agent create", e)
	}
	if e = s.DB.QueryRow("SELECT details_json FROM audit_events WHERE action='microapp.created' AND resource_id=?", agentIn.ID).Scan(&details); e != nil || details != `{"agent_client_id":"review-client","agent_grant_id":"`+p.Agent.ID+`","details":{"app_id":"`+app+`"}}` {
		t.Fatal("agent audit attribution", details, e)
	}
	replay := &AgentCall{Grant: call.Grant, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: call.Hash, ResultRef: call.ResultRef}
	if _, e = s.inv.CreateMicroapp(ctx, Actor{Principal: p, RequestID: newID(), Agent: replay}, tenant, app, agentIn); !errors.Is(e, errAgentReplay) {
		t.Fatal("replay executed again", e)
	}
	reused := &AgentCall{Grant: call.Grant, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: "different"}
	_, e = s.inv.UpdateMicroapp(ctx, Actor{Principal: p, RequestID: newID(), Agent: reused}, tenant, agentIn.ID, metadataPatch{Name: &name}, 1)
	wantProblem(t, e, 409, "conflict")
	missingScope := agentCall(p, "lastapp_lifecycle", nil, "lifecycle.manage")
	archived := stateArchived
	_, e = s.inv.UpdateMicroapp(ctx, Actor{Principal: p, RequestID: newID(), Agent: missingScope}, tenant, agentIn.ID, metadataPatch{State: &archived}, 1)
	wantProblem(t, e, 403, "needs_authorization")
	if o := classify(e); o.Authority == nil || len(o.Authority.Missing) != 1 || o.Authority.Missing[0] != "lifecycle.manage" {
		t.Fatal("missing scopes not classified", e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM agent_operation_receipts WHERE grant_id=?", p.Agent.ID).Scan(&n); e != nil || n != 1 {
		t.Fatal("receipts", n, e)
	}
}

func TestTenantInventoryOperations(t *testing.T) {
	s, _, operator, owner, _ := foundationFixture(t)
	ctx := context.Background()
	platform := Actor{Principal: Principal{ID: operator, Platform: true}, RequestID: newID()}
	human := Actor{Principal: Principal{ID: owner}, RequestID: newID()}
	tenant := tenantCreateInput{ID: newID(), Slug: "ops-tenant", Name: "Ops", Plan: "10000000-0000-4000-8000-000000000001", Admin: owner}
	_, e := s.inv.CreateTenant(ctx, human, tenant)
	wantProblem(t, e, 403, "forbidden")
	if ref, e := s.inv.CreateTenant(ctx, platform, tenant); e != nil || ref.Revision != 1 {
		t.Fatal("platform create", ref, e)
	}
	_, e = s.inv.UpdateTenant(ctx, platform, tenant.ID, tenantPatch{}, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.inv.UpdateTenant(ctx, platform, tenant.ID, tenantPatch{}, 9)
	wantProblem(t, e, 412, "stale_revision")
	_, e = s.inv.UpdateTenantProfile(ctx, platform, tenant.ID, "Platform rename", 1)
	wantProblem(t, e, 403, "tenant_authority_required")
	if ref, e := s.inv.UpdateTenantProfile(ctx, human, tenant.ID, "Ops company", 1); e != nil || ref.Revision != 2 {
		t.Fatal("profile update", ref, e)
	}

	target, resource := newID(), newID()
	if _, e = s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode) VALUES (?,'Ops target','127.0.0.2',33317,'disabled')", target); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'ops_metadata_only','registered','shared','ready',X'00')", resource, tenant.ID, target); e != nil {
		t.Fatal(e)
	}
	space := spaceCreateInput{namedInput: namedInput{ID: newID(), Slug: "ops", Name: "Ops"}, Manager: owner}
	space.Database.Resource = resource
	if _, e = s.inv.CreateSpace(ctx, human, tenant.ID, space); e != nil {
		t.Fatal("console space", e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND action IN ('space.created','grant.created')", human.RequestID).Scan(&n); e != nil || n != 2 {
		t.Fatal("space audit request attribution", n, e)
	}

	grant := newID()
	if _, e = s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'ops','Ops client',?,'tenant','[\"read\",\"resources.write\",\"production.deploy\"]','{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", grant, owner, tenant.ID, s.mcpURL()); e != nil {
		t.Fatal(e)
	}
	g, e := s.readAgentGrant(ctx, s.DB, grant)
	if e != nil {
		t.Fatal(e)
	}
	p := Principal{ID: owner, Agent: g}
	app := appCreateInput{namedInput: namedInput{ID: newID(), Slug: "ops", Name: "Ops"}, Manager: owner}
	call := agentCall(p, "lastapp_create", app, "resources.write")
	agent := Actor{Principal: p, RequestID: newID(), Agent: call}
	if _, e = s.inv.CreateApp(ctx, agent, tenant.ID, space.ID, app); e != nil || !call.Committed {
		t.Fatal("agent app", e)
	}
	// The initial manager assignment is attributed to the grant too, although
	// the operation was called with a context that does not carry the call.
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND JSON_VALUE(details_json,'$.agent_grant_id')=?", agent.RequestID, grant).Scan(&n); e != nil || n != 2 {
		t.Fatal("agent attribution of app and initial assignment", n, e)
	}
	replay := &AgentCall{Grant: g, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: call.Hash}
	if _, e = s.inv.CreateApp(ctx, Actor{Principal: p, RequestID: newID(), Agent: replay}, tenant.ID, space.ID, app); !errors.Is(e, errAgentReplay) {
		t.Fatal("replay executed again", e)
	}
	page, e := s.inv.Apps(ctx, agent, tenant.ID, space.ID, PageRequest{})
	if e != nil || len(page.Items) != 1 {
		t.Fatal("agent app list", page, e)
	}
	// A Tenant-bound delegation never reaches platform inventory.
	_, e = s.inv.Tenants(ctx, agent, PageRequest{})
	wantProblem(t, e, 404, "not_found")
}

func TestMembershipOperations(t *testing.T) {
	s, _, p, tenant, _, _ := agentSecurityFixture(t)
	ctx := context.Background()
	human := Actor{Principal: Principal{ID: p.ID}, RequestID: newID()}
	var operator, joe string
	for subject, dest := range map[string]*string{"operator": &operator, "outsider": &joe} {
		if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject=?", subject).Scan(dest); e != nil {
			t.Fatal(e)
		}
	}
	platform := Actor{Principal: Principal{ID: operator, Platform: true}, RequestID: newID()}

	// Console actor: membership, team and member revisions.
	if v, e := s.members.AddMembership(ctx, human, tenant, accountChoice{User: joe}); e != nil || v.UserID != joe {
		t.Fatal("console membership", v, e)
	}
	_, e := s.members.UpdateMembership(ctx, human, tenant, joe, stateDisabled, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.members.UpdateMembership(ctx, human, tenant, p.ID, stateDisabled, 1)
	wantProblem(t, e, 409, "conflict") // the last Tenant administrator stays active
	team := namedInput{ID: newID(), Slug: "ops", Name: "Ops"}
	if ref, e := s.members.CreateTeam(ctx, human, tenant, team); e != nil || ref.Revision != 1 {
		t.Fatal("console team", ref, e)
	}
	// Every member change, including add, needs the team's current revision.
	_, e = s.members.AddTeamMember(ctx, human, tenant, team.ID, joe, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.members.AddTeamMember(ctx, human, tenant, team.ID, joe, 2)
	wantProblem(t, e, 412, "stale_revision")
	if _, e = s.members.AddTeamMember(ctx, human, tenant, team.ID, joe, 1); e != nil {
		t.Fatal("console team member", e)
	}
	if _, e = s.members.SetTeamMemberState(ctx, human, tenant, team.ID, joe, stateDisabled, 2); e != nil {
		t.Fatal("console team member state", e)
	}
	page, e := s.members.TeamMembers(ctx, human, tenant, team.ID, PageRequest{})
	if e != nil || len(page.Items) != 1 || str(page.Items[0]["state"]) != stateDisabled {
		t.Fatal("console team members", page, e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND tenant_id=?", human.RequestID, tenant).Scan(&n); e != nil || n != 4 {
		t.Fatal("console audit attribution", n, e)
	}

	// Denied scope: Tenant administrators are platform operations.
	_, e = s.members.Administrators(ctx, human, tenant, PageRequest{})
	wantProblem(t, e, 403, "forbidden")
	_, e = s.members.AddAdministrator(ctx, human, tenant, accountChoice{User: joe})
	wantProblem(t, e, 403, "forbidden")
	if v, e := s.members.AddAdministrator(ctx, platform, tenant, accountChoice{User: joe}); e != nil || v.UserID != joe {
		t.Fatal("platform administrator", v, e)
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND action='grant.created' AND actor_user_id=?", platform.RequestID, operator).Scan(&n); e != nil || n != 1 {
		t.Fatal("administrator grant audit", n, e)
	}

	// Agent actor: access.manage is required; replay and grant attribution.
	call := agentCall(p, "lastapp_membership", "remove", "access.manage")
	_, e = s.members.RemoveTeamMember(ctx, Actor{Principal: p, RequestID: newID(), Agent: call}, tenant, team.ID, joe, 3)
	wantProblem(t, e, 403, "needs_authorization")
	if _, e = s.DB.Exec("UPDATE agent_grants SET permissions_json='[\"read\",\"access.manage\"]',revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	if p.Agent, e = s.readAgentGrant(ctx, s.DB, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	call = agentCall(p, "lastapp_membership", "remove", "access.manage")
	agent := Actor{Principal: p, RequestID: newID(), Agent: call}
	if _, e = s.members.RemoveTeamMember(ctx, agent, tenant, team.ID, joe, 3); e != nil || !call.Committed {
		t.Fatal("agent remove", e)
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND JSON_VALUE(details_json,'$.agent_grant_id')=?", agent.RequestID, p.Agent.ID).Scan(&n); e != nil || n != 1 {
		t.Fatal("agent audit attribution", n, e)
	}
	replay := &AgentCall{Grant: p.Agent, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: call.Hash}
	if _, e = s.members.RemoveTeamMember(ctx, Actor{Principal: p, RequestID: newID(), Agent: replay}, tenant, team.ID, joe, 4); !errors.Is(e, errAgentReplay) {
		t.Fatal("replay executed again", e)
	}
	if e = s.DB.QueryRow("SELECT revision FROM groups WHERE id=?", team.ID).Scan(&n); e != nil || n != 4 {
		t.Fatal("team revision after one removal", n, e)
	}
}
