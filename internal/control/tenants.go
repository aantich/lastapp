package control

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

type planInput struct {
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

func (in planInput) validate() error {
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

func (v *inventory) Plans(ctx context.Context, a op.Actor, page op.PageRequest) (op.Page[registry.Plan], error) {
	if e := v.run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return op.Page[registry.Plan]{}, e
	}
	return op.PageScan[registry.Plan](ctx, v.db, page, op.CollectionFor("plans"), "SELECT "+registry.PlanColumns+" FROM plans ORDER BY display_name")
}

func (v *inventory) Plan(ctx context.Context, a op.Actor, record string) (registry.Plan, error) {
	if e := v.run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return registry.Plan{}, e
	}
	return registry.ScanOne[registry.Plan](ctx, v.db, "SELECT "+registry.PlanColumns+" FROM plans WHERE id=?", record)
}

func (v *inventory) CreatePlan(ctx context.Context, a op.Actor, in planInput) (op.Ref, error) {
	if e := registry.ID(in.ID); e != nil {
		return op.Ref{}, e
	}
	if e := in.validate(); e != nil {
		return op.Ref{}, e
	}
	e := v.run.Mutate(ctx, a, "platform.admin", access.Resource{}, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, "INSERT INTO plans (id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", in.ID, in.Code, in.Name, in.Shared, in.Dedicated, in.Managed, in.Registered, in.Compute, in.Default, in.Spaces, in.Apps, in.Databases)
		if e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, "", "plan.created", "plan", in.ID, registry.Row{"code": in.Code})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.run.Ref(ctx, "plans", "", in.ID)
}

func (v *inventory) UpdatePlan(ctx context.Context, a op.Actor, record string, in planInput, revision int64) (op.Ref, error) {
	if e := in.validate(); e != nil {
		return op.Ref{}, e
	}
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	e := v.run.Mutate(ctx, a, "platform.admin", access.Resource{}, func(tx *sql.Tx) error {
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
	return v.run.Ref(ctx, "plans", "", record)
}

func (s *Server) plans(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Plans(r.Context(), httpActor(r, p), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in planInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreatePlan(r.Context(), httpActor(r, p), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in planInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.UpdatePlan(r.Context(), httpActor(r, p), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
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

func (s *Server) Users(ctx context.Context, actor op.Actor, page op.PageRequest) (op.Page[User], error) {
	p := actor.Principal
	if e := s.Authorize(ctx, s.DB, p, "platform.admin", access.Resource{}, false); e != nil {
		return op.Page[User]{}, e
	}
	return op.PageScan[User](ctx, s.DB, page, op.CollectionFor("users"), "SELECT created_at,display_name,email,id,oidc_issuer,oidc_subject,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_admin,(SELECT pg.id FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_id,(SELECT pg.revision FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_revision,revision,state FROM users ORDER BY display_name,id", access.SuperRole, access.SuperRole, access.SuperRole)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if !registry.EnabledState(in.State) {
		return registry.Bad("State must be active or disabled.")
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	record := r.PathValue("id")
	e = s.mutate(r.Context(), p, "platform.admin", access.Resource{}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		old, e := registry.LockRecord(r.Context(), tx, "users", "", record)
		if e != nil {
			return e
		}
		ts, e := registry.ScanStrings(r.Context(), tx, "SELECT tenant_id FROM tenant_memberships WHERE user_id=? ORDER BY tenant_id", record)
		if e != nil {
			return e
		}
		for _, t := range ts {
			var v string
			if e = tx.QueryRowContext(r.Context(), "SELECT id FROM tenants WHERE id=? FOR UPDATE", t).Scan(&v); e != nil {
				return e
			}
		}
		if e = op.CheckRevision(old.Revision, rev); e != nil {
			return e
		}
		_, e = tx.ExecContext(r.Context(), "UPDATE users SET state=?,revision=revision+1 WHERE id=?", in.State, record)
		if e != nil {
			return e
		}
		if e = access.EnsurePlatformAdmin(r.Context(), tx); e != nil {
			return e
		}
		for _, t := range ts {
			if e = access.EnsureTenantAdmin(r.Context(), tx, t); e != nil {
				return e
			}
		}
		if in.State == registry.StateDisabled {
			if _, e = tx.ExecContext(r.Context(), "UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE user_id=? AND state='active'", record); e != nil {
				return e
			}
			if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", record); e != nil {
				return e
			}
		}
		return op.Audit(r.Context(), tx, "", p.ID, "user.updated", "user", record, r.Header.Get("X-Request-ID"), registry.Row{"before": registry.Row{"state": old.State}, "after": registry.Row{"state": in.State}})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "users", "", record, 200)
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

// tenantItem is a Tenant collection row with its active Space and App counts.
type tenantItem struct{ Tenant }

func (t *tenantItem) ScanFields() []any {
	return append(t.Tenant.ScanFields(), &t.SpaceCount, &t.AppCount)
}

type tenantCreateInput struct {
	ID      string                   `json:"id"`
	Slug    string                   `json:"slug"`
	Name    string                   `json:"display_name"`
	Plan    string                   `json:"plan_id"`
	Admin   string                   `json:"initial_admin_user_id"`
	Account *membership.AccountInput `json:"initial_admin"`
	Layout  string                   `json:"default_layout"`
}

type tenantPatch struct {
	Name   *string `json:"display_name"`
	State  *string `json:"state"`
	Plan   *string `json:"plan_id"`
	Layout *string `json:"default_layout"`
}

func (v *inventory) Tenants(ctx context.Context, a op.Actor, page op.PageRequest) (op.Page[tenantItem], error) {
	if e := v.run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return op.Page[tenantItem]{}, e
	}
	return op.PageScan[tenantItem](ctx, v.db, page, op.CollectionFor("tenants"), "SELECT "+tenantColumns+",(SELECT COUNT(*) FROM spaces sp WHERE sp.tenant_id=t.id AND sp.state<>'archived') AS space_count,(SELECT COUNT(*) FROM apps a WHERE a.tenant_id=t.id AND a.state<>'archived') AS app_count FROM tenants t JOIN plans p ON p.id=t.plan_id ORDER BY t.created_at DESC")
}

func (v *inventory) Tenant(ctx context.Context, a op.Actor, tenant string) (Tenant, error) {
	if e := v.run.Authorize(ctx, a, "tenant.read", access.Resource{Tenant: tenant}); e != nil {
		return Tenant{}, e
	}
	return registry.ScanOne[Tenant](ctx, v.db, "SELECT "+tenantColumns+" FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", tenant)
}

// CreateTenant creates a Tenant with its first administrator: an existing
// account or a new local account. Platform administrators cannot be chosen.
func (v *inventory) CreateTenant(ctx context.Context, a op.Actor, in tenantCreateInput) (op.Ref, error) {
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
		if e := v.run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
			return op.Ref{}, e
		}
		var e error
		account, e = v.prepareAccount(*in.Account)
		if e != nil {
			return op.Ref{}, e
		}
	} else if e := registry.ID(in.Admin); e != nil {
		return op.Ref{}, e
	}
	ctx, cancel := account.Deadline(ctx)
	defer cancel()
	e := v.run.Mutate(ctx, a, "platform.admin", access.Resource{}, func(tx *sql.Tx) error {
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
	return v.run.Ref(ctx, "tenants", "", in.ID)
}

// UpdateTenant is the platform operation on a Tenant's name, lifecycle state,
// plan and default database layout.
func (v *inventory) UpdateTenant(ctx context.Context, a op.Actor, record string, in tenantPatch, revision int64) (op.Ref, error) {
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	e := v.run.Mutate(ctx, a, "platform.admin", access.Resource{Tenant: record}, func(tx *sql.Tx) error {
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
	return v.run.Ref(ctx, "tenants", "", record)
}

// UpdateTenantProfile is the Tenant administrator's company-name change.
func (v *inventory) UpdateTenantProfile(ctx context.Context, a op.Actor, tenant, name string, revision int64) (op.Ref, error) {
	if strings.TrimSpace(name) == "" || len(name) > 160 {
		return op.Ref{}, registry.Bad("Enter a company name (maximum 160 bytes).")
	}
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	e := v.run.Mutate(ctx, a, "tenant.manage", access.Resource{Tenant: tenant}, func(tx *sql.Tx) error {
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
	return v.run.Ref(ctx, "tenants", "", tenant)
}

func (s *Server) tenants(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Tenants(r.Context(), httpActor(r, p), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getTenant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Tenant(r.Context(), httpActor(r, p), r.PathValue("tenant"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in tenantCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateTenant(r.Context(), httpActor(r, p), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in tenantPatch
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.UpdateTenant(r.Context(), httpActor(r, p), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) updateTenantProfile(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		Name string `json:"display_name"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.UpdateTenantProfile(r.Context(), httpActor(r, p), r.PathValue("tenant"), in.Name, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.Users(r.Context(), httpActor(r, p), pageRequest(r.URL.Query()))
	return reply(w, 200, v, e)
}
