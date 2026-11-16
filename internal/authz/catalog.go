// Package authz defines shared role and scope semantics. Business-object ACL
// evaluation is deliberately deferred until the entity API contract is defined.
package authz

import "fmt"

type Kind string

const (
	Platform         Kind = "platform"
	Tenant           Kind = "tenant"
	Team             Kind = "team"
	Space            Kind = "space"
	App              Kind = "app"
	Control               = "control"
	Data                  = "data"
	SuperRole             = "00000000-0000-4000-8000-000000000001"
	TenantAdminRole       = "00000000-0000-4000-8000-000000000002"
	TeamAdminRole         = "00000000-0000-4000-8000-000000000003"
	TeamReviewerRole      = "00000000-0000-4000-8000-000000000004"
	SpaceManagerRole      = "00000000-0000-4000-8000-000000000005"
	AppManagerRole        = "00000000-0000-4000-8000-000000000006"
)

type Role struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"display_name"`
	Scope       Kind     `json:"scope_kind"`
	Permissions []string `json:"permissions"`
}

var (
	actions = []string{"platform.admin", "tenant.read", "tenant.manage", "membership.read", "membership.manage", "team.read", "team.manage", "authorization.read", "authorization.manage", "database.read", "database.manage", "space.read", "space.manage", "app.read", "app.manage", "microapp.read", "microapp.manage", "audit.read", "entity.manage", "entity.publish", "workspace.manage", "release.manage", "production.deploy"}
	catalog = []Role{
		{SuperRole, "platform.superadmin", "Platform superadmin", Platform, []string{"platform.admin"}},
		{TenantAdminRole, "tenant.admin", "Tenant administrator", Tenant, []string{"tenant.read", "tenant.manage", "membership.read", "membership.manage", "team.read", "team.manage", "authorization.read", "authorization.manage", "database.read", "database.manage", "space.read", "space.manage", "app.read", "app.manage", "microapp.read", "microapp.manage", "audit.read", "entity.manage", "entity.publish", "workspace.manage", "release.manage", "production.deploy"}},
		{TeamAdminRole, "team.admin", "Team administrator", Team, []string{"team.read", "team.manage", "authorization.read", "authorization.manage"}},
		{TeamReviewerRole, "team.reviewer", "Team Reviewer", Team, []string{"team.read"}},
		{SpaceManagerRole, "space.manager", "Space manager", Space, []string{"space.read", "space.manage", "app.read", "app.manage", "microapp.read", "microapp.manage", "authorization.read", "authorization.manage", "entity.manage", "entity.publish", "workspace.manage", "release.manage", "production.deploy"}},
		{AppManagerRole, "app.manager", "App manager", App, []string{"app.read", "app.manage", "microapp.read", "microapp.manage", "authorization.read", "authorization.manage"}},
	}
)

func Roles() []Role {
	out := make([]Role, len(catalog))
	copy(out, catalog)
	for i := range out {
		out[i].Permissions = append([]string{}, out[i].Permissions...)
	}
	return out
}
func Actions() []string { return append([]string{}, actions...) }
func IsControlAction(action string) bool {
	for _, a := range actions {
		if action == a {
			return true
		}
	}
	return false
}

func ValidKind(k Kind) bool {
	return k == Platform || k == Tenant || k == Team || k == Space || k == App
}

func Builtin(id string) (Role, bool) {
	for _, r := range Roles() {
		if r.ID == id {
			return r, true
		}
	}
	return Role{}, false
}

// Scope represents exactly one assignment scope. Ancestry is resolved by the
// service from trusted registry rows, never added as another assignment scope.
type Scope struct {
	Kind   Kind   `json:"scope_kind"`
	Tenant string `json:"tenant_id,omitempty"`
	Team   string `json:"scope_team_id,omitempty"`
	Space  string `json:"space_id,omitempty"`
	App    string `json:"app_id,omitempty"`
}

func (s Scope) Validate() error {
	if !ValidKind(s.Kind) {
		return fmt.Errorf("unsupported scope category")
	}
	if s.Kind == Platform {
		if s.Tenant != "" || s.Team != "" || s.Space != "" || s.App != "" {
			return fmt.Errorf("platform scope has no parent")
		}
		return nil
	}
	if s.Tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	if (s.Team != "") != (s.Kind == Team) || (s.Space != "") != (s.Kind == Space) || (s.App != "") != (s.Kind == App) {
		return fmt.Errorf("exactly the matching scope identifier is required")
	}
	return nil
}

func (s Scope) OwnerKey() string {
	if s.Kind == Space {
		return "s:" + s.Space
	}
	if s.Kind == App {
		return "a:" + s.App
	}
	return "builtin"
}
