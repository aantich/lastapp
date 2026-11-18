package control

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/agent"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

// The audit trail and the global user list check authority through
// op.Runner.Authorize like every other operation: the agent grant's scope and
// the calling tool's permissions narrow the human's authority, both through
// the MCP path (call in ctx) and for an explicit agent Actor on a plain ctx.
func TestAgentAuditAndUserReadsNarrowed(t *testing.T) {
	s, c, operator, owner, _ := foundationFixture(t)
	tenant, team := registry.NewID(), registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "agent-reads-" + tenant[:8], "display_name": "Agent reads", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	c.as(owner).want("POST", "/v1/tenants/"+tenant+"/teams", registry.Row{"id": team, "slug": "ops", "display_name": "Ops"}, 0, 201)
	grant := func(user, kind string, tenant, scope any, permissions ...string) access.Principal {
		t.Helper()
		id := registry.NewID()
		perms, _ := json.Marshal(permissions)
		if _, e := s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,scope_id,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'reads-client','Reads client',?,?,?,?,'{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", id, user, tenant, kind, scope, perms, s.mcpURL()); e != nil {
			t.Fatal(e)
		}
		g, e := s.ReadAgentGrant(context.Background(), s.DB, id)
		if e != nil {
			t.Fatal(e)
		}
		return access.Principal{ID: user, Agent: g}
	}
	// call is the agent call the MCP dispatcher builds for lastapp_list.
	call := func(p access.Principal) *access.AgentCall {
		return &access.AgentCall{Grant: p.Agent, Permission: []string{"read"}, Operation: "lastapp_list"}
	}
	mcpCtx := func(p access.Principal) (context.Context, *access.AgentCall) {
		a := call(p)
		return context.WithValue(context.Background(), access.AgentContextKey{}, a), a
	}
	needsAuthorization := func(e error) bool {
		var authority *access.AgentAuthorityError
		return errors.As(e, &authority)
	}
	notFound := func(e error) bool {
		var ae *registry.APIError
		return errors.As(e, &ae) && ae.Status == 404
	}
	audits := func(ctx context.Context, a op.Actor) error {
		_, e := s.authority.Audits(ctx, a, tenant, op.PageRequest{}, roles.AuditFilter{})
		return e
	}
	users := func(ctx context.Context, a op.Actor) error {
		_, e := s.inv.Users(ctx, a, op.PageRequest{})
		return e
	}

	// Tenant audit trail: allowed for a Tenant-wide grant with read.
	full := grant(owner, "tenant", tenant, nil, "read")
	ctx, _ := mcpCtx(full)
	if _, e := agentList(t, s, ctx, full, agent.ReadInput{Resource: "audit_event"}); e != nil {
		t.Fatal("Tenant-wide read grant cannot read the audit trail", e)
	}
	// A grant without the tool's permission is refused, via MCP and via an
	// explicit agent Actor on a context that carries no call.
	noRead := grant(owner, "tenant", tenant, nil, "resources.write")
	ctx, a := mcpCtx(noRead)
	if _, e := agentList(t, s, ctx, noRead, agent.ReadInput{Resource: "audit_event"}); !needsAuthorization(e) {
		t.Fatal("audit read accepted an agent grant without read", e)
	}
	if e := audits(context.Background(), op.Actor{Principal: noRead, RequestID: registry.NewID(), Agent: a}); !needsAuthorization(e) {
		t.Fatal("audit read ignored the actor's agent call", e)
	}
	// A team-scoped grant does not reach the Tenant-wide audit trail although
	// its human is the Tenant administrator.
	teamOnly := grant(owner, "team", tenant, team, "read")
	ctx, a = mcpCtx(teamOnly)
	if e := audits(ctx, op.Actor{Principal: teamOnly, RequestID: registry.NewID(), Agent: a}); !notFound(e) {
		t.Fatal("team-scoped agent grant read the Tenant audit trail", e)
	}
	if e := audits(context.Background(), op.ActorFrom(context.Background(), access.Principal{ID: owner}, registry.NewID())); e != nil {
		t.Fatal("the human Tenant administrator lost audit access", e)
	}

	// Global user list: platform administrators only, and only through a
	// platform grant that carries read.
	platform := grant(operator, "platform", nil, nil, "read")
	ctx, a = mcpCtx(platform)
	if e := users(ctx, op.Actor{Principal: platform, RequestID: registry.NewID(), Agent: a}); e != nil {
		t.Fatal("platform read grant cannot list users", e)
	}
	platformNoRead := grant(operator, "platform", nil, nil, "resources.write")
	ctx, a = mcpCtx(platformNoRead)
	if e := users(ctx, op.Actor{Principal: platformNoRead, RequestID: registry.NewID(), Agent: a}); !needsAuthorization(e) {
		t.Fatal("user list accepted a platform grant without read", e)
	}
	if e := users(context.Background(), op.Actor{Principal: platformNoRead, RequestID: registry.NewID(), Agent: a}); !needsAuthorization(e) {
		t.Fatal("user list ignored the actor's agent call", e)
	}
	// The platform administrator's Tenant-scoped grant does not list users.
	tenantOnly := grant(operator, "tenant", tenant, nil, "read")
	ctx, a = mcpCtx(tenantOnly)
	if e := users(ctx, op.Actor{Principal: tenantOnly, RequestID: registry.NewID(), Agent: a}); !notFound(e) {
		t.Fatal("Tenant-scoped grant of a platform administrator listed every user", e)
	}
}
