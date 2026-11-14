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
)

// Agent permissions are delegation ceilings, never independent user roles.
var agentPermissions = []Row{
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
var agentDefaultPermissions = []string{"read"}
var agentPresets = []Row{
	{"label": "Workspace author", "permissions": []string{"read", "workspaces.write", "workspaces.test", "releases.write", "resources.write", "entities.draft", "entities.check", "entities.publish", "access.manage", "lifecycle.manage"}},
	{"label": "Inspect only", "permissions": []string{"read"}},
	{"label": "App author", "permissions": []string{"read", "resources.write"}},
	{"label": "Entity author", "permissions": []string{"read", "entities.draft", "entities.check"}},
}

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
func agentPermissionCatalog() []Row {
	out := []Row{}
	for _, permission := range agentPermissions {
		p := Row{}
		for k, v := range permission {
			p[k] = v
		}
		p["scope_kinds"] = agentPermissionKinds(str(p["code"]))
		out = append(out, p)
	}
	return out
}

func agentScopeCodes() []string {
	out := make([]string, 0, len(agentPermissions))
	for _, permission := range agentPermissions {
		out = append(out, str(permission["code"]))
	}
	return out
}

const agentAccessLifetime = 15 * time.Minute
const agentGrantDays = 7
const agentMaxGrantDays = 30
const agentCodeLifetime = 5 * time.Minute
const agentMaxResponse = 2 << 20
const agentMaxDatabases = 20
const agentMaxTargets = 32
const agentMaxConnections = 100
const agentMaxReceipts = 10000
const agentMaxConcurrent = 16
const agentMaxTokenRotations = 4096
const agentMaxSyntheticCases = 10
const agentMaxClients = 1000
const agentUnapprovedClientLifetime = 10 * time.Minute
const agentBootstrapConcurrent = 4
const agentTokenConcurrent = 4
const agentValidationConcurrent = 8
const agentClientIdleLifetime = 30 * 24 * time.Hour
const agentMaxClientRedirects = 16
const agentMaxClientMetadata = 65536

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
type agentAuditKey struct{}
type agentAuditIdentity struct{ Grant, Client string }

func agentAuditContext(ctx context.Context, grant, client string) context.Context {
	if grant == "" {
		return ctx
	}
	return context.WithValue(ctx, agentAuditKey{}, agentAuditIdentity{Grant: grant, Client: client})
}

type agentContextKey struct{}
type agentExecution struct {
	Grant                *agentGrant
	Permission           []string
	Operation, Key, Hash string
	Result               Row
	Committed            bool
}
type agentAuthorityError struct{ Missing []string }

func (e *agentAuthorityError) Error() string {
	return "Additional delegated scopes required: " + strings.Join(e.Missing, ", ")
}
func (e *agentAuthorityError) Unwrap() error { return problem(403, "needs_authorization", e.Error()) }

var errAgentReplay = errors.New("agent operation already committed")

func agentHas(g *agentGrant, permission string) bool {
	return g != nil && slices.Contains(g.Permissions, permission)
}
func (s *Server) checkAgent(ctx context.Context, q Queryer, p Principal, res Resource, exact bool) error {
	g := p.Agent
	if g == nil {
		return nil
	}
	var revision int64
	e := q.QueryRowContext(ctx, `SELECT g.revision FROM agent_grants g JOIN users u ON u.id=g.user_id AND u.state='active'
 LEFT JOIN local_accounts l ON l.user_id=u.id WHERE g.id=? AND g.user_id=? AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6)
 AND COALESCE(l.must_change_password,0)=0 AND g.audience_uri=? LOCK IN SHARE MODE`, g.ID, p.ID, s.mcpURL()).Scan(&revision)
	if e != nil && e != sql.ErrNoRows {
		return e
	}
	if e != nil || revision != g.Revision {
		return problem(401, "agent_access_revoked", "Agent access expired or changed; reconnect.")
	}
	if a, ok := ctx.Value(agentContextKey{}).(*agentExecution); ok {
		if e := requireAgent(g, a.Permission...); e != nil {
			return e
		}
	}
	if g.Kind == "platform" {
		return nil
	} // Existing platformControlAction still forbids Tenant authoring.
	if res.Tenant != g.Tenant {
		return missing()
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
	return missing()
}

func (s *Server) agentScope(ctx context.Context, q Queryer, p Principal, sc scopeSet) scopeSet {
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
		own, _ := rows(ctx, q, "SELECT backing_space_id FROM development_workspaces WHERE production_space_id=? AND owner_user_id=? AND owner_agent_grant_id=? AND state='ready'", g.Scope, p.ID, g.ID)
		for _, v := range own {
			if sc.All || slices.Contains(sc.Spaces, str(v["backing_space_id"])) {
				out.Spaces = append(out.Spaces, str(v["backing_space_id"]))
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

func (s *Server) agentMutationBegin(ctx context.Context, tx *sql.Tx) error {
	a, ok := ctx.Value(agentContextKey{}).(*agentExecution)
	if !ok {
		return nil
	}
	if a.Key == "" {
		return bad("request_key is required for every mutation")
	}
	var previous string
	e := tx.QueryRowContext(ctx, "SELECT input_hash FROM agent_operation_receipts WHERE grant_id=? AND request_key=? FOR UPDATE", a.Grant.ID, a.Key).Scan(&previous)
	if e == nil {
		if previous != a.Hash {
			return conflict("request_key was already used for different arguments")
		}
		return errAgentReplay
	}
	if e != sql.ErrNoRows {
		return e
	}
	var n int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_operation_receipts WHERE grant_id=?", a.Grant.ID).Scan(&n); e != nil {
		return e
	}
	if n >= agentMaxReceipts {
		return problem(403, "agent_limit", "This connection has reached its operation limit. Approve a new connection.")
	}
	return nil
}
func (s *Server) agentMutationFinish(ctx context.Context, tx *sql.Tx) error {
	a, ok := ctx.Value(agentContextKey{}).(*agentExecution)
	if !ok {
		return nil
	}
	b, e := json.Marshal(a.Result)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO agent_operation_receipts(grant_id,request_key,input_hash,operation,result_json) VALUES(?,?,?,?,?)", a.Grant.ID, a.Key, a.Hash, a.Operation, b)
	if e == nil {
		_, e = tx.ExecContext(ctx, "UPDATE agent_grants SET last_used_at=UTC_TIMESTAMP(6) WHERE id=?", a.Grant.ID)
	}
	return e
}
func operationID(ctx context.Context) string {
	if a, ok := ctx.Value(agentContextKey{}).(*agentExecution); ok {
		return str(a.Result["id"])
	}
	return newID()
}
func (s *Server) agentJobAllowed(ctx context.Context, q Queryer, grant, user string, res Resource, permission string, targets ...string) error {
	if grant == "" {
		return nil
	}
	g, e := s.readAgentGrant(ctx, q, grant)
	if e != nil {
		return e
	}
	p := Principal{ID: user, Agent: g}
	ctx = context.WithValue(ctx, agentContextKey{}, &agentExecution{Grant: g, Permission: []string{permission}})
	if e = s.checkAgent(ctx, q, p, res, true); e != nil {
		return e
	}
	if permission == "databases.provision" && (len(targets) != 1 || !slices.Contains(g.Limits.TargetIDs, targets[0])) {
		return problem(403, "agent_limit", "The queued target is no longer approved for this connection.")
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
func (s *Server) readAgentGrant(ctx context.Context, q Queryer, key string) (*agentGrant, error) {
	v, e := row(ctx, q, "SELECT id,user_id,client_id,client_name,COALESCE(tenant_id,'') AS tenant_id,scope_kind,COALESCE(scope_id,'') AS scope_id,permissions_json,limits_json,revision,audience_uri FROM agent_grants WHERE id=?", key)
	if e != nil {
		return nil, e
	}
	return agentGrantFromRow(v)
}
func agentGrantFromRow(v Row) (*agentGrant, error) {
	g := &agentGrant{ID: str(v["id"]), User: str(v["user_id"]), Client: str(v["client_id"]), Name: str(v["client_name"]), Tenant: str(v["tenant_id"]), Kind: str(v["scope_kind"]), Scope: str(v["scope_id"]), Revision: integer(v["revision"])}
	g.Audience = str(v["audience_uri"])
	b, _ := json.Marshal(v["permissions_json"])
	if e := json.Unmarshal(b, &g.Permissions); e != nil {
		return nil, e
	}
	b, _ = json.Marshal(v["limits_json"])
	return g, json.Unmarshal(b, &g.Limits)
}
func (s *Server) agentPrincipal(r *http.Request) (Principal, error) {
	var p Principal
	raw := r.Header.Get("Authorization")
	if len(raw) < 8 || len(raw) > 512 || raw[:7] != "Bearer " {
		return p, problem(401, "unauthenticated", "An MCP access token is required.")
	}
	// Load the token-bound grant and its permissions in one statement/snapshot.
	// A later grant edit fails checkAgent; it cannot upgrade this access token.
	v, e := row(r.Context(), s.DB, `SELECT g.id,g.user_id,g.client_id,g.client_name,COALESCE(g.tenant_id,'') AS tenant_id,g.scope_kind,COALESCE(g.scope_id,'') AS scope_id,g.permissions_json,g.limits_json,g.revision,g.audience_uri,u.display_name,COALESCE(u.email,'') AS email FROM agent_token_sessions se JOIN agent_grants g ON g.id=se.grant_id AND g.revision=se.grant_revision JOIN users u ON u.id=g.user_id AND u.state='active' WHERE se.access_hash=? AND se.state='active' AND se.access_expires_at>UTC_TIMESTAMP(6) AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6)`, hash(raw[7:]))
	if e != nil {
		return p, problem(401, "unauthenticated", "Invalid or expired MCP access token.")
	}
	p.ID, p.Name, p.Email = str(v["user_id"]), str(v["display_name"]), str(v["email"])
	p.Agent, e = agentGrantFromRow(v)
	if e != nil {
		return p, e
	}
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
