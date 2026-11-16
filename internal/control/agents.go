package control

import (
	"net/http"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/registry"
)

// Agent permissions are delegation ceilings, never independent user roles.
var agentPermissions = []registry.Row{
	{"code": "workspaces.write", "label": "Create isolated workspaces", "description": "Allocate bounded, empty development databases on the Space target or platform-configured development target and manage your own workspaces. Never copies production records."},
	{"code": "workspaces.test", "label": "Test workspace Apps", "description": "Issue short-lived browser test sessions for your own workspace Apps. Normal business ACLs still apply. No production records."},
	{"code": "releases.write", "label": "Upload and test releases", "description": "Upload bounded static builds and capture immutable development App releases."},
	{"code": "production.deploy", "label": "Deploy to production", "description": "Explicitly authorize production configuration changes and activation of reviewed App releases. Separate from workspace development."},
	{"code": "read", "label": "Inspect configuration", "description": "Read authorized inventories, contracts, roles, documentation and operational audit; no business records."},
	{"code": "resources.write", "label": "Create and edit resources", "description": "Create/edit Spaces, Apps, micro-app registrations and teams. An initial manager defaults to you; choosing another manager also requires access-management authority. Reusing existing shared storage needs no provisioning grant; actual allocation does."},
	{"code": "entities.draft", "label": "Edit entity drafts", "description": "Create/edit full entity drafts, including proposed rules and ACLs; never activates them."},
	{"code": "entities.check", "label": "Validate and test drafts", "description": "Validate definitions and simulate synthetic cases; no business-record access."},
	{"code": "entities.publish", "label": "Publish entities, rules and ACLs", "description": "Activate the captured entity definition, including its schema, business rules and permissions. This can change data access."},
	{"code": "access.manage", "label": "Manage access", "description": "Manage custom roles, role assignments, memberships and App origin/access settings within your existing authority."},
	{"code": "databases.provision", "label": "Provision databases", "description": "Create managed databases on explicitly approved targets, including allocation during Space creation; retry existing jobs."},
	{"code": "lifecycle.manage", "label": "Change resource lifecycle", "description": "Archive, disable, restore or retire supported resources. Also requires access-management authority; changing entity lifecycle requires publication authority."},
	{"code": "platform.manage", "label": "Manage platform inventory", "description": "Manage Tenants and plans only with an explicitly approved platform grant and current platform authority. Does not permit Tenant authoring."},
}

var (
	agentDefaultPermissions = []string{"read"}
	agentPresets            = []registry.Row{
		{"label": "Workspace author", "permissions": []string{"read", "workspaces.write", "workspaces.test", "releases.write", "resources.write", "entities.draft", "entities.check", "entities.publish", "access.manage", "lifecycle.manage"}},
		{"label": "Inspect only", "permissions": []string{"read"}},
		{"label": "App author", "permissions": []string{"read", "resources.write"}},
		{"label": "Entity author", "permissions": []string{"read", "entities.draft", "entities.check"}},
	}
)

func agentPermissionKinds(code string) []string {
	switch code {
	case "read", "access.manage", "lifecycle.manage":
		return []string{"platform", "tenant", "team", "space", "app"}
	case "resources.write":
		return []string{"tenant", "team", "space", "app"}
	case "workspaces.write", "workspaces.test", "releases.write", "production.deploy", "entities.draft", "entities.check", "entities.publish":
		return []string{"tenant", "space"}
	case "databases.provision":
		return []string{"tenant"}
	case "platform.manage":
		return []string{"platform"}
	}
	return nil
}

func agentPermissionCatalog() []registry.Row {
	out := []registry.Row{}
	for _, permission := range agentPermissions {
		p := registry.Row{}
		for k, v := range permission {
			p[k] = v
		}
		p["scope_kinds"] = agentPermissionKinds(registry.Str(p["code"]))
		out = append(out, p)
	}
	return out
}

func agentScopeCodes() []string {
	out := make([]string, 0, len(agentPermissions))
	for _, permission := range agentPermissions {
		out = append(out, registry.Str(permission["code"]))
	}
	return out
}

const (
	agentAccessLifetime           = 15 * time.Minute
	agentGrantDays                = 7
	agentMaxGrantDays             = 30
	agentCodeLifetime             = 5 * time.Minute
	agentMaxResponse              = 2 << 20
	agentMaxDatabases             = 20
	agentMaxTargets               = 32
	agentMaxConnections           = 100
	agentMaxConcurrent            = 16
	agentMaxTokenRotations        = 4096
	agentMaxSyntheticCases        = 10
	agentMaxClients               = 1000
	agentUnapprovedClientLifetime = 10 * time.Minute
	agentBootstrapConcurrent      = 4
	agentTokenConcurrent          = 4
	agentValidationConcurrent     = 8
	agentClientIdleLifetime       = 30 * 24 * time.Hour
	agentMaxClientRedirects       = 16
	agentMaxClientMetadata        = 65536
)

func (s *Server) agentPrincipal(r *http.Request) (access.Principal, error) {
	var p access.Principal
	raw := r.Header.Get("Authorization")
	if len(raw) < 8 || len(raw) > 512 || raw[:7] != "Bearer " {
		return p, registry.Problem(401, "unauthenticated", "An MCP access token is required.")
	}
	// Load the token-bound grant and its permissions in one statement/snapshot.
	// A later grant edit fails checkAgent; it cannot upgrade this access token.
	var e error
	p.Agent, e = access.ScanAgentGrant(r.Context(), s.DB, `SELECT `+access.AgentGrantColumns+`,u.display_name,COALESCE(u.email,'') FROM agent_token_sessions se JOIN agent_grants g ON g.id=se.grant_id AND g.revision=se.grant_revision JOIN users u ON u.id=g.user_id AND u.state='active' WHERE se.access_hash=? AND se.state='active' AND se.access_expires_at>UTC_TIMESTAMP(6) AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6)`, []any{hash(raw[7:])}, &p.Name, &p.Email)
	if p.Agent == nil {
		return p, registry.Problem(401, "unauthenticated", "Invalid or expired MCP access token.")
	}
	if e != nil {
		return p, e
	}
	p.ID = p.Agent.User
	p.Platform, e = access.IsPlatform(r.Context(), s.DB, p.ID)
	if e != nil {
		return p, e
	}
	return p, s.CheckAgent(r.Context(), s.DB, p, access.Resource{Tenant: p.Agent.Tenant}, false)
}

// Admission happens before token/database work. Anonymous discovery/metadata
// fetching cannot consume authenticated MCP slots or all registry connections.
func (s *Server) admitAgent(w http.ResponseWriter, r *http.Request, limiter *requestLimiter, slots chan struct{}, prefix string, burst int, refill float64) bool {
	if !limiter.allowRate(prefix+":"+s.loginClientIP(r), time.Now(), float64(burst), refill) {
		writeError(w, r, rateLimited(w))
		return false
	}
	select {
	case slots <- struct{}{}:
		return true
	default:
		writeError(w, r, rateLimited(w))
		return false
	}
}
