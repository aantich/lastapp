package inventory

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

type PlanInput struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"display_name"`
	Shared     bool    `json:"allow_shared_database"`
	Dedicated  bool    `json:"allow_dedicated_database"`
	Managed    bool    `json:"allow_managed_database"`
	Registered bool    `json:"allow_registered_database"`
	Compute    bool    `json:"allow_dedicated_compute"`
	Default    string  `json:"default_layout"`
	Spaces     *uint32 `json:"max_spaces"`
	Apps       *uint32 `json:"max_apps"`
	Databases  *uint32 `json:"max_databases"`
}

func (in PlanInput) validate() error {
	if e := registry.Named(in.Code, in.Name); e != nil {
		return e
	}
	if !in.Managed && !in.Registered {
		return registry.Bad("Select at least one provisioning method.")
	}
	if (in.Default == "shared" && in.Shared) || (in.Default == "dedicated" && in.Dedicated) {
		return nil
	}
	return registry.Bad("The default layout must be allowed by this plan.")
}

func (v *Inventory) Plans(ctx context.Context, a op.Actor, page op.PageRequest) (op.Page[registry.Plan], error) {
	if e := v.Run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return op.Page[registry.Plan]{}, e
	}
	return op.PageScan[registry.Plan](ctx, v.DB, page, op.CollectionFor("plans"), "SELECT "+registry.PlanColumns+" FROM plans ORDER BY display_name")
}

func (v *Inventory) Plan(ctx context.Context, a op.Actor, record string) (registry.Plan, error) {
	if e := v.Run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return registry.Plan{}, e
	}
	return registry.ScanOne[registry.Plan](ctx, v.DB, "SELECT "+registry.PlanColumns+" FROM plans WHERE id=?", record)
}

func (v *Inventory) CreatePlan(ctx context.Context, a op.Actor, in PlanInput) (op.Ref, error) {
	if e := registry.ID(in.ID); e != nil {
		return op.Ref{}, e
	}
	if e := in.validate(); e != nil {
		return op.Ref{}, e
	}
	e := v.Run.Mutate(ctx, a, "platform.admin", access.Resource{}, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, "INSERT INTO plans (id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", in.ID, in.Code, in.Name, in.Shared, in.Dedicated, in.Managed, in.Registered, in.Compute, in.Default, in.Spaces, in.Apps, in.Databases)
		if e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, "", "plan.created", "plan", in.ID, registry.Row{"code": in.Code})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "plans", "", in.ID)
}

func (v *Inventory) UpdatePlan(ctx context.Context, a op.Actor, record string, in PlanInput, revision int64) (op.Ref, error) {
	if e := in.validate(); e != nil {
		return op.Ref{}, e
	}
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	e := v.Run.Mutate(ctx, a, "platform.admin", access.Resource{}, func(tx *sql.Tx) error {
		var current int64
		var code string
		e := registry.Lookup(ctx, tx, "SELECT revision,code FROM plans WHERE id=? FOR UPDATE", []any{record}, &current, &code)
		if e != nil {
			return e
		}
		if e = op.CheckRevision(current, revision); e != nil {
			return e
		}
		if code != in.Code {
			return registry.Bad("Plan codes are immutable.")
		}
		_, e = tx.ExecContext(ctx, "UPDATE plans SET display_name=?,allow_shared_database=?,allow_dedicated_database=?,allow_managed_database=?,allow_registered_database=?,allow_dedicated_compute=?,default_layout=?,max_spaces=?,max_apps=?,max_databases=?,revision=revision+1 WHERE id=?", in.Name, in.Shared, in.Dedicated, in.Managed, in.Registered, in.Compute, in.Default, in.Spaces, in.Apps, in.Databases, record)
		if e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, "", "plan.updated", "plan", record, registry.Row{"affects": "future allocations"})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "plans", "", record)
}

// User is a global account with its platform administrator assignment.
type User struct {
	CreatedAt                  time.Time    `json:"created_at"`
	Name                       string       `json:"display_name"`
	Email                      *string      `json:"email"`
	ID                         string       `json:"id"`
	OIDCIssuer                 string       `json:"oidc_issuer"`
	OIDCSubject                string       `json:"oidc_subject"`
	PlatformAdmin              registry.Bit `json:"platform_admin"`
	PlatformAssignmentID       *string      `json:"platform_assignment_id"`
	PlatformAssignmentRevision *int64       `json:"platform_assignment_revision"`
	Revision                   int64        `json:"revision"`
	State                      string       `json:"state"`
}

func (v *User) ScanFields() []any {
	return []any{&v.CreatedAt, &v.Name, &v.Email, &v.ID, &v.OIDCIssuer, &v.OIDCSubject, &v.PlatformAdmin, &v.PlatformAssignmentID, &v.PlatformAssignmentRevision, &v.Revision, &v.State}
}

// Tenant is a Tenant with its plan's name. The counts are read only in the
// collection (tenantItem).
type Tenant struct {
	AppCount              *int64    `json:"app_count,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	DefaultLayout         *string   `json:"default_layout"`
	DefaultSharedDatabase *string   `json:"default_shared_database_id"`
	Name                  string    `json:"display_name"`
	ID                    string    `json:"id"`
	PlanID                string    `json:"plan_id"`
	PlanName              string    `json:"plan_name"`
	Revision              int64     `json:"revision"`
	Slug                  string    `json:"slug"`
	SpaceCount            *int64    `json:"space_count,omitempty"`
	State                 string    `json:"state"`
}

const tenantColumns = "t.created_at,t.default_layout,t.default_shared_database_id,t.display_name,t.id,t.plan_id,p.display_name AS plan_name,t.revision,t.slug,t.state"

func (t *Tenant) ScanFields() []any {
	return []any{&t.CreatedAt, &t.DefaultLayout, &t.DefaultSharedDatabase, &t.Name, &t.ID, &t.PlanID, &t.PlanName, &t.Revision, &t.Slug, &t.State}
}

// TenantItem is a Tenant collection row with its active Space and App counts.
type TenantItem struct{ Tenant }

func (t *TenantItem) ScanFields() []any {
	return append(t.Tenant.ScanFields(), &t.SpaceCount, &t.AppCount)
}

type TenantCreateInput struct {
	ID      string                   `json:"id"`
	Slug    string                   `json:"slug"`
	Name    string                   `json:"display_name"`
	Plan    string                   `json:"plan_id"`
	Admin   string                   `json:"initial_admin_user_id"`
	Account *membership.AccountInput `json:"initial_admin"`
	Layout  string                   `json:"default_layout"`
}

type TenantPatch struct {
	Name   *string `json:"display_name"`
	State  *string `json:"state"`
	Plan   *string `json:"plan_id"`
	Layout *string `json:"default_layout"`
}

func (v *Inventory) Tenants(ctx context.Context, a op.Actor, page op.PageRequest) (op.Page[TenantItem], error) {
	if e := v.Run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return op.Page[TenantItem]{}, e
	}
	return op.PageScan[TenantItem](ctx, v.DB, page, op.CollectionFor("tenants"), "SELECT "+tenantColumns+",(SELECT COUNT(*) FROM spaces sp WHERE sp.tenant_id=t.id AND sp.state<>'archived') AS space_count,(SELECT COUNT(*) FROM apps a WHERE a.tenant_id=t.id AND a.state<>'archived') AS app_count FROM tenants t JOIN plans p ON p.id=t.plan_id ORDER BY t.created_at DESC")
}

func (v *Inventory) Tenant(ctx context.Context, a op.Actor, tenant string) (Tenant, error) {
	if e := v.Run.Authorize(ctx, a, "tenant.read", access.Resource{Tenant: tenant}); e != nil {
		return Tenant{}, e
	}
	return registry.ScanOne[Tenant](ctx, v.DB, "SELECT "+tenantColumns+" FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", tenant)
}

// CreateTenant creates a Tenant with its first administrator: an existing
// account or a new local account. Platform administrators cannot be chosen.
func (v *Inventory) CreateTenant(ctx context.Context, a op.Actor, in TenantCreateInput) (op.Ref, error) {
	for _, value := range []string{in.ID, in.Plan} {
		if e := registry.ID(value); e != nil {
			return op.Ref{}, e
		}
	}
	if e := registry.Named(in.Slug, in.Name); e != nil {
		return op.Ref{}, e
	}
	if (in.Admin == "") == (in.Account == nil) {
		return op.Ref{}, registry.Bad("Select an existing administrator or supply a new administrator account.")
	}
	var account membership.PreparedAccount
	if in.Account != nil {
		if e := v.Run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
			return op.Ref{}, e
		}
		var e error
		account, e = v.Passwords.PrepareAccount(*in.Account)
		if e != nil {
			return op.Ref{}, e
		}
	} else if e := registry.ID(in.Admin); e != nil {
		return op.Ref{}, e
	}
	ctx, cancel := account.Deadline(ctx)
	defer cancel()
	e := v.Run.Mutate(ctx, a, "platform.admin", access.Resource{}, func(tx *sql.Tx) error {
		if in.Account != nil {
			var e error
			in.Admin, e = membership.LocalAccount(ctx, tx, account, true)
			if e != nil {
				return e
			}
		}
		policy, e := registry.ScanOne[registry.Plan](ctx, tx, "SELECT "+registry.PlanColumns+" FROM plans WHERE id=?", in.Plan)
		if e != nil {
			return e
		}
		if e = registry.LayoutAllowed(policy, in.Layout); e != nil {
			return e
		}
		var user string
		if e = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE id=? AND state='active'", in.Admin).Scan(&user); e != nil {
			return registry.Bad("Select an active registered administrator.")
		}
		platform, e := access.IsPlatform(ctx, tx, in.Admin)
		if e != nil {
			return e
		}
		if platform {
			return registry.Bad("Choose a Tenant administrator account without platform administration privileges.")
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO tenants (id,slug,display_name,plan_id,default_layout,created_by) VALUES (?,?,?,?,?,?)", in.ID, in.Slug, in.Name, in.Plan, registry.Nullable(in.Layout), a.Principal.ID)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO tenant_memberships (tenant_id,user_id) VALUES (?,?)", in.ID, in.Admin); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,tenant_id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,?,'tenant','control',?,?,'builtin')", registry.NewID(), in.ID, in.Admin, access.TenantAdminRole); e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, in.ID, "tenant.created", "tenant", in.ID, registry.Row{"initial_admin_user_id": in.Admin})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "tenants", "", in.ID)
}

// UpdateTenant is the platform operation on a Tenant's name, lifecycle state,
// plan and default database layout.
func (v *Inventory) UpdateTenant(ctx context.Context, a op.Actor, record string, in TenantPatch, revision int64) (op.Ref, error) {
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	e := v.Run.Mutate(ctx, a, "platform.admin", access.Resource{Tenant: record}, func(tx *sql.Tx) error {
		var name, state, plan, layout string
		var current int64
		e := registry.Lookup(ctx, tx, "SELECT display_name,state,plan_id,COALESCE(default_layout,''),revision FROM tenants WHERE id=?", []any{record}, &name, &state, &plan, &layout, &current)
		if e != nil {
			return e
		}
		if e = op.CheckRevision(current, revision); e != nil {
			return e
		}
		if in.Name != nil {
			name = *in.Name
			if strings.TrimSpace(name) == "" || len(name) > 160 {
				return registry.Bad("Invalid name.")
			}
		}
		if in.State != nil {
			state = *in.State
			if !registry.ValidState(state) {
				return registry.Bad("Invalid state.")
			}
		}
		if in.Plan != nil {
			plan = *in.Plan
		}
		policy, e := registry.ScanOne[registry.Plan](ctx, tx, "SELECT "+registry.PlanColumns+" FROM plans WHERE id=?", plan)
		if e != nil {
			return e
		}
		if in.Layout != nil {
			layout = *in.Layout
			if e = registry.LayoutAllowed(policy, layout); e != nil {
				return e
			}
		}
		_, e = tx.ExecContext(ctx, "UPDATE tenants SET display_name=?,state=?,plan_id=?,default_layout=?,revision=revision+1 WHERE id=?", name, state, plan, registry.Nullable(layout), record)
		if e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, record, "tenant.updated", "tenant", record, registry.Row{"state": state, "plan_id": plan})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "tenants", "", record)
}

// UpdateTenantProfile is the Tenant administrator's company-name change.
func (v *Inventory) UpdateTenantProfile(ctx context.Context, a op.Actor, tenant, name string, revision int64) (op.Ref, error) {
	if strings.TrimSpace(name) == "" || len(name) > 160 {
		return op.Ref{}, registry.Bad("Enter a company name (maximum 160 bytes).")
	}
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	e := v.Run.Mutate(ctx, a, "tenant.manage", access.Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		before, e := registry.LockRecord(ctx, tx, "tenants", "", tenant)
		if e != nil {
			return e
		}
		if e = op.CheckRevision(before.Revision, revision); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE tenants SET display_name=?,revision=revision+1 WHERE id=?", name, tenant); e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, tenant, "tenant.profile_updated", "tenant", tenant, registry.Row{"display_name": name})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.Run.Ref(ctx, "tenants", "", tenant)
}
