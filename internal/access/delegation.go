package access

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/aantich/lastapp/internal/registry"
)

// AgentCall is one delegated MCP tool execution: the grant, the delegated
// permissions it requires, the receipt identity (request key and input hash),
// the compact result reference stored in the receipt and whether it committed.
type AgentCall struct {
	Grant                *AgentGrant
	Permission           []string
	Operation, Key, Hash string
	ResultRef            registry.Row
	Committed            bool
}

type AgentContextKey struct{}

func AgentCallFrom(ctx context.Context) *AgentCall {
	a, _ := ctx.Value(AgentContextKey{}).(*AgentCall)
	return a
}

type AgentLimits struct {
	TargetIDs    []string `json:"target_ids"`
	MaxDatabases int      `json:"max_databases"`
}

type AgentGrant struct {
	ID          string      `json:"id"`
	User        string      `json:"user_id"`
	Client      string      `json:"client_id"`
	Name        string      `json:"client_name"`
	Tenant      string      `json:"tenant_id"`
	Kind        string      `json:"scope_kind"`
	Scope       string      `json:"scope_id"`
	Permissions []string    `json:"permissions"`
	Limits      AgentLimits `json:"limits"`
	Revision    int64       `json:"revision"`
	Audience    string      `json:"resource"`
}

type AgentAuthorityError struct{ Missing []string }

func (e *AgentAuthorityError) Error() string {
	return "Additional delegated scopes required: " + strings.Join(e.Missing, ", ")
}

func (e *AgentAuthorityError) Unwrap() error {
	return registry.Problem(403, "needs_authorization", e.Error())
}

func AgentHas(g *AgentGrant, permission string) bool {
	return g != nil && slices.Contains(g.Permissions, permission)
}

func (s *Checker) CheckAgent(ctx context.Context, q registry.Queryer, p Principal, res Resource, exact bool) error {
	g := p.Agent
	if g == nil {
		return nil
	}
	var revision int64
	e := q.QueryRowContext(ctx, `SELECT g.revision FROM agent_grants g JOIN users u ON u.id=g.user_id AND u.state='active'
 LEFT JOIN local_accounts l ON l.user_id=u.id WHERE g.id=? AND g.user_id=? AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6)
 AND COALESCE(l.must_change_password,0)=0 AND g.audience_uri=? LOCK IN SHARE MODE`, g.ID, p.ID, s.Audience).Scan(&revision)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if e != nil || revision != g.Revision {
		return registry.Problem(401, "agent_access_revoked", "Agent access expired or changed; reconnect.")
	}
	if a := AgentCallFrom(ctx); a != nil {
		if e := RequireAgent(g, a.Permission...); e != nil {
			return e
		}
	}
	if g.Kind == "platform" {
		return nil
	} // Existing platformControlAction still forbids Tenant authoring.
	if res.Tenant != g.Tenant {
		return registry.Missing()
	}
	if e = s.Member(ctx, q, Principal{ID: p.ID}, g.Tenant, false); e != nil {
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

func (s *Checker) agentScope(ctx context.Context, q registry.Queryer, p Principal, sc ScopeSet) ScopeSet {
	g := p.Agent
	if g == nil || g.Kind == "tenant" || g.Kind == "platform" {
		return sc
	}
	out := ScopeSet{}
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

func (s *Checker) AgentJobAllowed(ctx context.Context, q registry.Queryer, grant, user string, res Resource, permission string, targets ...string) error {
	if grant == "" {
		return nil
	}
	g, e := s.ReadAgentGrant(ctx, q, grant)
	if e != nil {
		return e
	}
	p := Principal{ID: user, Agent: g}
	ctx = context.WithValue(ctx, AgentContextKey{}, &AgentCall{Grant: g, Permission: []string{permission}})
	if e = s.CheckAgent(ctx, q, p, res, true); e != nil {
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
	return s.Authorize(ctx, q, p, action, res, true)
}

func (s *Checker) ReadAgentGrant(ctx context.Context, q registry.Queryer, key string) (*AgentGrant, error) {
	g, e := ScanAgentGrant(ctx, q, "SELECT "+AgentGrantColumns+" FROM agent_grants g WHERE g.id=?", []any{key})
	if e != nil {
		return nil, e
	}
	return g, nil
}

// AgentGrantColumns selects an agentGrant from agent_grants g.
const AgentGrantColumns = "g.id,g.user_id,g.client_id,g.client_name,COALESCE(g.tenant_id,''),g.scope_kind,COALESCE(g.scope_id,''),g.revision,g.audience_uri,g.permissions_json,g.limits_json"

// ScanAgentGrant reads a grant selected by agentGrantColumns, followed by
// any extra columns scanned into extra. A read failure returns no grant; a
// grant whose stored permissions or limits do not decode is returned with
// the decoding error.
func ScanAgentGrant(ctx context.Context, q registry.Queryer, query string, args []any, extra ...any) (*AgentGrant, error) {
	g := &AgentGrant{}
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

func RequireAgent(g *AgentGrant, permissions ...string) error {
	missing := []string{}
	for _, permission := range permissions {
		if !AgentHas(g, permission) {
			missing = append(missing, permission)
		}
	}
	if len(missing) > 0 {
		return &AgentAuthorityError{Missing: missing}
	}
	return nil
}
