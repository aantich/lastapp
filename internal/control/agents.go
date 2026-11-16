package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

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
	agentMaxReceipts              = 10000
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

type agentLimits struct {
	TargetIDs    []string `json:"target_ids"`
	MaxDatabases int      `json:"max_databases"`
}
type agentGrant struct {
	ID          string      `json:"id"`
	User        string      `json:"user_id"`
	Client      string      `json:"client_id"`
	Name        string      `json:"client_name"`
	Tenant      string      `json:"tenant_id"`
	Kind        string      `json:"scope_kind"`
	Scope       string      `json:"scope_id"`
	Permissions []string    `json:"permissions"`
	Limits      agentLimits `json:"limits"`
	Revision    int64       `json:"revision"`
	Audience    string      `json:"resource"`
}

// Audit identity is metadata, never an authorization credential. Workers carry
// it from persisted jobs, including after revocation or recovery.
type (
	agentAuditKey      struct{}
	agentAuditIdentity struct{ Grant, Client string }
)

func agentAuditContext(ctx context.Context, grant, client string) context.Context {
	if grant == "" {
		return ctx
	}
	return context.WithValue(ctx, agentAuditKey{}, agentAuditIdentity{Grant: grant, Client: client})
}

type agentAuthorityError struct{ Missing []string }

func (e *agentAuthorityError) Error() string {
	return "Additional delegated scopes required: " + strings.Join(e.Missing, ", ")
}

func (e *agentAuthorityError) Unwrap() error {
	return registry.Problem(403, "needs_authorization", e.Error())
}

var errAgentReplay = errors.New("agent operation already committed")

func agentHas(g *agentGrant, permission string) bool {
	return g != nil && slices.Contains(g.Permissions, permission)
}

func (s *Server) checkAgent(ctx context.Context, q registry.Queryer, p Principal, res Resource, exact bool) error {
	g := p.Agent
	if g == nil {
		return nil
	}
	var revision int64
	e := q.QueryRowContext(ctx, `SELECT g.revision FROM agent_grants g JOIN users u ON u.id=g.user_id AND u.state='active'
 LEFT JOIN local_accounts l ON l.user_id=u.id WHERE g.id=? AND g.user_id=? AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6)
 AND COALESCE(l.must_change_password,0)=0 AND g.audience_uri=? LOCK IN SHARE MODE`, g.ID, p.ID, s.mcpURL()).Scan(&revision)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if e != nil || revision != g.Revision {
		return registry.Problem(401, "agent_access_revoked", "Agent access expired or changed; reconnect.")
	}
	if a := agentCallFrom(ctx); a != nil {
		if e := requireAgent(g, a.Permission...); e != nil {
			return e
		}
	}
	if g.Kind == "platform" {
		return nil
	} // Existing platformControlAction still forbids Tenant authoring.
	if res.Tenant != g.Tenant {
		return registry.Missing()
	}
	if e = s.member(ctx, q, Principal{ID: p.ID}, g.Tenant, false); e != nil {
		return e
	}
	switch g.Kind {
	case "tenant":
		return nil
	case "team":
		if !exact || res.Team == g.Scope {
			return nil
		}
	case "space":
		workspaceScope := ""
		_ = q.QueryRowContext(ctx, "SELECT production_space_id FROM development_workspaces WHERE backing_space_id=? AND owner_user_id=? AND owner_agent_grant_id=? AND state<>'archived'", res.Space, p.ID, g.ID).Scan(&workspaceScope)
		if !exact || res.Space == g.Scope || workspaceScope == g.Scope {
			return nil
		}
	case "app":
		if !exact || res.App == g.Scope {
			return nil
		}
	}
	return registry.Missing()
}

func (s *Server) agentScope(ctx context.Context, q registry.Queryer, p Principal, sc scopeSet) scopeSet {
	g := p.Agent
	if g == nil || g.Kind == "tenant" || g.Kind == "platform" {
		return sc
	}
	out := scopeSet{}
	switch g.Kind {
	case "team":
		if sc.All || slices.Contains(sc.Teams, g.Scope) {
			out.Teams = []string{g.Scope}
		}
	case "space":
		own, _ := registry.ScanStrings(ctx, q, "SELECT backing_space_id FROM development_workspaces WHERE production_space_id=? AND owner_user_id=? AND owner_agent_grant_id=? AND state='ready'", g.Scope, p.ID, g.ID)
		for _, space := range own {
			if sc.All || slices.Contains(sc.Spaces, space) {
				out.Spaces = append(out.Spaces, space)
			}
		}
		if sc.All || slices.Contains(sc.Spaces, g.Scope) {
			out.Spaces = append(out.Spaces, g.Scope)
		}
		for _, app := range sc.Apps {
			var parent string
			if q.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", app, g.Tenant).Scan(&parent) == nil && parent == g.Scope {
				out.Apps = append(out.Apps, app)
			}
		}
	case "app":
		var parent string
		if q.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", g.Scope, g.Tenant).Scan(&parent) == nil && (sc.All || slices.Contains(sc.Apps, g.Scope) || slices.Contains(sc.Spaces, parent)) {
			out.Apps = []string{g.Scope}
		}
	}
	return out
}

func agentMutationBegin(ctx context.Context, tx *sql.Tx, a *AgentCall) error {
	if a == nil {
		return nil
	}
	if a.Key == "" {
		return registry.Bad("request_key is required for every mutation")
	}
	var previous string
	e := tx.QueryRowContext(ctx, "SELECT input_hash FROM agent_operation_receipts WHERE grant_id=? AND request_key=? FOR UPDATE", a.Grant.ID, a.Key).Scan(&previous)
	if e == nil {
		if previous != a.Hash {
			return registry.Conflict("request_key was already used for different arguments")
		}
		return errAgentReplay
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	var n int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_operation_receipts WHERE grant_id=?", a.Grant.ID).Scan(&n); e != nil {
		return e
	}
	if n >= agentMaxReceipts {
		return registry.Problem(403, "agent_limit", "This connection has reached its operation limit. Approve a new connection.")
	}
	return nil
}

func agentMutationFinish(ctx context.Context, tx *sql.Tx, a *AgentCall) error {
	if a == nil {
		return nil
	}
	b, e := json.Marshal(a.ResultRef)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO agent_operation_receipts(grant_id,request_key,input_hash,operation,result_json) VALUES(?,?,?,?,?)", a.Grant.ID, a.Key, a.Hash, a.Operation, b)
	if e == nil {
		_, e = tx.ExecContext(ctx, "UPDATE agent_grants SET last_used_at=UTC_TIMESTAMP(6) WHERE id=?", a.Grant.ID)
	}
	return e
}

func (s *Server) agentJobAllowed(ctx context.Context, q registry.Queryer, grant, user string, res Resource, permission string, targets ...string) error {
	if grant == "" {
		return nil
	}
	g, e := s.readAgentGrant(ctx, q, grant)
	if e != nil {
		return e
	}
	p := Principal{ID: user, Agent: g}
	ctx = context.WithValue(ctx, agentContextKey{}, &AgentCall{Grant: g, Permission: []string{permission}})
	if e = s.checkAgent(ctx, q, p, res, true); e != nil {
		return e
	}
	if permission == "databases.provision" && (len(targets) != 1 || !slices.Contains(g.Limits.TargetIDs, targets[0])) {
		return registry.Problem(403, "agent_limit", "The queued target is no longer approved for this connection.")
	}
	action := "entity.publish"
	if permission == "production.deploy" {
		action = "production.deploy"
	}
	if permission == "workspaces.write" {
		action = "workspace.manage"
	}
	if permission == "databases.provision" {
		action = "database.manage"
	}
	return s.authorize(ctx, q, p, action, res, true)
}

func (s *Server) readAgentGrant(ctx context.Context, q registry.Queryer, key string) (*agentGrant, error) {
	g, e := scanAgentGrant(ctx, q, "SELECT "+agentGrantColumns+" FROM agent_grants g WHERE g.id=?", []any{key})
	if e != nil {
		return nil, e
	}
	return g, nil
}

// agentGrantColumns selects an agentGrant from agent_grants g.
const agentGrantColumns = "g.id,g.user_id,g.client_id,g.client_name,COALESCE(g.tenant_id,''),g.scope_kind,COALESCE(g.scope_id,''),g.revision,g.audience_uri,g.permissions_json,g.limits_json"

// scanAgentGrant reads a grant selected by agentGrantColumns, followed by
// any extra columns scanned into extra. A read failure returns no grant; a
// grant whose stored permissions or limits do not decode is returned with
// the decoding error.
func scanAgentGrant(ctx context.Context, q registry.Queryer, query string, args []any, extra ...any) (*agentGrant, error) {
	g := &agentGrant{}
	var permissions, limits registry.JSONValue
	dest := []any{&g.ID, &g.User, &g.Client, &g.Name, &g.Tenant, &g.Kind, &g.Scope, &g.Revision, &g.Audience, &permissions, &limits}
	if e := registry.Lookup(ctx, q, query, args, append(dest, extra...)...); e != nil {
		return nil, e
	}
	b, _ := json.Marshal(permissions)
	if e := json.Unmarshal(b, &g.Permissions); e != nil {
		return g, e
	}
	b, _ = json.Marshal(limits)
	return g, json.Unmarshal(b, &g.Limits)
}

func (s *Server) agentPrincipal(r *http.Request) (Principal, error) {
	var p Principal
	raw := r.Header.Get("Authorization")
	if len(raw) < 8 || len(raw) > 512 || raw[:7] != "Bearer " {
		return p, registry.Problem(401, "unauthenticated", "An MCP access token is required.")
	}
	// Load the token-bound grant and its permissions in one statement/snapshot.
	// A later grant edit fails checkAgent; it cannot upgrade this access token.
	var e error
	p.Agent, e = scanAgentGrant(r.Context(), s.DB, `SELECT `+agentGrantColumns+`,u.display_name,COALESCE(u.email,'') FROM agent_token_sessions se JOIN agent_grants g ON g.id=se.grant_id AND g.revision=se.grant_revision JOIN users u ON u.id=g.user_id AND u.state='active' WHERE se.access_hash=? AND se.state='active' AND se.access_expires_at>UTC_TIMESTAMP(6) AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6)`, []any{hash(raw[7:])}, &p.Name, &p.Email)
	if p.Agent == nil {
		return p, registry.Problem(401, "unauthenticated", "Invalid or expired MCP access token.")
	}
	if e != nil {
		return p, e
	}
	p.ID = p.Agent.User
	p.Platform, e = isPlatform(r.Context(), s.DB, p.ID)
	if e != nil {
		return p, e
	}
	return p, s.checkAgent(r.Context(), s.DB, p, Resource{Tenant: p.Agent.Tenant}, false)
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
