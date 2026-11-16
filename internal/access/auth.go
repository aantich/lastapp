// Package access holds the live authorization checks: principals, resources,
// scopes, registry ancestry, delegated agent grants and the production and
// workspace guards. Authority is always rechecked against the registry.
package access

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/registry"
)

const SuperRole = authz.SuperRole

const TenantAdminRole = authz.TenantAdminRole

type Principal struct {
	ID, Name, Email        string
	Platform               bool
	AuthMethod             string
	PasswordChangeRequired bool
	Agent                  *AgentGrant
}

// Checker holds the live authorization checks shared by every operation:
// identity, membership and registry ancestry, delegated grants and the
// production and workspace guards.
type Checker struct {
	// Audience is the MCP resource URI that every delegated grant must name.
	Audience string
}

type Resource struct{ Tenant, Team, Space, App string }

type ScopeSet struct {
	All                 bool
	Spaces, Apps, Teams []string
}

func IsPlatform(ctx context.Context, q registry.Queryer, user string) (bool, error) {
	var n int
	e := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM access_grants g JOIN users u ON u.id=g.user_id AND u.state='active' JOIN roles ro ON ro.id=g.role_id AND ro.state='active' WHERE g.scope_kind='platform' AND g.domain='control' AND g.user_id=? AND g.role_id=?`, user, SuperRole).Scan(&n)
	return n > 0, e
}

func (s *Checker) Member(ctx context.Context, q registry.Queryer, p Principal, tenant string, mutating bool) error {
	var state string
	e := q.QueryRowContext(ctx, "SELECT t.state FROM tenants t JOIN tenant_memberships m ON m.tenant_id=t.id JOIN users u ON u.id=m.user_id WHERE t.id=? AND m.user_id=? AND m.state='active' AND u.state='active'", tenant, p.ID).Scan(&state)
	if errors.Is(e, sql.ErrNoRows) {
		return registry.Missing()
	}
	if e != nil {
		return e
	}
	if mutating && state != registry.StateActive {
		return registry.Problem(403, "tenant_inactive", "This Tenant is suspended or archived.")
	}
	return nil
}

func (s *Checker) Scopes(ctx context.Context, q registry.Queryer, p Principal, tenant, action string) (ScopeSet, error) {
	if p.Agent != nil {
		if e := s.CheckAgent(ctx, q, p, Resource{Tenant: tenant}, false); e != nil {
			return ScopeSet{}, e
		}
	}
	if !controlAction(action) {
		return ScopeSet{}, registry.Problem(403, "unknown_action", "Unsupported control-plane action.")
	}
	global, e := IsPlatform(ctx, q, p.ID)
	if e != nil {
		return ScopeSet{}, e
	}
	if global {
		return s.agentScope(ctx, q, p, ScopeSet{All: strings.HasSuffix(action, ".read")}), nil
	}
	if e = s.Member(ctx, q, p, tenant, false); e != nil {
		return ScopeSet{}, e
	}
	p.Platform = false // global authority was rechecked above, inside the mutation transaction
	all, e := s.EffectiveScopes(ctx, q, p, tenant)
	return s.agentScope(ctx, q, p, all[action]), e
}

// effectiveAssignments is shared by management checks and the caller context.
// Data roles are relationships only in this foundation, never management grants.
const EffectiveAssignmentSQL = `SELECT g.app_id,ro.code,ro.display_name,ro.domain,g.group_id,g.id,g.role_id,g.scope_kind,g.scope_team_id,g.space_id
 FROM access_grants g JOIN roles ro ON ro.id=g.role_id AND ro.state='active'
 JOIN users u ON u.id=? AND u.state='active'
 JOIN tenant_memberships tm ON tm.tenant_id=g.tenant_id AND tm.user_id=u.id AND tm.state='active'
 JOIN tenants t ON t.id=g.tenant_id
 LEFT JOIN groups recipient ON recipient.id=g.group_id AND recipient.tenant_id=g.tenant_id AND recipient.state='active'
 LEFT JOIN groups scoped ON scoped.id=g.scope_team_id AND scoped.tenant_id=g.tenant_id AND scoped.state='active'
 LEFT JOIN group_memberships scoped_member ON scoped_member.tenant_id=g.tenant_id AND scoped_member.group_id=scoped.id AND scoped_member.user_id=u.id AND scoped_member.state='active'
 LEFT JOIN apps a ON a.id=g.app_id AND a.tenant_id=g.tenant_id
 LEFT JOIN spaces sp ON sp.id=COALESCE(g.space_id,a.space_id) AND sp.tenant_id=g.tenant_id
 WHERE g.tenant_id=? AND (g.user_id=u.id OR (recipient.id IS NOT NULL AND EXISTS(SELECT 1 FROM group_memberships gm WHERE gm.tenant_id=g.tenant_id AND gm.group_id=recipient.id AND gm.user_id=u.id AND gm.state='active')))
 AND (g.scope_kind<>'team' OR scoped_member.user_id IS NOT NULL)
 AND (ro.domain='control' OR (t.state='active' AND sp.state='active' AND (g.app_id IS NULL OR a.state='active')))
 ORDER BY g.scope_kind,g.id`

// EffectiveAssignment is an assignment that currently applies to a user,
// directly or through a team, with its role's code, name and domain.
type EffectiveAssignment struct {
	AppID       *string `json:"app_id"`
	Code        string  `json:"code"`
	Name        string  `json:"display_name"`
	Domain      string  `json:"domain"`
	GroupID     *string `json:"group_id"`
	ID          string  `json:"id"`
	RoleID      string  `json:"role_id"`
	ScopeKind   string  `json:"scope_kind"`
	ScopeTeamID *string `json:"scope_team_id"`
	SpaceID     *string `json:"space_id"`
}

func (v *EffectiveAssignment) ScanFields() []any {
	return []any{&v.AppID, &v.Code, &v.Name, &v.Domain, &v.GroupID, &v.ID, &v.RoleID, &v.ScopeKind, &v.ScopeTeamID, &v.SpaceID}
}

// Deref is the value of an optional column, or its zero value for NULL.
func Deref[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}
	return *v
}

func effectiveAssignments(ctx context.Context, q registry.Queryer, user, tenant string) ([]EffectiveAssignment, error) {
	return registry.ScanAll[EffectiveAssignment](ctx, q, EffectiveAssignmentSQL, user, tenant)
}

func (s *Checker) EffectiveScopes(ctx context.Context, q registry.Queryer, p Principal, tenant string) (map[string]ScopeSet, error) {
	global, e := IsPlatform(ctx, q, p.ID)
	if e != nil {
		return nil, e
	}
	out := map[string]ScopeSet{}
	for _, code := range authz.Actions() {
		if code != "platform.admin" {
			out[code] = ScopeSet{All: global && platformControlAction(code, Resource{Tenant: tenant}, strings.HasSuffix(code, ".manage")), Spaces: []string{}, Apps: []string{}, Teams: []string{}}
		}
	}
	if global {
		return out, nil
	}
	assignments, e := effectiveAssignments(ctx, q, p.ID, tenant)
	if e != nil {
		return nil, e
	}
	// The control catalog is closed and platform-owned; custom data roles cannot
	// contribute actions, even if their display names imitate protected roles.
	for _, g := range assignments {
		if g.Domain != authz.Control {
			continue
		}
		role, ok := authz.Builtin(g.RoleID)
		if !ok {
			continue
		}
		for _, code := range role.Permissions {
			sc, ok := out[code]
			if !ok {
				continue
			}
			switch authz.Kind(g.ScopeKind) {
			case authz.Tenant:
				sc.All = true
			case authz.Team:
				sc.Teams = appendUnique(sc.Teams, Deref(g.ScopeTeamID))
			case authz.Space:
				sc.Spaces = appendUnique(sc.Spaces, Deref(g.SpaceID))
			case authz.App:
				sc.Apps = appendUnique(sc.Apps, Deref(g.AppID))
			}
			out[code] = sc
		}
	}
	return out, nil
}

func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

func (s *Checker) Authorize(ctx context.Context, q registry.Queryer, p Principal, action string, res Resource, mutating bool) error {
	if !controlAction(action) {
		return registry.Problem(403, "unknown_action", "Unsupported control-plane action.")
	}
	if res.Team != "" {
		if res.Space != "" || res.App != "" {
			return registry.Bad("Team scope cannot be combined with Space or App scope.")
		}
		var found string
		if e := registry.Lookup(ctx, q, "SELECT id FROM groups WHERE tenant_id=? AND id=?", []any{res.Tenant, res.Team}, &found); e != nil {
			return e
		}
	}
	if res.App != "" {
		var parent string
		if e := registry.Lookup(ctx, q, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", []any{res.Tenant, res.App}, &parent); e != nil {
			return e
		}
		if res.Space != "" && res.Space != parent {
			return registry.Missing()
		}
		res.Space = parent
	}
	if res.Space != "" {
		var found string
		if e := registry.Lookup(ctx, q, "SELECT id FROM spaces WHERE tenant_id=? AND id=?", []any{res.Tenant, res.Space}, &found); e != nil {
			return e
		}
	}
	if e := s.WorkspaceGuard(ctx, q, p, res.Space); e != nil {
		return e
	}
	if p.Agent != nil {
		if e := s.CheckAgent(ctx, q, p, res, true); e != nil {
			return e
		}
	}
	global, e := IsPlatform(ctx, q, p.ID)
	if e != nil {
		return e
	}
	if global {
		if platformControlAction(action, res, mutating) {
			return nil
		}
		return registry.Problem(403, "tenant_authority_required", "Tenant administrators and delegated managers manage Tenant resources.")
	}
	if action == "platform.admin" || res.Tenant == "" {
		return registry.Problem(403, "forbidden", "Platform administrator permission is required.")
	}
	if e = s.Member(ctx, q, p, res.Tenant, mutating); e != nil {
		return e
	}
	all, e := s.EffectiveScopes(ctx, q, p, res.Tenant)
	if e != nil {
		return e
	}
	allowed := all[action]
	if allowed.All {
		return nil
	}
	for _, v := range allowed.Teams {
		if res.Team != "" && res.Team == v {
			return nil
		}
	}
	for _, v := range allowed.Spaces {
		if res.Space != "" && res.Space == v {
			return nil
		}
	}
	for _, v := range allowed.Apps {
		if res.App != "" && res.App == v {
			return nil
		}
	}
	return registry.Problem(403, "forbidden", "This action is not permitted in this scope.")
}

// Platform operators administer the service and Tenant account recovery. They
// can inspect operational metadata, but never author Tenant Spaces/Apps or policy.
func platformControlAction(action string, res Resource, mutating bool) bool {
	if action == "platform.admin" {
		return true
	}
	if !mutating && strings.HasSuffix(action, ".read") {
		return true
	}
	return res.Tenant != "" && res.Team == "" && res.Space == "" && res.App == "" && (action == "membership.manage" || action == "authorization.manage")
}

func ScopeSQL(sc ScopeSet, spaceColumn, appColumn string) (string, []any) {
	if sc.All {
		return "1=1", nil
	}
	clauses := []string{"1=0"}
	args := []any{}
	if spaceColumn != "" {
		for _, v := range sc.Spaces {
			clauses = append(clauses, spaceColumn+"=?")
			args = append(args, v)
		}
	}
	if appColumn != "" {
		for _, v := range sc.Apps {
			clauses = append(clauses, appColumn+"=?")
			args = append(args, v)
		}
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func controlAction(action string) bool { return authz.IsControlAction(action) }
