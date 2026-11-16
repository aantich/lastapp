package roles

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// ScopeOf derives the one authorization scope selected by the given IDs: the
// most specific of App, Space, team and Tenant, or the platform without a Tenant.
func ScopeOf(t, team, space, app string) (authz.Scope, access.Resource, error) {
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
		return scope, access.Resource{}, registry.Bad(e.Error())
	}
	for _, v := range []string{t, team, space, app} {
		if v != "" {
			if e := registry.ID(v); e != nil {
				return scope, access.Resource{}, e
			}
		}
	}
	return scope, access.Resource{Tenant: t, Team: team, Space: space, App: app}, nil
}

// Authority owns roles, role assignments and recipient discovery. Console
// handlers and MCP tools both call these methods.
type Authority struct {
	DB  *sql.DB
	Run *op.Runner
}

type (
	RoleInput struct {
		ID    string `json:"id"`
		Code  string `json:"code"`
		Name  string `json:"display_name"`
		State string `json:"state"`
	}
	RolePatch struct {
		Name  *string `json:"display_name"`
		State *string `json:"state"`
	}
	AssignmentInput struct {
		ID    string `json:"id"`
		User  string `json:"user_id"`
		Group string `json:"group_id"`
		Role  string `json:"role_id"`
	}
)

// Role is a role visible at a scope. The collection (roleItem) adds whether
// it is platform-defined and its permissions: control permission codes for a
// fixed role, active data permissions for a custom role. An audit snapshot
// adds its permission codes.
type Role struct {
	AppID           *string           `json:"app_id"`
	Code            string            `json:"code"`
	DataPermissions *[]RolePermission `json:"data_permissions,omitempty"`
	Name            string            `json:"display_name"`
	Domain          string            `json:"domain"`
	ID              string            `json:"id"`
	Permissions     *[]string         `json:"permissions,omitempty"`
	Protected       *registry.Bit     `json:"protected,omitempty"`
	Revision        int64             `json:"revision"`
	ScopeKind       string            `json:"scope_kind"`
	SpaceID         *string           `json:"space_id"`
	State           string            `json:"state"`
	TenantID        *string           `json:"tenant_id"`
}

const roleColumns = "app_id,code,display_name,domain,id,revision,scope_kind,space_id,state,tenant_id"

func (v *Role) ScanFields() []any {
	return []any{&v.AppID, &v.Code, &v.Name, &v.Domain, &v.ID, &v.Revision, &v.ScopeKind, &v.SpaceID, &v.State, &v.TenantID}
}

type RoleItem struct{ Role }

func (v *RoleItem) ScanFields() []any {
	v.Protected = new(registry.Bit)
	return append(v.Role.ScanFields(), v.Protected)
}

// RolePermission is a data permission linked to a custom role.
type RolePermission struct {
	Code        string  `json:"code"`
	Description string  `json:"description"`
	EntityID    *string `json:"entity_id"`
	ID          string  `json:"id"`
}

func (v *RolePermission) ScanFields() []any {
	return []any{&v.Code, &v.Description, &v.EntityID, &v.ID}
}

// Assignment is a role assignment at its scope; also its audit snapshot.
type Assignment struct {
	AppID       *string `json:"app_id"`
	Domain      string  `json:"domain"`
	GroupID     *string `json:"group_id"`
	ID          string  `json:"id"`
	Revision    int64   `json:"revision"`
	RoleID      string  `json:"role_id"`
	ScopeKind   string  `json:"scope_kind"`
	ScopeTeamID *string `json:"scope_team_id"`
	SpaceID     *string `json:"space_id"`
	TenantID    *string `json:"tenant_id"`
	UserID      *string `json:"user_id"`
}

const assignmentColumns = "app_id,domain,group_id,id,revision,role_id,scope_kind,scope_team_id,space_id,tenant_id,user_id"

func (v *Assignment) ScanFields() []any {
	return []any{&v.AppID, &v.Domain, &v.GroupID, &v.ID, &v.Revision, &v.RoleID, &v.ScopeKind, &v.ScopeTeamID, &v.SpaceID, &v.TenantID, &v.UserID}
}

// AssignmentItem is an assignment collection row with display names.
type AssignmentItem struct {
	AccessName    string    `json:"access_name"`
	AppID         *string   `json:"app_id"`
	CreatedAt     time.Time `json:"created_at"`
	Domain        string    `json:"domain"`
	GroupID       *string   `json:"group_id"`
	ID            string    `json:"id"`
	PrincipalName *string   `json:"principal_name"`
	Revision      int64     `json:"revision"`
	RoleID        string    `json:"role_id"`
	RoleState     string    `json:"role_state"`
	ScopeKind     string    `json:"scope_kind"`
	ScopeName     string    `json:"scope_name"`
	ScopeTeamID   *string   `json:"scope_team_id"`
	SpaceID       *string   `json:"space_id"`
	UserID        *string   `json:"user_id"`
}

func (v *AssignmentItem) ScanFields() []any {
	return []any{&v.AccessName, &v.AppID, &v.CreatedAt, &v.Domain, &v.GroupID, &v.ID, &v.PrincipalName, &v.Revision, &v.RoleID, &v.RoleState, &v.ScopeKind, &v.ScopeName, &v.ScopeTeamID, &v.SpaceID, &v.UserID}
}

// Recipient is an active Tenant member who can receive a role.
type Recipient struct {
	Name   string  `json:"display_name"`
	Email  *string `json:"email"`
	ID     string  `json:"id"`
	UserID string  `json:"user_id"`
}

func (v *Recipient) ScanFields() []any { return []any{&v.Name, &v.Email, &v.ID, &v.UserID} }

// Named is an id with its display name: a team that can receive a role, or
// a user or team a business record can name or share with.
type Named struct {
	Name string `json:"display_name"`
	ID   string `json:"id"`
}

func (v *Named) ScanFields() []any { return []any{&v.Name, &v.ID} }

// Roles pages the roles defined at scope: fixed platform-defined roles with
// their control permissions, and custom roles with their data permissions.
func (v *Authority) Roles(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, page op.PageRequest) (op.Page[RoleItem], error) {
	if e := v.Run.Authorize(ctx, a, "authorization.read", res); e != nil {
		return op.Page[RoleItem]{}, e
	}
	out, e := op.PageScan[RoleItem](ctx, v.DB, page, op.CollectionFor("roles"), "SELECT "+roleColumns+",(domain='control') AS protected FROM roles WHERE scope_kind=? AND (owner_key='builtin' OR owner_key=?) ORDER BY domain,display_name,id", scope.Kind, scope.OwnerKey())
	if e != nil {
		return out, e
	}
	for i := range out.Items {
		role := &out.Items[i]
		permissions := []string{}
		if def, ok := authz.Builtin(role.ID); ok {
			permissions = def.Permissions
		} else {
			links, err := registry.ScanAll[RolePermission](ctx, v.DB, "SELECT p.code,p.description,p.entity_id,p.id FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=? AND p.domain='data' AND p.state='active'", role.ID)
			if err != nil {
				return op.Page[RoleItem]{}, err
			}
			role.DataPermissions = &links
		}
		role.Permissions = &permissions
	}
	return out, nil
}

// Role reads one role visible at scope; res carries the trusted ancestry.
func (v *Authority) Role(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, record string) (Role, error) {
	if e := v.Run.Authorize(ctx, a, "authorization.read", res); e != nil {
		return Role{}, e
	}
	return registry.ScanOne[Role](ctx, v.DB, "SELECT "+roleColumns+" FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?)", record, scope.Kind, scope.OwnerKey())
}

// CreateRole adds a custom business role to a Space or App; Tenant and team
// roles are fixed.
func (v *Authority) CreateRole(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, in RoleInput) (op.Ref, error) {
	if scope.Kind != authz.Space && scope.Kind != authz.App {
		return op.Ref{}, registry.Bad("Tenant and team roles are fixed.")
	}
	if e := registry.ID(in.ID); e != nil {
		return op.Ref{}, e
	}
	if e := registry.Named(in.Code, in.Name); e != nil {
		return op.Ref{}, e
	}
	if in.State == "" {
		in.State = registry.StateActive
	}
	if !registry.EnabledState(in.State) {
		return op.Ref{}, registry.Bad("Invalid role state.")
	}
	e := v.Run.Mutate(ctx, a, "authorization.manage", res, func(tx *sql.Tx) error {
		if e := registry.MetadataQuota(ctx, tx, scope.Tenant, "roles"); e != nil {
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
			return registry.Conflict("Role limit reached for this scope.")
		}
		space := scope.Space
		if scope.Kind == authz.App {
			if e := tx.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", scope.Tenant, scope.App).Scan(&space); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO roles (id,tenant_id,space_id,app_id,owner_key,scope_kind,domain,code,display_name,state) VALUES (?,?,?,?,?,?,'data',?,?,?)", in.ID, scope.Tenant, space, registry.Nullable(scope.App), scope.OwnerKey(), scope.Kind, in.Code, in.Name, in.State); e != nil {
			return e
		}
		return AuditChangeAs(ctx, tx, a, scope.Tenant, "role.created", "roles", "role", in.ID, nil)
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "roles", scope.Tenant, in.ID)
}

// UpdateRole renames or enables/disables a custom role under its revision.
func (v *Authority) UpdateRole(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, record string, in RolePatch, revision int64) (op.Ref, error) {
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	if in.Name != nil && (strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 160) {
		return op.Ref{}, registry.Bad("Invalid name.")
	}
	if in.State != nil && !registry.EnabledState(*in.State) {
		return op.Ref{}, registry.Bad("Invalid state.")
	}
	e := v.Run.Mutate(ctx, a, "authorization.manage", res, func(tx *sql.Tx) error {
		var domain string
		var current int64
		e := registry.Lookup(ctx, tx, "SELECT domain,revision FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?) FOR UPDATE", []any{record, scope.Kind, scope.OwnerKey()}, &domain, &current)
		if e != nil {
			return e
		}
		if domain == authz.Control {
			return registry.Problem(403, "protected_role", "Platform-defined roles cannot be edited.")
		}
		if e = op.CheckRevision(current, revision); e != nil {
			return e
		}
		before, e := roleSnapshot(ctx, tx, scope.Tenant, record)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE roles SET display_name=COALESCE(?,display_name),state=COALESCE(?,state),revision=revision+1 WHERE id=?", in.Name, in.State, record); e != nil {
			return e
		}
		return AuditChangeAs(ctx, tx, a, scope.Tenant, "role.updated", "roles", "role", record, before)
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "roles", scope.Tenant, record)
}

func AssignmentFilter(scope authz.Scope) (string, []any) {
	where := "g.scope_kind=? AND g.tenant_id <=> ?"
	args := []any{scope.Kind, registry.Nullable(scope.Tenant)}
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

func (v *Authority) Assignments(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, page op.PageRequest) (op.Page[AssignmentItem], error) {
	if e := v.Run.Authorize(ctx, a, assignmentAction(scope, false), res); e != nil {
		return op.Page[AssignmentItem]{}, e
	}
	where, args := AssignmentFilter(scope)
	return op.PageScan[AssignmentItem](ctx, v.DB, page, op.CollectionFor("role-assignments"), `SELECT ro.display_name AS access_name,g.app_id,g.created_at,g.domain,g.group_id,g.id,COALESCE(u.display_name,gr.display_name) AS principal_name,g.revision,g.role_id,ro.state AS role_state,g.scope_kind,CASE WHEN g.scope_kind='platform' THEN 'Platform' ELSE COALESCE(team.display_name,sp.display_name,a.display_name,'Entire Tenant') END AS scope_name,g.scope_team_id,g.space_id,g.user_id FROM access_grants g LEFT JOIN users u ON u.id=g.user_id LEFT JOIN groups gr ON gr.id=g.group_id JOIN roles ro ON ro.id=g.role_id LEFT JOIN groups team ON team.id=g.scope_team_id LEFT JOIN spaces sp ON sp.id=g.space_id LEFT JOIN apps a ON a.id=g.app_id WHERE `+where+" ORDER BY g.created_at DESC,g.id", args...)
}

// Assignment reads one assignment at scope after an authorization.read check
// on res, the trusted ancestry of the requested target.
func (v *Authority) Assignment(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, record string) (Assignment, error) {
	if e := v.Run.Authorize(ctx, a, "authorization.read", res); e != nil {
		return Assignment{}, e
	}
	filter, args := AssignmentFilter(scope)
	args = append(args, record)
	return registry.ScanOne[Assignment](ctx, v.DB, "SELECT "+registry.Prefixed("g", assignmentColumns)+" FROM access_grants g WHERE "+filter+" AND g.id=?", args...)
}

// assignmentAuthority is the role-specific authority to grant or revoke: fixed
// administrative roles need authority above the scope they confer.
func (v *Authority) assignmentAuthority(ctx context.Context, tx *sql.Tx, a op.Actor, scope authz.Scope, res access.Resource, role string) error {
	switch role {
	case authz.SuperRole:
		return v.Run.AuthorizeTx(ctx, tx, a, "platform.admin", access.Resource{})
	case authz.TenantAdminRole, authz.SpaceManagerRole:
		return v.Run.AuthorizeTx(ctx, tx, a, "authorization.manage", access.Resource{Tenant: scope.Tenant})
	case authz.AppManagerRole:
		space := res.Space
		if space == "" {
			if e := tx.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", scope.Tenant, scope.App).Scan(&space); e != nil {
				return e
			}
		}
		return v.Run.AuthorizeTx(ctx, tx, a, "authorization.manage", access.Resource{Tenant: scope.Tenant, Space: space})
	default:
		return v.Run.AuthorizeTx(ctx, tx, a, "authorization.manage", res)
	}
}

// Grant assigns an existing active role at scope to one user or team.
func (v *Authority) Grant(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, in AssignmentInput) (op.Ref, error) {
	for _, value := range []string{in.ID, in.Role} {
		if e := registry.ID(value); e != nil {
			return op.Ref{}, e
		}
	}
	if (in.User == "") == (in.Group == "") {
		return op.Ref{}, registry.Bad("Select one user or team recipient.")
	}
	for _, value := range []string{in.User, in.Group} {
		if value != "" {
			if e := registry.ID(value); e != nil {
				return op.Ref{}, e
			}
		}
	}
	e := v.Run.Mutate(ctx, a, assignmentAction(scope, true), res, func(tx *sql.Tx) error {
		var domain, owner, state string
		e := registry.Lookup(ctx, tx, "SELECT domain,owner_key,state FROM roles WHERE id=? AND scope_kind=? AND (owner_key='builtin' OR owner_key=?)", []any{in.Role, scope.Kind, scope.OwnerKey()}, &domain, &owner, &state)
		if e != nil {
			return e
		}
		if e = v.assignmentAuthority(ctx, tx, a, scope, res, in.Role); e != nil {
			return e
		}
		if state != registry.StateActive {
			return registry.Bad("An active role is required.")
		}
		if domain == authz.Control && in.Group != "" {
			return registry.Bad("Administrative roles require individual users.")
		}

		if scope.Kind != authz.Platform {
			if e = registry.MetadataQuota(ctx, tx, scope.Tenant, "access_grants"); e != nil {
				return e
			}
			if in.User != "" {
				if e = registry.ActiveTenantUser(ctx, tx, scope.Tenant, in.User); e != nil {
					return e
				}
			} else {
				var found string
				if e = registry.Lookup(ctx, tx, "SELECT id FROM groups WHERE tenant_id=? AND id=? AND state='active'", []any{scope.Tenant, in.Group}, &found); e != nil {
					return e
				}
			}
		} else {
			var found string
			if e = registry.Lookup(ctx, tx, "SELECT id FROM users WHERE id=? AND state='active' FOR UPDATE", []any{in.User}, &found); e != nil {
				return e
			}
		}
		if domain == authz.Control {
			if scope.Kind != authz.Platform {
				platform, e := access.IsPlatform(ctx, tx, in.User)
				if e != nil {
					return e
				}
				if platform {
					return registry.Bad("Use a Tenant account for Tenant, team, Space and App roles.")
				}
			} else {
				if e := PlatformCandidate(ctx, tx, in.User); e != nil {
					return e
				}
			}
		}
		if scope.Kind == authz.Team {
			var found string
			if e = registry.Lookup(ctx, tx, "SELECT gm.user_id FROM group_memberships gm JOIN groups team ON team.id=gm.group_id AND team.state='active' WHERE gm.tenant_id=? AND gm.group_id=? AND gm.user_id=? AND gm.state='active'", []any{scope.Tenant, scope.Team, in.User}, &found); e != nil {
				return registry.Bad("An active member of this team is required.")
			}
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,tenant_id,scope_kind,domain,user_id,group_id,scope_team_id,space_id,app_id,role_id,role_owner_key) VALUES (?,?,?,?,?,?,?,?,?,?,?)", in.ID, registry.Nullable(scope.Tenant), scope.Kind, domain, registry.Nullable(in.User), registry.Nullable(in.Group), registry.Nullable(scope.Team), registry.Nullable(scope.Space), registry.Nullable(scope.App), in.Role, owner); e != nil {
			return e
		}
		return AuditChangeAs(ctx, tx, a, scope.Tenant, "grant.created", "access_grants", "access_grant", in.ID, nil)
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "access_grants", scope.Tenant, in.ID)
}

// The recipient user row must already be locked, including during bootstrap.
func PlatformCandidate(ctx context.Context, q registry.Queryer, user string) error {
	var n int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM access_grants WHERE user_id=? AND tenant_id IS NOT NULL AND domain='control'", user).Scan(&n); e != nil {
		return e
	}
	if n > 0 {
		return registry.Conflict("Remove this user's Tenant and team roles before granting platform administration.")
	}
	return nil
}

// Revoke deletes an assignment under its revision, keeping at least one
// platform and one Tenant administrator.
func (v *Authority) Revoke(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, record string, revision int64) (op.Done, error) {
	if revision < 1 {
		return op.Done{}, op.RevisionRequired()
	}
	e := v.Run.Mutate(ctx, a, assignmentAction(scope, true), res, func(tx *sql.Tx) error {
		where, args := AssignmentFilter(scope)
		args = append(args, record)
		var role string
		var current int64
		e := registry.Lookup(ctx, tx, "SELECT g.role_id,g.revision FROM access_grants g WHERE "+where+" AND g.id=? FOR UPDATE", args, &role, &current)
		if e != nil {
			return e
		}
		var found string
		if e = registry.Lookup(ctx, tx, "SELECT id FROM roles WHERE id=?", []any{role}, &found); e != nil {
			return e
		}
		if e = v.assignmentAuthority(ctx, tx, a, scope, res, role); e != nil {
			return e
		}
		if e = op.CheckRevision(current, revision); e != nil {
			return e
		}
		before, e := assignmentSnapshot(ctx, tx, scope.Tenant, record)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM access_grants WHERE id=?", record); e != nil {
			return e
		}
		if role == access.SuperRole {
			if e = access.EnsurePlatformAdmin(ctx, tx); e != nil {
				return e
			}
		}
		if role == access.TenantAdminRole {
			if e = access.EnsureTenantAdmin(ctx, tx, scope.Tenant); e != nil {
				return e
			}
		}
		return op.AuditAs(ctx, tx, a, scope.Tenant, "grant.deleted", "access_grant", record, registry.Row{"before": before, "after": nil})
	})
	if e != nil {
		return op.Done{}, e
	}
	return op.Done{OK: true}, nil
}

// Recipients lets scope managers search eligible users or active teams without
// reading the global user directory or gaining membership authority.
func (v *Authority) Recipients(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, page op.PageRequest) (op.Page[Recipient], error) {
	if e := v.recipientScope(ctx, a, scope, res); e != nil {
		return op.Page[Recipient]{}, e
	}
	return op.PageScan[Recipient](ctx, v.DB, page, op.CollectionFor("recipients"), "SELECT u.display_name,u.email,u.id,u.id AS user_id FROM users u JOIN tenant_memberships m ON m.user_id=u.id WHERE m.tenant_id=? AND m.state='active' AND u.state='active' AND NOT EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) ORDER BY u.display_name,u.id", scope.Tenant, access.SuperRole)
}

// TeamRecipients is Recipients for active teams.
func (v *Authority) TeamRecipients(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource, page op.PageRequest) (op.Page[Named], error) {
	if e := v.recipientScope(ctx, a, scope, res); e != nil {
		return op.Page[Named]{}, e
	}
	return op.PageScan[Named](ctx, v.DB, page, op.CollectionFor("team-recipients"), "SELECT display_name,id FROM groups WHERE tenant_id=? AND state='active' ORDER BY display_name,id", scope.Tenant)
}

func (v *Authority) recipientScope(ctx context.Context, a op.Actor, scope authz.Scope, res access.Resource) error {
	if scope.Kind == authz.Platform {
		return registry.Bad("A Tenant scope is required.")
	}
	return v.Run.Authorize(ctx, a, "authorization.manage", res)
}

// AssignInitialManager gives a new Space or App its manager inside the
// creating transaction.
func AssignInitialManager(ctx context.Context, tx *sql.Tx, a op.Actor, tenant, space, app, user string) error {
	if e := registry.MetadataQuota(ctx, tx, tenant, "access_grants"); e != nil {
		return e
	}
	if e := registry.ID(user); e != nil {
		return registry.Bad("Select an active Tenant member as initial manager.")
	}
	if e := registry.ActiveTenantUser(ctx, tx, tenant, user); e != nil {
		return e
	}
	platform, e := access.IsPlatform(ctx, tx, user)
	if e != nil {
		return e
	}
	if platform {
		return registry.Bad("Choose a Tenant account as initial manager.")
	}
	kind, role := authz.Space, authz.SpaceManagerRole
	var scopeSpace any = space
	var scopeApp any
	if app != "" {
		kind, role = authz.App, authz.AppManagerRole
		scopeSpace = nil
		scopeApp = app
	}
	record := registry.NewID()
	if _, e := tx.ExecContext(ctx, "INSERT INTO access_grants(id,tenant_id,scope_kind,domain,user_id,space_id,app_id,role_id,role_owner_key) VALUES (?,?,?,'control',?,?,?,?,'builtin')", record, tenant, kind, user, scopeSpace, scopeApp, role); e != nil {
		return e
	}
	return AuditChangeAs(ctx, tx, a, tenant, "grant.created", "access_grants", "access_grant", record, nil)
}
