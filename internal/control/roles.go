package control

import (
	"context"
	"database/sql"
	"github.com/aantich/lastapp/internal/authz"
	"net/http"
	"strings"
)

func requestScope(r *http.Request) (authz.Scope, Resource, error) {
	t, team, space, app := r.PathValue("tenant"), r.PathValue("team"), r.PathValue("space"), r.PathValue("app")
	k := authz.Tenant
	if t == "" {
		k = authz.Platform
	}
	if team != "" {
		k = authz.Team
	}
	if space != "" {
		k = authz.Space
	}
	if app != "" {
		k = authz.App
	}
	scope := authz.Scope{Kind: k, Tenant: t, Team: team, Space: space, App: app}
	if k == authz.App {
		scope.Space = ""
	}
	if e := scope.Validate(); e != nil {
		return scope, Resource{}, bad(e.Error())
	}
	for _, v := range []string{t, team, space, app} {
		if v != "" {
			if e := id(v); e != nil {
				return scope, Resource{}, e
			}
		}
	}
	return scope, Resource{Tenant: t, Team: team, Space: space, App: app}, nil
}
func activeTenantUser(ctx context.Context, q Queryer, t, user string) error {
	var state string
	e := q.QueryRowContext(ctx, "SELECT u.state FROM users u JOIN tenant_memberships m ON m.user_id=u.id AND m.tenant_id=? AND m.state='active' WHERE u.id=? FOR UPDATE", t, user).Scan(&state)
	if e == sql.ErrNoRows {
		return bad("An active Tenant member is required.")
	}
	if e != nil {
		return e
	}
	if state != "active" {
		return bad("An active Tenant member is required.")
	}
	return nil
}
func (s *Server) roles(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	if e = s.authorize(r.Context(), s.DB, p, "authorization.read", res, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT id,tenant_id,space_id,app_id,code,display_name,state,revision,scope_kind,domain,(domain='control') AS protected FROM roles WHERE scope_kind=? AND (owner_key='builtin' OR owner_key=?) ORDER BY domain,display_name,id", scope.Kind, scope.OwnerKey())
	if e != nil {
		return e
	}
	for _, role := range v {
		role["permissions"] = []string{}
		if def, ok := authz.Builtin(str(role["id"])); ok {
			role["permissions"] = def.Permissions
		} else {
			links, err := rows(r.Context(), s.DB, "SELECT p.id,p.entity_id,p.code,p.description FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=? AND p.domain='data' AND p.state='active'", role["id"])
			if err != nil {
				return err
			}
			role["data_permissions"] = links
		}
	}
	sendPage(w, r, v)
	return nil
}
func (s *Server) createRole(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	if scope.Kind != authz.Space && scope.Kind != authz.App {
		return bad("Tenant and team roles are fixed.")
	}
	var in struct {
		ID    string `json:"id"`
		Code  string `json:"code"`
		Name  string `json:"display_name"`
		State string `json:"state"`
	}
	if e = decode(w, r, &in); e != nil {
		return e
	}
	if e = id(in.ID); e != nil {
		return e
	}
	if e = named(in.Code, in.Name); e != nil {
		return e
	}
	if in.State == "" {
		in.State = "active"
	}
	if in.State != "active" && in.State != "disabled" {
		return bad("Invalid role state.")
	}
	e = s.mutate(r.Context(), p, "authorization.manage", res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if e := metadataQuota(r.Context(), tx, scope.Tenant, "roles"); e != nil {
			return e
		}
		var n int
		column, value := "space_id", scope.Space
		if scope.Kind == authz.App {
			column, value = "app_id", scope.App
		}
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM roles WHERE tenant_id=? AND "+column+"=? AND scope_kind=? AND domain='data'", scope.Tenant, value, scope.Kind).Scan(&n); e != nil {
			return e
		}
		if n >= 200 {
			return conflict("Role limit reached for this scope.")
		}
		space := scope.Space
		if scope.Kind == authz.App {
			if e := tx.QueryRowContext(r.Context(), "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", scope.Tenant, scope.App).Scan(&space); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(r.Context(), "INSERT INTO roles (id,tenant_id,space_id,app_id,owner_key,scope_kind,domain,code,display_name,state) VALUES (?,?,?,?,?,?,'data',?,?,?)", in.ID, scope.Tenant, space, nullable(scope.App), scope.OwnerKey(), scope.Kind, in.Code, in.Name, in.State); e != nil {
			return e
		}
		return auditChange(r.Context(), tx, scope.Tenant, p.ID, "role.created", "roles", "role", in.ID, r.Header.Get("X-Request-ID"), nil)
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "roles", scope.Tenant, in.ID, 201)
}
func (s *Server) updateRole(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in struct {
		Name  *string `json:"display_name"`
		State *string `json:"state"`
	}
	if e = decode(w, r, &in); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	if in.Name != nil && (strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 160) {
		return bad("Invalid name.")
	}
	if in.State != nil && *in.State != "active" && *in.State != "disabled" {
		return bad("Invalid state.")
	}
	record := r.PathValue("id")
	e = s.mutate(r.Context(), p, "authorization.manage", res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		before, e := row(r.Context(), tx, "SELECT id,domain,owner_key,revision FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?) FOR UPDATE", record, scope.Kind, scope.OwnerKey())
		if e != nil {
			return e
		}
		if str(before["domain"]) == authz.Control {
			return problem(403, "protected_role", "Platform-defined roles cannot be edited.")
		}
		if e = checkRevision(integer(before["revision"]), rev); e != nil {
			return e
		}
		before, e = securitySnapshot(r.Context(), tx, "roles", scope.Tenant, record)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE roles SET display_name=COALESCE(?,display_name),state=COALESCE(?,state),revision=revision+1 WHERE id=?", in.Name, in.State, record); e != nil {
			return e
		}
		return auditChange(r.Context(), tx, scope.Tenant, p.ID, "role.updated", "roles", "role", record, r.Header.Get("X-Request-ID"), before)
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "roles", scope.Tenant, record, 200)
}
func assignmentFilter(scope authz.Scope) (string, []any) {
	where := "g.scope_kind=? AND g.tenant_id <=> ?"
	args := []any{scope.Kind, nullable(scope.Tenant)}
	switch scope.Kind {
	case authz.Team:
		where += " AND g.scope_team_id=?"
		args = append(args, scope.Team)
	case authz.Space:
		where += " AND g.space_id=?"
		args = append(args, scope.Space)
	case authz.App:
		where += " AND g.app_id=?"
		args = append(args, scope.App)
	}
	return where, args
}
func (s *Server) grants(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	action := "authorization.read"
	if scope.Kind == authz.Platform {
		action = "platform.admin"
	}
	if e = s.authorize(r.Context(), s.DB, p, action, res, false); e != nil {
		return e
	}
	where, args := assignmentFilter(scope)
	v, e := pageRows(r.Context(), s.DB, r, `SELECT g.id,g.scope_kind,g.domain,g.user_id,g.group_id,g.scope_team_id,g.space_id,g.app_id,g.role_id,g.revision,g.created_at,COALESCE(u.display_name,gr.display_name) AS principal_name,ro.display_name AS access_name,ro.state AS role_state,CASE WHEN g.scope_kind='platform' THEN 'Platform' ELSE COALESCE(team.display_name,sp.display_name,a.display_name,'Entire Tenant') END AS scope_name FROM access_grants g LEFT JOIN users u ON u.id=g.user_id LEFT JOIN groups gr ON gr.id=g.group_id JOIN roles ro ON ro.id=g.role_id LEFT JOIN groups team ON team.id=g.scope_team_id LEFT JOIN spaces sp ON sp.id=g.space_id LEFT JOIN apps a ON a.id=g.app_id WHERE `+where+" ORDER BY g.created_at DESC,g.id", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func (s *Server) assignmentAuthority(ctx context.Context, q Queryer, p Principal, scope authz.Scope, res Resource, role Row) error {
	switch str(role["id"]) {
	case authz.SuperRole:
		return s.authorize(ctx, q, p, "platform.admin", Resource{}, true)
	case authz.TenantAdminRole, authz.SpaceManagerRole:
		return s.authorize(ctx, q, p, "authorization.manage", Resource{Tenant: scope.Tenant}, true)
	case authz.AppManagerRole:
		space := res.Space
		if space == "" {
			if e := q.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", scope.Tenant, scope.App).Scan(&space); e != nil {
				return e
			}
		}
		return s.authorize(ctx, q, p, "authorization.manage", Resource{Tenant: scope.Tenant, Space: space}, true)
	default:
		return s.authorize(ctx, q, p, "authorization.manage", res, true)
	}
}
func (s *Server) createGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in struct {
		ID    string `json:"id"`
		User  string `json:"user_id"`
		Group string `json:"group_id"`
		Role  string `json:"role_id"`
	}
	if e = decode(w, r, &in); e != nil {
		return e
	}
	for _, v := range []string{in.ID, in.Role} {
		if e = id(v); e != nil {
			return e
		}
	}
	if (in.User == "") == (in.Group == "") {
		return bad("Select one user or team recipient.")
	}
	for _, v := range []string{in.User, in.Group} {
		if v != "" {
			if e = id(v); e != nil {
				return e
			}
		}
	}
	action := "authorization.manage"
	if scope.Kind == authz.Platform {
		action = "platform.admin"
	}
	e = s.mutate(r.Context(), p, action, res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		role, e := row(r.Context(), tx, "SELECT id,domain,owner_key,state FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?)", in.Role, scope.Kind, scope.OwnerKey())
		if e != nil {
			return e
		}
		if e = s.assignmentAuthority(r.Context(), tx, p, scope, res, role); e != nil {
			return e
		}
		if str(role["state"]) != "active" {
			return bad("An active role is required.")
		}
		if str(role["domain"]) == authz.Control && in.Group != "" {
			return bad("Administrative roles require individual users.")
		}

		if scope.Kind != authz.Platform {
			if e = metadataQuota(r.Context(), tx, scope.Tenant, "access_grants"); e != nil {
				return e
			}
			if in.User != "" {
				if e = activeTenantUser(r.Context(), tx, scope.Tenant, in.User); e != nil {
					return e
				}
			} else {
				if _, e = row(r.Context(), tx, "SELECT id FROM groups WHERE tenant_id=? AND id=? AND state='active'", scope.Tenant, in.Group); e != nil {
					return e
				}
			}
		} else {
			if _, e = row(r.Context(), tx, "SELECT id FROM users WHERE id=? AND state='active' FOR UPDATE", in.User); e != nil {
				return e
			}
		}
		if str(role["domain"]) == authz.Control {
			if scope.Kind != authz.Platform {
				platform, e := isPlatform(r.Context(), tx, in.User)
				if e != nil {
					return e
				}
				if platform {
					return bad("Use a Tenant account for Tenant, team, Space and App roles.")
				}
			} else {
				if e := platformCandidate(r.Context(), tx, in.User); e != nil {
					return e
				}
			}
		}
		if scope.Kind == authz.Team {
			if _, e = row(r.Context(), tx, "SELECT gm.user_id FROM group_memberships gm JOIN groups team ON team.id=gm.group_id AND team.state='active' WHERE gm.tenant_id=? AND gm.group_id=? AND gm.user_id=? AND gm.state='active'", scope.Tenant, scope.Team, in.User); e != nil {
				return bad("An active member of this team is required.")
			}
		}
		if _, e = tx.ExecContext(r.Context(), "INSERT INTO access_grants (id,tenant_id,scope_kind,domain,user_id,group_id,scope_team_id,space_id,app_id,role_id,role_owner_key) VALUES (?,?,?,?,?,?,?,?,?,?,?)", in.ID, nullable(scope.Tenant), scope.Kind, role["domain"], nullable(in.User), nullable(in.Group), nullable(scope.Team), nullable(scope.Space), nullable(scope.App), in.Role, role["owner_key"]); e != nil {
			return e
		}
		return auditChange(r.Context(), tx, scope.Tenant, p.ID, "grant.created", "access_grants", "access_grant", in.ID, r.Header.Get("X-Request-ID"), nil)
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "access_grants", scope.Tenant, in.ID, 201)
}

// The recipient user row must already be locked, including during bootstrap.
func platformCandidate(ctx context.Context, q Queryer, user string) error {
	var n int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM access_grants WHERE user_id=? AND tenant_id IS NOT NULL AND domain='control'", user).Scan(&n); e != nil {
		return e
	}
	if n > 0 {
		return conflict("Remove this user's Tenant and team roles before granting platform administration.")
	}
	return nil
}
func (s *Server) deleteGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	record := r.PathValue("id")
	action := "authorization.manage"
	if scope.Kind == authz.Platform {
		action = "platform.admin"
	}
	e = s.mutate(r.Context(), p, action, res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		where, args := assignmentFilter(scope)
		args = append(args, record)
		grant, e := row(r.Context(), tx, "SELECT g.id,g.role_id,g.revision FROM access_grants g WHERE "+where+" AND g.id=? FOR UPDATE", args...)
		if e != nil {
			return e
		}
		role, e := row(r.Context(), tx, "SELECT id FROM roles WHERE id=?", grant["role_id"])
		if e != nil {
			return e
		}
		if e = s.assignmentAuthority(r.Context(), tx, p, scope, res, role); e != nil {
			return e
		}
		if e = checkRevision(integer(grant["revision"]), rev); e != nil {
			return e
		}
		before, e := securitySnapshot(r.Context(), tx, "access_grants", scope.Tenant, record)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(r.Context(), "DELETE FROM access_grants WHERE id=?", record); e != nil {
			return e
		}
		if str(grant["role_id"]) == SuperRole {
			if e = ensurePlatformAdmin(r.Context(), tx); e != nil {
				return e
			}
		}
		if str(grant["role_id"]) == TenantAdminRole {
			if e = ensureTenantAdmin(r.Context(), tx, scope.Tenant); e != nil {
				return e
			}
		}
		return audit(r.Context(), tx, scope.Tenant, p.ID, "grant.deleted", "access_grant", record, r.Header.Get("X-Request-ID"), Row{"before": before, "after": nil})
	})
	if e == nil {
		send(w, 200, Row{"ok": true})
	}
	return e
}

func assignInitialManager(ctx context.Context, tx *sql.Tx, p Principal, tenant, space, app, user, request string) error {
	if e := metadataQuota(ctx, tx, tenant, "access_grants"); e != nil {
		return e
	}
	if e := id(user); e != nil {
		return bad("Select an active Tenant member as initial manager.")
	}
	if e := activeTenantUser(ctx, tx, tenant, user); e != nil {
		return e
	}
	platform, e := isPlatform(ctx, tx, user)
	if e != nil {
		return e
	}
	if platform {
		return bad("Choose a Tenant account as initial manager.")
	}
	kind, role := authz.Space, authz.SpaceManagerRole
	var scopeSpace any = space
	var scopeApp any
	if app != "" {
		kind, role = authz.App, authz.AppManagerRole
		scopeSpace = nil
		scopeApp = app
	}
	record := newID()
	if _, e := tx.ExecContext(ctx, "INSERT INTO access_grants(id,tenant_id,scope_kind,domain,user_id,space_id,app_id,role_id,role_owner_key) VALUES (?,?,?,'control',?,?,?,?,'builtin')", record, tenant, kind, user, scopeSpace, scopeApp, role); e != nil {
		return e
	}
	return auditChange(ctx, tx, tenant, p.ID, "grant.created", "access_grants", "access_grant", record, request, nil)
}
func (s *Server) updateGroupMember(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if in.State != "active" && in.State != "disabled" {
		return bad("Invalid membership state.")
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	t, g, u := r.PathValue("tenant"), r.PathValue("id"), r.PathValue("user")
	e = s.mutate(r.Context(), p, "team.manage", Resource{Tenant: t, Team: g}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		team, e := lockRecord(r.Context(), tx, "groups", t, g)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(team["revision"]), rev); e != nil {
			return e
		}
		before, e := row(r.Context(), tx, "SELECT user_id,state FROM group_memberships WHERE tenant_id=? AND group_id=? AND user_id=?", t, g, u)
		if e != nil {
			return e
		}
		if in.State == "active" {
			if e = activeTenantUser(r.Context(), tx, t, u); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE group_memberships SET state=? WHERE tenant_id=? AND group_id=? AND user_id=?", in.State, t, g, u); e != nil {
			return e
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE groups SET revision=revision+1 WHERE id=?", g); e != nil {
			return e
		}
		return audit(r.Context(), tx, t, p.ID, "team.membership.updated", "group", g, r.Header.Get("X-Request-ID"), Row{"before": before, "after": Row{"user_id": u, "state": in.State}})
	})
	if e == nil {
		send(w, 200, Row{"ok": true})
	}
	return e
}
