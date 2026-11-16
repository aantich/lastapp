package control

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/aantich/lastapp/internal/authz"
)

// requestScope reads a role/assignment scope from the console route.
func requestScope(r *http.Request) (authz.Scope, Resource, error) {
	return scopeOf(r.PathValue("tenant"), r.PathValue("team"), r.PathValue("space"), r.PathValue("app"))
}

// scopeOf derives the one authorization scope selected by the given IDs: the
// most specific of App, Space, team and Tenant, or the platform without a Tenant.
func scopeOf(t, team, space, app string) (authz.Scope, Resource, error) {
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

// authority owns roles, role assignments and recipient discovery. Console
// handlers and MCP tools both call these methods.
type authority struct {
	db  *sql.DB
	run *runner
}

type (
	roleInput struct {
		ID    string `json:"id"`
		Code  string `json:"code"`
		Name  string `json:"display_name"`
		State string `json:"state"`
	}
	rolePatch struct {
		Name  *string `json:"display_name"`
		State *string `json:"state"`
	}
	assignmentInput struct {
		ID    string `json:"id"`
		User  string `json:"user_id"`
		Group string `json:"group_id"`
		Role  string `json:"role_id"`
	}
)

func activeTenantUser(ctx context.Context, q Queryer, t, user string) error {
	var state string
	e := q.QueryRowContext(ctx, "SELECT u.state FROM users u JOIN tenant_memberships m ON m.user_id=u.id AND m.tenant_id=? AND m.state='active' WHERE u.id=? FOR UPDATE", t, user).Scan(&state)
	if errors.Is(e, sql.ErrNoRows) {
		return bad("An active Tenant member is required.")
	}
	if e != nil {
		return e
	}
	if state != stateActive {
		return bad("An active Tenant member is required.")
	}
	return nil
}

// Roles pages the roles defined at scope: fixed platform-defined roles with
// their control permissions, and custom roles with their data permissions.
func (v *authority) Roles(ctx context.Context, a Actor, scope authz.Scope, res Resource, page PageRequest) (Page[Row], error) {
	if e := v.run.authorize(ctx, a, "authorization.read", res); e != nil {
		return Page[Row]{}, e
	}
	out, e := pageQuery(ctx, v.db, page, collectionFor("roles"), "SELECT id,tenant_id,space_id,app_id,code,display_name,state,revision,scope_kind,domain,(domain='control') AS protected FROM roles WHERE scope_kind=? AND (owner_key='builtin' OR owner_key=?) ORDER BY domain,display_name,id", scope.Kind, scope.OwnerKey())
	if e != nil {
		return out, e
	}
	for _, role := range out.Items {
		role["permissions"] = []string{}
		if def, ok := authz.Builtin(str(role["id"])); ok {
			role["permissions"] = def.Permissions
		} else {
			links, err := rows(ctx, v.db, "SELECT p.id,p.entity_id,p.code,p.description FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=? AND p.domain='data' AND p.state='active'", role["id"])
			if err != nil {
				return Page[Row]{}, err
			}
			role["data_permissions"] = links
		}
	}
	return out, nil
}

// Role reads one role visible at scope; res carries the trusted ancestry.
func (v *authority) Role(ctx context.Context, a Actor, scope authz.Scope, res Resource, record string) (Row, error) {
	if e := v.run.authorize(ctx, a, "authorization.read", res); e != nil {
		return nil, e
	}
	return row(ctx, v.db, "SELECT id,tenant_id,scope_kind,domain,space_id,app_id,code,display_name,state,revision FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?)", record, scope.Kind, scope.OwnerKey())
}

// CreateRole adds a custom business role to a Space or App; Tenant and team
// roles are fixed.
func (v *authority) CreateRole(ctx context.Context, a Actor, scope authz.Scope, res Resource, in roleInput) (Ref, error) {
	if scope.Kind != authz.Space && scope.Kind != authz.App {
		return Ref{}, bad("Tenant and team roles are fixed.")
	}
	if e := id(in.ID); e != nil {
		return Ref{}, e
	}
	if e := named(in.Code, in.Name); e != nil {
		return Ref{}, e
	}
	if in.State == "" {
		in.State = stateActive
	}
	if !enabledState(in.State) {
		return Ref{}, bad("Invalid role state.")
	}
	e := v.run.mutate(ctx, a, "authorization.manage", res, func(tx *sql.Tx) error {
		if e := metadataQuota(ctx, tx, scope.Tenant, "roles"); e != nil {
			return e
		}
		var n int
		column, value := "space_id", scope.Space
		if scope.Kind == authz.App {
			column, value = "app_id", scope.App
		}
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM roles WHERE tenant_id=? AND "+column+"=? AND scope_kind=? AND domain='data'", scope.Tenant, value, scope.Kind).Scan(&n); e != nil {
			return e
		}
		if n >= 200 {
			return conflict("Role limit reached for this scope.")
		}
		space := scope.Space
		if scope.Kind == authz.App {
			if e := tx.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", scope.Tenant, scope.App).Scan(&space); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO roles (id,tenant_id,space_id,app_id,owner_key,scope_kind,domain,code,display_name,state) VALUES (?,?,?,?,?,?,'data',?,?,?)", in.ID, scope.Tenant, space, nullable(scope.App), scope.OwnerKey(), scope.Kind, in.Code, in.Name, in.State); e != nil {
			return e
		}
		return auditChangeAs(ctx, tx, a, scope.Tenant, "role.created", "roles", "role", in.ID, nil)
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "roles", scope.Tenant, in.ID)
}

// UpdateRole renames or enables/disables a custom role under its revision.
func (v *authority) UpdateRole(ctx context.Context, a Actor, scope authz.Scope, res Resource, record string, in rolePatch, revision int64) (Ref, error) {
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	if in.Name != nil && (strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 160) {
		return Ref{}, bad("Invalid name.")
	}
	if in.State != nil && !enabledState(*in.State) {
		return Ref{}, bad("Invalid state.")
	}
	e := v.run.mutate(ctx, a, "authorization.manage", res, func(tx *sql.Tx) error {
		before, e := row(ctx, tx, "SELECT id,domain,owner_key,revision FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?) FOR UPDATE", record, scope.Kind, scope.OwnerKey())
		if e != nil {
			return e
		}
		if str(before["domain"]) == authz.Control {
			return problem(403, "protected_role", "Platform-defined roles cannot be edited.")
		}
		if e = checkRevision(integer(before["revision"]), revision); e != nil {
			return e
		}
		before, e = securitySnapshot(ctx, tx, "roles", scope.Tenant, record)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE roles SET display_name=COALESCE(?,display_name),state=COALESCE(?,state),revision=revision+1 WHERE id=?", in.Name, in.State, record); e != nil {
			return e
		}
		return auditChangeAs(ctx, tx, a, scope.Tenant, "role.updated", "roles", "role", record, before)
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "roles", scope.Tenant, record)
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

// assignmentAction is the authority to read or change assignments at scope.
func assignmentAction(scope authz.Scope, manage bool) string {
	if scope.Kind == authz.Platform {
		return "platform.admin"
	}
	if manage {
		return "authorization.manage"
	}
	return "authorization.read"
}

func (v *authority) Assignments(ctx context.Context, a Actor, scope authz.Scope, res Resource, page PageRequest) (Page[Row], error) {
	if e := v.run.authorize(ctx, a, assignmentAction(scope, false), res); e != nil {
		return Page[Row]{}, e
	}
	where, args := assignmentFilter(scope)
	return pageQuery(ctx, v.db, page, collectionFor("role-assignments"), `SELECT g.id,g.scope_kind,g.domain,g.user_id,g.group_id,g.scope_team_id,g.space_id,g.app_id,g.role_id,g.revision,g.created_at,COALESCE(u.display_name,gr.display_name) AS principal_name,ro.display_name AS access_name,ro.state AS role_state,CASE WHEN g.scope_kind='platform' THEN 'Platform' ELSE COALESCE(team.display_name,sp.display_name,a.display_name,'Entire Tenant') END AS scope_name FROM access_grants g LEFT JOIN users u ON u.id=g.user_id LEFT JOIN groups gr ON gr.id=g.group_id JOIN roles ro ON ro.id=g.role_id LEFT JOIN groups team ON team.id=g.scope_team_id LEFT JOIN spaces sp ON sp.id=g.space_id LEFT JOIN apps a ON a.id=g.app_id WHERE `+where+" ORDER BY g.created_at DESC,g.id", args...)
}

// Assignment reads one assignment at scope after an authorization.read check
// on res, the trusted ancestry of the requested target.
func (v *authority) Assignment(ctx context.Context, a Actor, scope authz.Scope, res Resource, record string) (Row, error) {
	if e := v.run.authorize(ctx, a, "authorization.read", res); e != nil {
		return nil, e
	}
	filter, args := assignmentFilter(scope)
	args = append(args, record)
	return row(ctx, v.db, "SELECT id,tenant_id,scope_kind,domain,user_id,group_id,scope_team_id,space_id,app_id,role_id,revision FROM access_grants g WHERE "+filter+" AND g.id=?", args...)
}

// assignmentAuthority is the role-specific authority to grant or revoke: fixed
// administrative roles need authority above the scope they confer.
func (v *authority) assignmentAuthority(ctx context.Context, tx *sql.Tx, a Actor, scope authz.Scope, res Resource, role Row) error {
	switch str(role["id"]) {
	case authz.SuperRole:
		return v.run.authorizeTx(ctx, tx, a, "platform.admin", Resource{})
	case authz.TenantAdminRole, authz.SpaceManagerRole:
		return v.run.authorizeTx(ctx, tx, a, "authorization.manage", Resource{Tenant: scope.Tenant})
	case authz.AppManagerRole:
		space := res.Space
		if space == "" {
			if e := tx.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", scope.Tenant, scope.App).Scan(&space); e != nil {
				return e
			}
		}
		return v.run.authorizeTx(ctx, tx, a, "authorization.manage", Resource{Tenant: scope.Tenant, Space: space})
	default:
		return v.run.authorizeTx(ctx, tx, a, "authorization.manage", res)
	}
}

// Grant assigns an existing active role at scope to one user or team.
func (v *authority) Grant(ctx context.Context, a Actor, scope authz.Scope, res Resource, in assignmentInput) (Ref, error) {
	for _, value := range []string{in.ID, in.Role} {
		if e := id(value); e != nil {
			return Ref{}, e
		}
	}
	if (in.User == "") == (in.Group == "") {
		return Ref{}, bad("Select one user or team recipient.")
	}
	for _, value := range []string{in.User, in.Group} {
		if value != "" {
			if e := id(value); e != nil {
				return Ref{}, e
			}
		}
	}
	e := v.run.mutate(ctx, a, assignmentAction(scope, true), res, func(tx *sql.Tx) error {
		role, e := row(ctx, tx, "SELECT id,domain,owner_key,state FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?)", in.Role, scope.Kind, scope.OwnerKey())
		if e != nil {
			return e
		}
		if e = v.assignmentAuthority(ctx, tx, a, scope, res, role); e != nil {
			return e
		}
		if str(role["state"]) != stateActive {
			return bad("An active role is required.")
		}
		if str(role["domain"]) == authz.Control && in.Group != "" {
			return bad("Administrative roles require individual users.")
		}

		if scope.Kind != authz.Platform {
			if e = metadataQuota(ctx, tx, scope.Tenant, "access_grants"); e != nil {
				return e
			}
			if in.User != "" {
				if e = activeTenantUser(ctx, tx, scope.Tenant, in.User); e != nil {
					return e
				}
			} else {
				if _, e = row(ctx, tx, "SELECT id FROM groups WHERE tenant_id=? AND id=? AND state='active'", scope.Tenant, in.Group); e != nil {
					return e
				}
			}
		} else {
			if _, e = row(ctx, tx, "SELECT id FROM users WHERE id=? AND state='active' FOR UPDATE", in.User); e != nil {
				return e
			}
		}
		if str(role["domain"]) == authz.Control {
			if scope.Kind != authz.Platform {
				platform, e := isPlatform(ctx, tx, in.User)
				if e != nil {
					return e
				}
				if platform {
					return bad("Use a Tenant account for Tenant, team, Space and App roles.")
				}
			} else {
				if e := platformCandidate(ctx, tx, in.User); e != nil {
					return e
				}
			}
		}
		if scope.Kind == authz.Team {
			if _, e = row(ctx, tx, "SELECT gm.user_id FROM group_memberships gm JOIN groups team ON team.id=gm.group_id AND team.state='active' WHERE gm.tenant_id=? AND gm.group_id=? AND gm.user_id=? AND gm.state='active'", scope.Tenant, scope.Team, in.User); e != nil {
				return bad("An active member of this team is required.")
			}
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,tenant_id,scope_kind,domain,user_id,group_id,scope_team_id,space_id,app_id,role_id,role_owner_key) VALUES (?,?,?,?,?,?,?,?,?,?,?)", in.ID, nullable(scope.Tenant), scope.Kind, role["domain"], nullable(in.User), nullable(in.Group), nullable(scope.Team), nullable(scope.Space), nullable(scope.App), in.Role, role["owner_key"]); e != nil {
			return e
		}
		return auditChangeAs(ctx, tx, a, scope.Tenant, "grant.created", "access_grants", "access_grant", in.ID, nil)
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "access_grants", scope.Tenant, in.ID)
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

// Revoke deletes an assignment under its revision, keeping at least one
// platform and one Tenant administrator.
func (v *authority) Revoke(ctx context.Context, a Actor, scope authz.Scope, res Resource, record string, revision int64) (done, error) {
	if revision < 1 {
		return done{}, revisionRequired()
	}
	e := v.run.mutate(ctx, a, assignmentAction(scope, true), res, func(tx *sql.Tx) error {
		where, args := assignmentFilter(scope)
		args = append(args, record)
		grant, e := row(ctx, tx, "SELECT g.id,g.role_id,g.revision FROM access_grants g WHERE "+where+" AND g.id=? FOR UPDATE", args...)
		if e != nil {
			return e
		}
		role, e := row(ctx, tx, "SELECT id FROM roles WHERE id=?", grant["role_id"])
		if e != nil {
			return e
		}
		if e = v.assignmentAuthority(ctx, tx, a, scope, res, role); e != nil {
			return e
		}
		if e = checkRevision(integer(grant["revision"]), revision); e != nil {
			return e
		}
		before, e := securitySnapshot(ctx, tx, "access_grants", scope.Tenant, record)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM access_grants WHERE id=?", record); e != nil {
			return e
		}
		if str(grant["role_id"]) == SuperRole {
			if e = ensurePlatformAdmin(ctx, tx); e != nil {
				return e
			}
		}
		if str(grant["role_id"]) == TenantAdminRole {
			if e = ensureTenantAdmin(ctx, tx, scope.Tenant); e != nil {
				return e
			}
		}
		return auditAs(ctx, tx, a, scope.Tenant, "grant.deleted", "access_grant", record, Row{"before": before, "after": nil})
	})
	if e != nil {
		return done{}, e
	}
	return done{OK: true}, nil
}

// Recipients lets scope managers search eligible users or active teams without
// reading the global user directory or gaining membership authority.
func (v *authority) Recipients(ctx context.Context, a Actor, scope authz.Scope, res Resource, teams bool, page PageRequest) (Page[Row], error) {
	if scope.Kind == authz.Platform {
		return Page[Row]{}, bad("A Tenant scope is required.")
	}
	if e := v.run.authorize(ctx, a, "authorization.manage", res); e != nil {
		return Page[Row]{}, e
	}
	if teams {
		return pageQuery(ctx, v.db, page, collectionFor("team-recipients"), "SELECT id,display_name FROM groups WHERE tenant_id=? AND state='active' ORDER BY display_name,id", scope.Tenant)
	}
	return pageQuery(ctx, v.db, page, collectionFor("recipients"), "SELECT u.id,u.id AS user_id,u.display_name,u.email FROM users u JOIN tenant_memberships m ON m.user_id=u.id WHERE m.tenant_id=? AND m.state='active' AND u.state='active' AND NOT EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) ORDER BY u.display_name,u.id", scope.Tenant, SuperRole)
}

// assignInitialManager gives a new Space or App its manager inside the
// creating transaction.
func assignInitialManager(ctx context.Context, tx *sql.Tx, a Actor, tenant, space, app, user string) error {
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
	return auditChangeAs(ctx, tx, a, tenant, "grant.created", "access_grants", "access_grant", record, nil)
}

func (s *Server) roles(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Roles(r.Context(), httpActor(r, p), scope, res, pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in roleInput
	if e = decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.CreateRole(r.Context(), httpActor(r, p), scope, res, in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in rolePatch
	if e = decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.UpdateRole(r.Context(), httpActor(r, p), scope, res, r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) grants(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Assignments(r.Context(), httpActor(r, p), scope, res, pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in assignmentInput
	if e = decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.Grant(r.Context(), httpActor(r, p), scope, res, in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) deleteGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Revoke(r.Context(), httpActor(r, p), scope, res, r.PathValue("id"), ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) recipientOptions(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Recipients(r.Context(), httpActor(r, p), scope, res, r.URL.Query().Get("type") == "team", pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}
