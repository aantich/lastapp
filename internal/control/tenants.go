package control

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/inventory"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

func (s *Server) plans(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Plans(r.Context(), httpapi.Actor(r, p), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in inventory.PlanInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreatePlan(r.Context(), httpapi.Actor(r, p), in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in inventory.PlanInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.UpdatePlan(r.Context(), httpapi.Actor(r, p), r.PathValue("id"), in, httpapi.IfMatch(r))
	if e == nil {
		httpapi.SendRef(w, 200, v)
	}
	return e
}

func (s *Server) Users(ctx context.Context, actor op.Actor, page op.PageRequest) (op.Page[inventory.User], error) {
	p := actor.Principal
	if e := s.Authorize(ctx, s.DB, p, "platform.admin", access.Resource{}, false); e != nil {
		return op.Page[inventory.User]{}, e
	}
	return op.PageScan[inventory.User](ctx, s.DB, page, op.CollectionFor("users"), "SELECT created_at,display_name,email,id,oidc_issuer,oidc_subject,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_admin,(SELECT pg.id FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_id,(SELECT pg.revision FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_revision,revision,state FROM users ORDER BY display_name,id", access.SuperRole, access.SuperRole, access.SuperRole)
}

// createLocalUser creates a global email/password account without
// memberships or roles; platform administrators only.
func (s *Server) createLocalUser(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	if e := s.Authorize(r.Context(), s.DB, p, "platform.admin", access.Resource{}, false); e != nil {
		return e
	}
	var in membership.AccountInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if in.Password == "" {
		return registry.Bad("A new account requires a name and initial password.")
	}
	account, e := s.members.Passwords.PrepareAccount(in)
	if e != nil {
		return e
	}
	ctx, cancel := op.RenewDeadline(r.Context())
	defer cancel()
	var user string
	e = s.mutate(ctx, p, "platform.admin", access.Resource{}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var e error
		user, e = membership.LocalAccount(ctx, tx, account, true)
		if e != nil {
			return e
		}
		return op.Audit(ctx, tx, "", p.ID, "user.created", "user", user, r.Header.Get("X-Request-ID"), registry.Row{"display_name": account.Name, "email": account.Email})
	})
	if e == nil {
		httpapi.Send(w, 201, registry.Row{"id": user, "display_name": account.Name, "email": account.Email, "state": registry.StateActive})
	}
	return e
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if !registry.EnabledState(in.State) {
		return registry.Bad("State must be active or disabled.")
	}
	rev, e := httpapi.ExpectedRevision(r)
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

func (s *Server) tenants(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Tenants(r.Context(), httpapi.Actor(r, p), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) getTenant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Tenant(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in inventory.TenantCreateInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateTenant(r.Context(), httpapi.Actor(r, p), in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in inventory.TenantPatch
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.UpdateTenant(r.Context(), httpapi.Actor(r, p), r.PathValue("id"), in, httpapi.IfMatch(r))
	if e == nil {
		httpapi.SendRef(w, 200, v)
	}
	return e
}

func (s *Server) updateTenantProfile(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		Name string `json:"display_name"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.UpdateTenantProfile(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), in.Name, httpapi.IfMatch(r))
	if e == nil {
		httpapi.SendRef(w, 200, v)
	}
	return e
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.Users(r.Context(), httpapi.Actor(r, p), httpapi.PageRequest(r.URL.Query()))
	return httpapi.Reply(w, 200, v, e)
}
