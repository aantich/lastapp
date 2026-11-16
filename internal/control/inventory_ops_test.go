package control

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

func wantProblem(t *testing.T, e error, status int, code string) {
	t.Helper()
	o := op.Classify(e)
	if e == nil || o.Status != status || o.Code != code {
		t.Fatalf("got %v (%d %s), want %d %s", e, o.Status, o.Code, status, code)
	}
}

// agentCall is the delegated call an MCP adapter builds for one mutation.
// wire is v as an MCP client decodes it.
func wire(t *testing.T, v any) registry.Row {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	out := registry.Row{}
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}

func agentCall(p access.Principal, operation string, input any, permissions ...string) *access.AgentCall {
	return &access.AgentCall{Grant: p.Agent, Permission: permissions, Operation: operation, Key: registry.NewID(), Hash: entity.Hash(registry.Row{"operation": operation, "input": input}), ResultRef: registry.Row{"type": "test"}}
}

func TestInventoryOperations(t *testing.T) {
	s, _, p, tenant, _, app := agentSpaceFixture(t)
	ctx := context.Background()
	human := op.Actor{Principal: access.Principal{ID: p.ID}, RequestID: registry.NewID()}

	// Console actor: create, revision rules and audit attribution.
	in := microappCreateInput{NamedInput: op.NamedInput{ID: registry.NewID(), Slug: "ops", Name: "Ops"}, Order: 2}
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
	page, e := s.inv.Microapps(ctx, human, tenant, app, op.PageRequest{Limit: 1})
	if e != nil || len(page.Items) != 1 || page.Pagination.Limit != 1 || page.Pagination.HasMore {
		t.Fatal("console list", page, e)
	}
	_, e = s.inv.Microapps(ctx, human, tenant, app, op.PageRequest{Limit: 201})
	wantProblem(t, e, 400, "invalid_request")

	// Denied scope: an outsider and a non-platform plan author.
	var outsider string
	if e = s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='outsider'").Scan(&outsider); e != nil {
		t.Fatal(e)
	}
	_, e = s.inv.CreateMicroapp(ctx, op.Actor{Principal: access.Principal{ID: outsider}, RequestID: registry.NewID()}, tenant, app, microappCreateInput{NamedInput: op.NamedInput{ID: registry.NewID(), Slug: "denied", Name: "Denied"}})
	wantProblem(t, e, 404, "not_found")
	_, e = s.inv.Plans(ctx, human, op.PageRequest{})
	wantProblem(t, e, 403, "forbidden")

	// Agent actor: receipt, replay, conflicting reuse and grant attribution.
	agentIn := microappCreateInput{NamedInput: op.NamedInput{ID: registry.NewID(), Slug: "agent", Name: "Agent"}}
	call := agentCall(p, "lastapp_create", agentIn, "resources.write")
	agent := op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}
	if _, e = s.inv.CreateMicroapp(ctx, agent, tenant, app, agentIn); e != nil || !call.Committed {
		t.Fatal("agent create", e)
	}
	if e = s.DB.QueryRow("SELECT details_json FROM audit_events WHERE action='microapp.created' AND resource_id=?", agentIn.ID).Scan(&details); e != nil || details != `{"agent_client_id":"review-client","agent_grant_id":"`+p.Agent.ID+`","details":{"app_id":"`+app+`"}}` {
		t.Fatal("agent audit attribution", details, e)
	}
	replay := &access.AgentCall{Grant: call.Grant, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: call.Hash, ResultRef: call.ResultRef}
	if _, e = s.inv.CreateMicroapp(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: replay}, tenant, app, agentIn); !errors.Is(e, op.ErrAgentReplay) {
		t.Fatal("replay executed again", e)
	}
	reused := &access.AgentCall{Grant: call.Grant, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: "different"}
	_, e = s.inv.UpdateMicroapp(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: reused}, tenant, agentIn.ID, metadataPatch{Name: &name}, 1)
	wantProblem(t, e, 409, "conflict")
	missingScope := agentCall(p, "lastapp_lifecycle", nil, "lifecycle.manage")
	archived := registry.StateArchived
	_, e = s.inv.UpdateMicroapp(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: missingScope}, tenant, agentIn.ID, metadataPatch{State: &archived}, 1)
	wantProblem(t, e, 403, "needs_authorization")
	if o := op.Classify(e); o.Authority == nil || len(o.Authority.Missing) != 1 || o.Authority.Missing[0] != "lifecycle.manage" {
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
	platform := op.Actor{Principal: access.Principal{ID: operator, Platform: true}, RequestID: registry.NewID()}
	human := op.Actor{Principal: access.Principal{ID: owner}, RequestID: registry.NewID()}
	tenant := tenantCreateInput{ID: registry.NewID(), Slug: "ops-tenant", Name: "Ops", Plan: "10000000-0000-4000-8000-000000000001", Admin: owner}
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

	target, resource := registry.NewID(), registry.NewID()
	if _, e = s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode) VALUES (?,'Ops target','127.0.0.2',33317,'disabled')", target); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'ops_metadata_only','registered','shared','ready',X'00')", resource, tenant.ID, target); e != nil {
		t.Fatal(e)
	}
	space := spaceCreateInput{NamedInput: op.NamedInput{ID: registry.NewID(), Slug: "ops", Name: "Ops"}, Manager: owner}
	space.Database.Resource = resource
	if _, e = s.inv.CreateSpace(ctx, human, tenant.ID, space); e != nil {
		t.Fatal("console space", e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND action IN ('space.created','grant.created')", human.RequestID).Scan(&n); e != nil || n != 2 {
		t.Fatal("space audit request attribution", n, e)
	}

	grant := registry.NewID()
	if _, e = s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'ops','Ops client',?,'tenant','[\"read\",\"resources.write\",\"production.deploy\"]','{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", grant, owner, tenant.ID, s.mcpURL()); e != nil {
		t.Fatal(e)
	}
	g, e := s.ReadAgentGrant(ctx, s.DB, grant)
	if e != nil {
		t.Fatal(e)
	}
	p := access.Principal{ID: owner, Agent: g}
	app := appCreateInput{NamedInput: op.NamedInput{ID: registry.NewID(), Slug: "ops", Name: "Ops"}, Manager: owner}
	call := agentCall(p, "lastapp_create", app, "resources.write")
	agent := op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}
	if _, e = s.inv.CreateApp(ctx, agent, tenant.ID, space.ID, app); e != nil || !call.Committed {
		t.Fatal("agent app", e)
	}
	// The initial manager assignment is attributed to the grant too, although
	// the operation was called with a context that does not carry the call.
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND JSON_VALUE(details_json,'$.agent_grant_id')=?", agent.RequestID, grant).Scan(&n); e != nil || n != 2 {
		t.Fatal("agent attribution of app and initial assignment", n, e)
	}
	replay := &access.AgentCall{Grant: g, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: call.Hash}
	if _, e = s.inv.CreateApp(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: replay}, tenant.ID, space.ID, app); !errors.Is(e, op.ErrAgentReplay) {
		t.Fatal("replay executed again", e)
	}
	page, e := s.inv.Apps(ctx, agent, tenant.ID, space.ID, op.PageRequest{})
	if e != nil || len(page.Items) != 1 {
		t.Fatal("agent app list", page, e)
	}
	// A Tenant-bound delegation never reaches platform inventory.
	_, e = s.inv.Tenants(ctx, agent, op.PageRequest{})
	wantProblem(t, e, 404, "not_found")
}

func TestMembershipOperations(t *testing.T) {
	s, _, p, tenant, _, _ := agentSecurityFixture(t)
	ctx := context.Background()
	human := op.Actor{Principal: access.Principal{ID: p.ID}, RequestID: registry.NewID()}
	var operator, joe string
	for subject, dest := range map[string]*string{"operator": &operator, "outsider": &joe} {
		if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject=?", subject).Scan(dest); e != nil {
			t.Fatal(e)
		}
	}
	platform := op.Actor{Principal: access.Principal{ID: operator, Platform: true}, RequestID: registry.NewID()}

	// Console actor: membership, team and member revisions.
	if v, e := s.members.AddMembership(ctx, human, tenant, accountChoice{User: joe}); e != nil || v.UserID != joe {
		t.Fatal("console membership", v, e)
	}
	_, e := s.members.UpdateMembership(ctx, human, tenant, joe, registry.StateDisabled, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.members.UpdateMembership(ctx, human, tenant, p.ID, registry.StateDisabled, 1)
	wantProblem(t, e, 409, "conflict") // the last Tenant administrator stays active
	team := op.NamedInput{ID: registry.NewID(), Slug: "ops", Name: "Ops"}
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
	if _, e = s.members.SetTeamMemberState(ctx, human, tenant, team.ID, joe, registry.StateDisabled, 2); e != nil {
		t.Fatal("console team member state", e)
	}
	page, e := s.members.TeamMembers(ctx, human, tenant, team.ID, op.PageRequest{})
	if e != nil || len(page.Items) != 1 || page.Items[0].State != registry.StateDisabled {
		t.Fatal("console team members", page, e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND tenant_id=?", human.RequestID, tenant).Scan(&n); e != nil || n != 4 {
		t.Fatal("console audit attribution", n, e)
	}

	// Denied scope: Tenant administrators are platform operations.
	_, e = s.members.Administrators(ctx, human, tenant, op.PageRequest{})
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
	_, e = s.members.RemoveTeamMember(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}, tenant, team.ID, joe, 3)
	wantProblem(t, e, 403, "needs_authorization")
	if _, e = s.DB.Exec("UPDATE agent_grants SET permissions_json='[\"read\",\"access.manage\"]',revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	if p.Agent, e = s.ReadAgentGrant(ctx, s.DB, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	call = agentCall(p, "lastapp_membership", "remove", "access.manage")
	agent := op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}
	if _, e = s.members.RemoveTeamMember(ctx, agent, tenant, team.ID, joe, 3); e != nil || !call.Committed {
		t.Fatal("agent remove", e)
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND JSON_VALUE(details_json,'$.agent_grant_id')=?", agent.RequestID, p.Agent.ID).Scan(&n); e != nil || n != 1 {
		t.Fatal("agent audit attribution", n, e)
	}
	replay := &access.AgentCall{Grant: p.Agent, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: call.Hash}
	if _, e = s.members.RemoveTeamMember(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: replay}, tenant, team.ID, joe, 4); !errors.Is(e, op.ErrAgentReplay) {
		t.Fatal("replay executed again", e)
	}
	if e = s.DB.QueryRow("SELECT revision FROM groups WHERE id=?", team.ID).Scan(&n); e != nil || n != 4 {
		t.Fatal("team revision after one removal", n, e)
	}
}

func TestAuthorityOperations(t *testing.T) {
	s, _, p, tenant, space, _ := agentSpaceFixture(t)
	ctx := context.Background()
	human := op.Actor{Principal: access.Principal{ID: p.ID}, RequestID: registry.NewID()}
	scope, res, e := scopeOf(tenant, "", space, "")
	if e != nil {
		t.Fatal(e)
	}

	// Console actor: custom role, revisions and the fixed-role guard.
	role := roleInput{ID: registry.NewID(), Code: "reviewer", Name: "Reviewer"}
	ref, e := s.authority.CreateRole(ctx, human, scope, res, role)
	if e != nil || ref.Revision != 1 {
		t.Fatal("console role", ref, e)
	}
	tenantScope, tenantRes, _ := scopeOf(tenant, "", "", "")
	_, e = s.authority.CreateRole(ctx, human, tenantScope, tenantRes, roleInput{ID: registry.NewID(), Code: "x", Name: "X"})
	wantProblem(t, e, 400, "invalid_request")
	name := "Lead reviewer"
	_, e = s.authority.UpdateRole(ctx, human, scope, res, role.ID, rolePatch{Name: &name}, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.authority.UpdateRole(ctx, human, scope, res, role.ID, rolePatch{Name: &name}, 2)
	wantProblem(t, e, 412, "stale_revision")
	if ref, e = s.authority.UpdateRole(ctx, human, scope, res, role.ID, rolePatch{Name: &name}, 1); e != nil || ref.Revision != 2 {
		t.Fatal("console role update", ref, e)
	}
	_, e = s.authority.UpdateRole(ctx, human, scope, res, authz.SpaceManagerRole, rolePatch{Name: &name}, 1)
	wantProblem(t, e, 403, "protected_role")

	// Grant and revoke the custom role; the audit carries the console request.
	grant := assignmentInput{ID: registry.NewID(), User: p.ID, Role: role.ID}
	if ref, e = s.authority.Grant(ctx, human, scope, res, grant); e != nil || ref.Revision != 1 {
		t.Fatal("console grant", ref, e)
	}
	page, e := s.authority.Assignments(ctx, human, scope, res, op.PageRequest{})
	if e != nil || len(page.Items) != 2 {
		t.Fatal("console assignments", page, e)
	}
	if v, e := s.authority.Assignment(ctx, human, scope, res, grant.ID); e != nil || v.RoleID != role.ID {
		t.Fatal("console assignment", v, e)
	}
	_, e = s.authority.Revoke(ctx, human, scope, res, grant.ID, 0)
	wantProblem(t, e, 428, "revision_required")
	if _, e = s.authority.Revoke(ctx, human, scope, res, grant.ID, 1); e != nil {
		t.Fatal("console revoke", e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND action IN ('role.created','role.updated','grant.created','grant.deleted')", human.RequestID).Scan(&n); e != nil || n != 4 {
		t.Fatal("console audit attribution", n, e)
	}

	// Agent actor: access.manage is required and the receipt records the grant.
	call := agentCall(p, "lastapp_assignment", "grant", "access.manage")
	_, e = s.authority.Grant(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}, scope, res, assignmentInput{ID: registry.NewID(), User: p.ID, Role: role.ID})
	wantProblem(t, e, 403, "needs_authorization")
	if _, e = s.DB.Exec("UPDATE agent_grants SET permissions_json='[\"read\",\"access.manage\",\"production.deploy\"]',revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	if p.Agent, e = s.ReadAgentGrant(ctx, s.DB, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	call = agentCall(p, "lastapp_assignment", "grant", "access.manage")
	agent := op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}
	team := assignmentInput{ID: registry.NewID(), User: p.ID, Role: role.ID}
	if _, e = s.authority.Grant(ctx, agent, scope, res, team); e != nil || !call.Committed {
		t.Fatal("agent grant", e)
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND JSON_VALUE(details_json,'$.agent_grant_id')=?", agent.RequestID, p.Agent.ID).Scan(&n); e != nil || n != 1 {
		t.Fatal("agent audit attribution", n, e)
	}
	recipients, e := s.authority.Recipients(ctx, agent, scope, res, op.PageRequest{})
	if e != nil || len(recipients.Items) == 0 {
		t.Fatal("agent recipients", recipients, e)
	}
	_, e = s.authority.Recipients(ctx, agent, authz.Scope{Kind: authz.Platform}, access.Resource{}, op.PageRequest{})
	wantProblem(t, e, 400, "invalid_request")
}

func TestStorageOperations(t *testing.T) {
	s, _, p, tenant, allowed, denied := agentSecurityFixture(t)
	ctx := context.Background()
	human := op.Actor{Principal: access.Principal{ID: p.ID}, RequestID: registry.NewID()}
	var operator string
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='operator'").Scan(&operator); e != nil {
		t.Fatal(e)
	}
	platform := op.Actor{Principal: access.Principal{ID: operator, Platform: true}, RequestID: registry.NewID()}

	// Targets are platform inventory; a Tenant sees only active targets.
	_, e := s.storage.Targets(ctx, human, op.PageRequest{})
	wantProblem(t, e, 403, "forbidden")
	name := "Renamed target"
	_, e = s.storage.UpdateTarget(ctx, platform, denied, targetPatch{Name: &name}, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.storage.UpdateTarget(ctx, platform, denied, targetPatch{Name: &name}, 2)
	wantProblem(t, e, 412, "stale_revision")
	disabled := registry.StateDisabled
	if ref, e := s.storage.UpdateTarget(ctx, platform, denied, targetPatch{State: &disabled}, 1); e != nil || ref.Revision != 2 {
		t.Fatal("platform target update", ref, e)
	}
	available, e := s.storage.AvailableTargets(ctx, human, tenant, true, op.PageRequest{})
	if e != nil || len(available.Items) != 1 || available.Items[0].ID != allowed {
		t.Fatal("available targets", available, e)
	}
	_, e = s.storage.CreateTarget(ctx, platform, targetInput{ID: registry.NewID(), Name: "Bad", Host: "db.example", Port: 3306, TLS: "none"})
	wantProblem(t, e, 400, "invalid_request")

	// Resources: registration, reads, revisions and the retirement guard.
	record := registry.NewID()
	in := databaseInput{ID: record, Mode: "registered", Layout: "shared", Target: allowed, Database: "review_registered", Credentials: credentials{Username: "u", Password: "p"}}
	if ref, e := s.storage.CreateDatabase(ctx, human, tenant, in); e != nil || ref.ID != record {
		t.Fatal("register database", ref, e)
	}
	if v, e := s.storage.Database(ctx, human, tenant, record); e != nil || v.State != "pending" {
		t.Fatal("database", v, e)
	}
	if page, e := s.storage.Databases(ctx, human, tenant, op.PageRequest{}); e != nil || len(page.Items) == 0 {
		t.Fatal("databases", page, e)
	}
	_, e = s.storage.UpdateDatabase(ctx, human, tenant, record, databaseChange{State: registry.StateRetired}, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.storage.UpdateDatabase(ctx, human, tenant, record, databaseChange{State: registry.StateRetired}, 1)
	wantProblem(t, e, 409, "conflict") // still pending
	if _, e = s.DB.Exec("UPDATE database_resources SET state='failed' WHERE id=?", record); e != nil {
		t.Fatal(e)
	}
	if ref, e := s.storage.UpdateDatabase(ctx, human, tenant, record, databaseChange{State: registry.StateRetired}, 1); e != nil || ref.Revision != 2 {
		t.Fatal("retire database", ref, e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND action IN ('database.requested','database.retired')", human.RequestID).Scan(&n); e != nil || n != 2 {
		t.Fatal("console audit attribution", n, e)
	}

	// Agent actor: managed provisioning is limited to approved targets.
	call := agentCall(p, "lastapp_database", "provision", "databases.provision")
	_, e = s.storage.CreateDatabase(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}, tenant, databaseInput{ID: registry.NewID(), Mode: "managed", Layout: "shared", Target: denied})
	wantProblem(t, e, 403, "needs_authorization")
}

func TestAuthoringOperations(t *testing.T) {
	s, _, p, _, space, app := agentSpaceFixture(t)
	ctx := context.Background()
	human := op.Actor{Principal: access.Principal{ID: p.ID}, RequestID: registry.NewID()}

	// Console actor: draft lifecycle, revisions and generated views.
	d := entity.Definition{Format: 1, Key: "ticket", Label: "Ticket", Fields: []entity.Field{{Key: "title", Label: "Title", Type: "text", MaxLength: 100}}, Projection: []string{"title"}, Rules: []entity.Rule{}, Permissions: []entity.Permission{}}
	created, e := s.authoring.CreateEntity(ctx, human, space, entityCreateInput{Definition: d})
	if e != nil || created.DraftContract == nil || created.Revision != 1 {
		t.Fatal("console entity", created, e)
	}
	key := created.ID
	d.Label = "Support ticket"
	_, e = s.authoring.SaveEntity(ctx, human, key, entitySaveInput{Definition: d}, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.authoring.SaveEntity(ctx, human, key, entitySaveInput{Definition: d}, 2)
	wantProblem(t, e, 412, "stale_revision")
	if saved, e := s.authoring.SaveEntity(ctx, human, key, entitySaveInput{Definition: d}, 1); e != nil || saved.Label != d.Label {
		t.Fatal("console save", saved, e)
	}
	if v, e := s.authoring.Entity(ctx, human, key, entityView{Section: "definition"}); e != nil || v.(registry.Row)["section"] != "definition" {
		t.Fatal("entity section", v, e)
	}
	_, e = s.authoring.Entity(ctx, human, key, entityView{Field: "x"})
	wantProblem(t, e, 400, "invalid_request")
	if v, e := s.authoring.ValidateEntity(ctx, human, key, d, "diagnostics"); e != nil || v["schema"] != nil {
		t.Fatal("validate", v, e)
	}
	if page, e := s.authoring.Entities(ctx, human, space); e != nil || len(page["items"].([]entityItem)) != 1 {
		t.Fatal("entities", page, e)
	}
	_, e = s.authoring.PublishEntity(ctx, human, key, 0)
	wantProblem(t, e, 428, "revision_required")
	queued, e := s.authoring.PublishEntity(ctx, human, key, 2)
	if e != nil || queued["state"] != "queued" {
		t.Fatal("publish", queued, e)
	}
	if v, e := s.authoring.Publication(ctx, human, key, registry.Str(queued["id"])); e != nil || v.ID != registry.Str(queued["id"]) {
		t.Fatal("publication", v, e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE request_id=? AND action IN ('entity.created','entity.saved')", human.RequestID).Scan(&n); e != nil || n != 2 {
		t.Fatal("console audit attribution", n, e)
	}

	// App data access and contracts.
	access, e := s.authoring.AppAccess(ctx, human, app)
	if e != nil {
		t.Fatal("app access", e)
	}
	_, e = s.authoring.SaveAppAccess(ctx, human, app, entity.AppAccess{Inherit: true}, 0)
	wantProblem(t, e, 428, "revision_required")
	if v, e := s.authoring.SaveAppAccess(ctx, human, app, entity.AppAccess{Inherit: true}, registry.Integer(access["revision"])); e != nil || registry.Integer(v["revision"]) != registry.Integer(access["revision"])+1 {
		t.Fatal("save app access", v, e)
	}
	contracts, e := s.authoring.AppContracts(ctx, human, app, contractView{Section: "inventory"})
	if e != nil || contracts["pagination"] == nil {
		t.Fatal("app contracts", contracts, e)
	}

	// Agent actor: publication needs the delegated entities.publish scope.
	call := agentCall(p, "lastapp_entity_publish", "publish", "entities.publish")
	_, e = s.authoring.PublishEntity(ctx, op.Actor{Principal: p, RequestID: registry.NewID(), Agent: call}, key, 3)
	wantProblem(t, e, 403, "needs_authorization")
	if call.Committed {
		t.Fatal("rejected publication committed its receipt")
	}
}
