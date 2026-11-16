package control

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

const planColumns = "id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases,revision,created_at"

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
	if e := named(in.Code, in.Name); e != nil {
		return e
	}
	if !in.Managed && !in.Registered {
		return bad("Select at least one provisioning method.")
	}
	if (in.Default == "shared" && in.Shared) || (in.Default == "dedicated" && in.Dedicated) {
		return nil
	}
	return bad("The default layout must be allowed by this plan.")
}

func (v *inventory) Plans(ctx context.Context, a Actor, page PageRequest) (Page[Row], error) {
	if e := v.run.authorize(ctx, a, "platform.admin", Resource{}); e != nil {
		return Page[Row]{}, e
	}
	return pageQuery(ctx, v.db, page, collectionFor("plans"), "SELECT "+planColumns+" FROM plans ORDER BY display_name")
}

func (v *inventory) Plan(ctx context.Context, a Actor, record string) (Row, error) {
	if e := v.run.authorize(ctx, a, "platform.admin", Resource{}); e != nil {
		return nil, e
	}
	return row(ctx, v.db, "SELECT "+planColumns+" FROM plans WHERE id=?", record)
}

func (v *inventory) CreatePlan(ctx context.Context, a Actor, in planInput) (Ref, error) {
	if e := id(in.ID); e != nil {
		return Ref{}, e
	}
	if e := in.validate(); e != nil {
		return Ref{}, e
	}
	e := v.run.mutate(ctx, a, "platform.admin", Resource{}, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, "INSERT INTO plans (id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", in.ID, in.Code, in.Name, in.Shared, in.Dedicated, in.Managed, in.Registered, in.Compute, in.Default, in.Spaces, in.Apps, in.Databases)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, "", "plan.created", "plan", in.ID, Row{"code": in.Code})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "plans", "", in.ID)
}

func (v *inventory) UpdatePlan(ctx context.Context, a Actor, record string, in planInput, revision int64) (Ref, error) {
	if e := in.validate(); e != nil {
		return Ref{}, e
	}
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	e := v.run.mutate(ctx, a, "platform.admin", Resource{}, func(tx *sql.Tx) error {
		old, e := row(ctx, tx, "SELECT id,revision,code FROM plans WHERE id=? FOR UPDATE", record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), revision); e != nil {
			return e
		}
		if str(old["code"]) != in.Code {
			return bad("Plan codes are immutable.")
		}
		_, e = tx.ExecContext(ctx, "UPDATE plans SET display_name=?,allow_shared_database=?,allow_dedicated_database=?,allow_managed_database=?,allow_registered_database=?,allow_dedicated_compute=?,default_layout=?,max_spaces=?,max_apps=?,max_databases=?,revision=revision+1 WHERE id=?", in.Name, in.Shared, in.Dedicated, in.Managed, in.Registered, in.Compute, in.Default, in.Spaces, in.Apps, in.Databases, record)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, "", "plan.updated", "plan", record, Row{"affects": "future allocations"})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "plans", "", record)
}

func (s *Server) plans(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.inv.Plans(r.Context(), httpActor(r, p), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request, p Principal) error {
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

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request, p Principal) error {
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

func (s *Server) users(w http.ResponseWriter, r *http.Request, p Principal) error {
	if e := s.authorize(r.Context(), s.DB, p, "platform.admin", Resource{}, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT id,display_name,email,oidc_issuer,oidc_subject,state,revision,created_at,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_admin,(SELECT pg.id FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_id,(SELECT pg.revision FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_revision FROM users ORDER BY display_name,id", SuperRole, SuperRole, SuperRole)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if !enabledState(in.State) {
		return bad("State must be active or disabled.")
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	record := r.PathValue("id")
	e = s.mutate(r.Context(), p, "platform.admin", Resource{}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		old, e := lockRecord(r.Context(), tx, "users", "", record)
		if e != nil {
			return e
		}
		ts, e := rows(r.Context(), tx, "SELECT tenant_id FROM tenant_memberships WHERE user_id=? ORDER BY tenant_id", record)
		if e != nil {
			return e
		}
		for _, t := range ts {
			var v string
			if e = tx.QueryRowContext(r.Context(), "SELECT id FROM tenants WHERE id=? FOR UPDATE", t["tenant_id"]).Scan(&v); e != nil {
				return e
			}
		}
		if e = checkRevision(integer(old["revision"]), rev); e != nil {
			return e
		}
		_, e = tx.ExecContext(r.Context(), "UPDATE users SET state=?,revision=revision+1 WHERE id=?", in.State, record)
		if e != nil {
			return e
		}
		if e = ensurePlatformAdmin(r.Context(), tx); e != nil {
			return e
		}
		for _, t := range ts {
			if e = ensureTenantAdmin(r.Context(), tx, str(t["tenant_id"])); e != nil {
				return e
			}
		}
		if in.State == stateDisabled {
			if _, e = tx.ExecContext(r.Context(), "UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE user_id=? AND state='active'", record); e != nil {
				return e
			}
			if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", record); e != nil {
				return e
			}
		}
		return audit(r.Context(), tx, "", p.ID, "user.updated", "user", record, r.Header.Get("X-Request-ID"), Row{"before": Row{"state": old["state"]}, "after": Row{"state": in.State}})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "users", "", record, 200)
}

const tenantColumns = "t.id,t.slug,t.display_name,t.state,t.plan_id,t.default_layout,t.default_shared_database_id,t.revision,t.created_at,p.display_name AS plan_name"

type tenantCreateInput struct {
	ID      string        `json:"id"`
	Slug    string        `json:"slug"`
	Name    string        `json:"display_name"`
	Plan    string        `json:"plan_id"`
	Admin   string        `json:"initial_admin_user_id"`
	Account *accountInput `json:"initial_admin"`
	Layout  string        `json:"default_layout"`
}

type tenantPatch struct {
	Name   *string `json:"display_name"`
	State  *string `json:"state"`
	Plan   *string `json:"plan_id"`
	Layout *string `json:"default_layout"`
}

func (v *inventory) Tenants(ctx context.Context, a Actor, page PageRequest) (Page[Row], error) {
	if e := v.run.authorize(ctx, a, "platform.admin", Resource{}); e != nil {
		return Page[Row]{}, e
	}
	return pageQuery(ctx, v.db, page, collectionFor("tenants"), "SELECT "+tenantColumns+",(SELECT COUNT(*) FROM spaces sp WHERE sp.tenant_id=t.id AND sp.state<>'archived') AS space_count,(SELECT COUNT(*) FROM apps a WHERE a.tenant_id=t.id AND a.state<>'archived') AS app_count FROM tenants t JOIN plans p ON p.id=t.plan_id ORDER BY t.created_at DESC")
}

func (v *inventory) Tenant(ctx context.Context, a Actor, tenant string) (Row, error) {
	if e := v.run.authorize(ctx, a, "tenant.read", Resource{Tenant: tenant}); e != nil {
		return nil, e
	}
	return row(ctx, v.db, "SELECT "+tenantColumns+" FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", tenant)
}

// CreateTenant creates a Tenant with its first administrator: an existing
// account or a new local account. Platform administrators cannot be chosen.
func (v *inventory) CreateTenant(ctx context.Context, a Actor, in tenantCreateInput) (Ref, error) {
	for _, value := range []string{in.ID, in.Plan} {
		if e := id(value); e != nil {
			return Ref{}, e
		}
	}
	if e := named(in.Slug, in.Name); e != nil {
		return Ref{}, e
	}
	if (in.Admin == "") == (in.Account == nil) {
		return Ref{}, bad("Select an existing administrator or supply a new administrator account.")
	}
	var account preparedAccount
	if in.Account != nil {
		if e := v.run.authorize(ctx, a, "platform.admin", Resource{}); e != nil {
			return Ref{}, e
		}
		var e error
		account, e = v.prepareAccount(*in.Account)
		if e != nil {
			return Ref{}, e
		}
	} else if e := id(in.Admin); e != nil {
		return Ref{}, e
	}
	e := v.run.mutate(ctx, a, "platform.admin", Resource{}, func(tx *sql.Tx) error {
		if in.Account != nil {
			var e error
			in.Admin, e = localAccount(ctx, tx, account, true)
			if e != nil {
				return e
			}
		}
		policy, e := row(ctx, tx, "SELECT "+planColumns+" FROM plans WHERE id=?", in.Plan)
		if e != nil {
			return e
		}
		if e = layoutAllowed(policy, in.Layout); e != nil {
			return e
		}
		var user string
		if e = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE id=? AND state='active'", in.Admin).Scan(&user); e != nil {
			return bad("Select an active registered administrator.")
		}
		platform, e := isPlatform(ctx, tx, in.Admin)
		if e != nil {
			return e
		}
		if platform {
			return bad("Choose a Tenant administrator account without platform administration privileges.")
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO tenants (id,slug,display_name,plan_id,default_layout,created_by) VALUES (?,?,?,?,?,?)", in.ID, in.Slug, in.Name, in.Plan, nullable(in.Layout), a.Principal.ID)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO tenant_memberships (tenant_id,user_id) VALUES (?,?)", in.ID, in.Admin); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,tenant_id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,?,'tenant','control',?,?,'builtin')", newID(), in.ID, in.Admin, TenantAdminRole); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, in.ID, "tenant.created", "tenant", in.ID, Row{"initial_admin_user_id": in.Admin})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "tenants", "", in.ID)
}

// UpdateTenant is the platform operation on a Tenant's name, lifecycle state,
// plan and default database layout.
func (v *inventory) UpdateTenant(ctx context.Context, a Actor, record string, in tenantPatch, revision int64) (Ref, error) {
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	e := v.run.mutate(ctx, a, "platform.admin", Resource{Tenant: record}, func(tx *sql.Tx) error {
		old, e := row(ctx, tx, "SELECT id,display_name,state,plan_id,default_layout,revision FROM tenants WHERE id=?", record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), revision); e != nil {
			return e
		}
		name, state, plan, layout := str(old["display_name"]), str(old["state"]), str(old["plan_id"]), str(old["default_layout"])
		if in.Name != nil {
			name = *in.Name
			if strings.TrimSpace(name) == "" || len(name) > 160 {
				return bad("Invalid name.")
			}
		}
		if in.State != nil {
			state = *in.State
			if !validState(state) {
				return bad("Invalid state.")
			}
		}
		if in.Plan != nil {
			plan = *in.Plan
		}
		policy, e := row(ctx, tx, "SELECT "+planColumns+" FROM plans WHERE id=?", plan)
		if e != nil {
			return e
		}
		if in.Layout != nil {
			layout = *in.Layout
			if e = layoutAllowed(policy, layout); e != nil {
				return e
			}
		}
		_, e = tx.ExecContext(ctx, "UPDATE tenants SET display_name=?,state=?,plan_id=?,default_layout=?,revision=revision+1 WHERE id=?", name, state, plan, nullable(layout), record)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, record, "tenant.updated", "tenant", record, Row{"state": state, "plan_id": plan})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "tenants", "", record)
}

// UpdateTenantProfile is the Tenant administrator's company-name change.
func (v *inventory) UpdateTenantProfile(ctx context.Context, a Actor, tenant, name string, revision int64) (Ref, error) {
	if strings.TrimSpace(name) == "" || len(name) > 160 {
		return Ref{}, bad("Enter a company name (maximum 160 bytes).")
	}
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	e := v.run.mutate(ctx, a, "tenant.manage", Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		before, e := lockRecord(ctx, tx, "tenants", "", tenant)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(before["revision"]), revision); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE tenants SET display_name=?,revision=revision+1 WHERE id=?", name, tenant); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "tenant.profile_updated", "tenant", tenant, Row{"display_name": name})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "tenants", "", tenant)
}

func (s *Server) tenants(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.inv.Tenants(r.Context(), httpActor(r, p), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getTenant(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.inv.Tenant(r.Context(), httpActor(r, p), r.PathValue("tenant"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request, p Principal) error {
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

func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request, p Principal) error {
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

func (s *Server) updateTenantProfile(w http.ResponseWriter, r *http.Request, p Principal) error {
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

func layoutAllowed(policy Row, layout string) error {
	if layout == "" {
		return nil
	}
	if (layout == "shared" && integer(policy["allow_shared_database"]) == 1) || (layout == "dedicated" && integer(policy["allow_dedicated_database"]) == 1) {
		return nil
	}
	return problem(403, "plan_restriction", "This database layout is not permitted by the selected plan.")
}

func policyFor(ctx context.Context, q Queryer, tenant string) (Row, error) {
	v, e := row(ctx, q, "SELECT p.*,t.state AS tenant_state,t.default_layout AS tenant_default_layout,t.default_shared_database_id FROM plans p JOIN tenants t ON t.plan_id=p.id WHERE t.id=?", tenant)
	if e != nil {
		return nil, e
	}
	if str(v["tenant_state"]) != stateActive {
		return nil, conflict("The Tenant must be active to allocate resources.")
	}
	return v, nil
}

func quota(ctx context.Context, q Queryer, tenant, table, column string, policy Row) error {
	if policy[column] == nil {
		return nil
	}
	var n int
	state := "state<>'archived'"
	if table == "spaces" {
		state += " AND workspace_id IS NULL"
	}
	if table == "apps" {
		state += " AND NOT EXISTS(SELECT 1 FROM spaces sp WHERE sp.id=apps.space_id AND sp.workspace_id IS NOT NULL)"
	}
	if table == "database_resources" {
		state = "1=1"
	}
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id=? AND "+state, tenant).Scan(&n); e != nil {
		return e
	}
	if int64(n) >= integer(policy[column]) {
		return problem(409, "quota_exceeded", "The Tenant's "+strings.TrimPrefix(column, "max_")+" limit has been reached.")
	}
	return nil
}

func ensurePlatformAdmin(ctx context.Context, q Queryer) error {
	var n int
	e := q.QueryRowContext(ctx, "SELECT COUNT(DISTINCT g.user_id) FROM access_grants g JOIN users u ON u.id=g.user_id AND u.state='active' WHERE g.scope_kind='platform' AND g.role_id=?", SuperRole).Scan(&n)
	if e != nil {
		return e
	}
	if n < 1 {
		return conflict("Keep at least one active platform administrator.")
	}
	return nil
}

func ensureTenantAdmin(ctx context.Context, q Queryer, tenant string) error {
	var n int
	e := q.QueryRowContext(ctx, "SELECT COUNT(DISTINCT g.user_id) FROM access_grants g JOIN users u ON u.id=g.user_id AND u.state='active' JOIN tenant_memberships m ON m.tenant_id=g.tenant_id AND m.user_id=g.user_id AND m.state='active' WHERE g.tenant_id=? AND g.role_id=? AND g.space_id IS NULL AND g.app_id IS NULL AND NOT EXISTS (SELECT 1 FROM access_grants pg WHERE pg.user_id=g.user_id AND pg.role_id=?)", tenant, TenantAdminRole, SuperRole).Scan(&n)
	if e != nil {
		return e
	}
	if n < 1 {
		return conflict("Keep at least one active, directly assigned Tenant administrator.")
	}
	return nil
}
