package control

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

// AddAdministrator makes an existing account or a new local account a Tenant
// administrator: account, membership and authority are one platform operation.
// Existing credentials are never replaced; no Tenant-authoring authority is added.
func (m *membership) AddAdministrator(ctx context.Context, a op.Actor, tenant string, in accountChoice) (memberRef, error) {
	if e := m.run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return memberRef{}, e
	}
	if (in.User == "") == (in.Account == nil) {
		return memberRef{}, registry.Bad("Choose an existing account or enter account details.")
	}
	var account preparedAccount
	var e error
	if in.Account != nil {
		account, e = m.prepareAccount(*in.Account)
	} else {
		e = registry.ID(in.User)
	}
	if e != nil {
		return memberRef{}, e
	}
	ctx, cancel := account.deadline(ctx)
	defer cancel()
	e = m.run.Mutate(ctx, a, "authorization.manage", access.Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		if e := m.run.AuthorizeTx(ctx, tx, a, "platform.admin", access.Resource{}); e != nil {
			return e
		}
		if in.Account != nil {
			var e error
			in.User, e = localAccount(ctx, tx, account, true)
			if e != nil {
				return e
			}
		}
		var state string
		if e := tx.QueryRowContext(ctx, "SELECT state FROM users WHERE id=? FOR UPDATE", in.User).Scan(&state); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return registry.Missing()
			}
			return e
		}
		if state != registry.StateActive {
			return registry.Bad("Enable this account before assigning Tenant administration.")
		}
		platform, e := access.IsPlatform(ctx, tx, in.User)
		if e != nil {
			return e
		}
		if platform {
			return registry.Bad("Choose an account without platform administration privileges.")
		}
		var memberState string
		e = tx.QueryRowContext(ctx, "SELECT state FROM tenant_memberships WHERE tenant_id=? AND user_id=? FOR UPDATE", tenant, in.User).Scan(&memberState)
		if errors.Is(e, sql.ErrNoRows) {
			if e = registry.MetadataQuota(ctx, tx, tenant, "tenant_memberships"); e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO tenant_memberships(tenant_id,user_id) VALUES (?,?)", tenant, in.User); e != nil {
				return e
			}
			if e = op.AuditAs(ctx, tx, a, tenant, "membership.created", "user", in.User, registry.Row{"state": registry.StateActive}); e != nil {
				return e
			}
		} else if e != nil {
			return e
		} else if memberState != registry.StateActive {
			if _, e = tx.ExecContext(ctx, "UPDATE tenant_memberships SET state='active',revision=revision+1 WHERE tenant_id=? AND user_id=?", tenant, in.User); e != nil {
				return e
			}
			if e = op.AuditAs(ctx, tx, a, tenant, "membership.updated", "user", in.User, registry.Row{"before": registry.Row{"state": memberState}, "after": registry.Row{"state": registry.StateActive}}); e != nil {
				return e
			}
		}
		var count int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM access_grants WHERE tenant_id=? AND user_id=? AND role_id=?", tenant, in.User, access.TenantAdminRole).Scan(&count); e != nil {
			return e
		}
		if count == 0 {
			if e = registry.MetadataQuota(ctx, tx, tenant, "access_grants"); e != nil {
				return e
			}
			grant := registry.NewID()
			if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants(id,tenant_id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,?,'tenant','control',?,?,'builtin')", grant, tenant, in.User, access.TenantAdminRole); e != nil {
				return e
			}
			return roles.AuditChangeAs(ctx, tx, a, tenant, "grant.created", "access_grants", "access_grant", grant, nil)
		}
		return nil
	})
	if e != nil {
		return memberRef{}, e
	}
	return memberRef{UserID: in.User}, nil
}

// Administrator is a Tenant administrator assignment with its account and
// membership state.
type Administrator struct {
	Name      string  `json:"display_name"`
	Email     *string `json:"email"`
	ID        string  `json:"id"`
	Revision  int64   `json:"revision"`
	State     string  `json:"state"`
	UserID    string  `json:"user_id"`
	UserState string  `json:"user_state"`
}

func (v *Administrator) ScanFields() []any {
	return []any{&v.Name, &v.Email, &v.ID, &v.Revision, &v.State, &v.UserID, &v.UserState}
}

func (m *membership) Administrators(ctx context.Context, a op.Actor, tenant string, page op.PageRequest) (op.Page[Administrator], error) {
	if e := m.run.Authorize(ctx, a, "platform.admin", access.Resource{}); e != nil {
		return op.Page[Administrator]{}, e
	}
	return op.PageScan[Administrator](ctx, m.db, page, op.CollectionFor("administrators"), "SELECT u.display_name,u.email,g.id,g.revision,m.state,g.user_id,u.state AS user_state FROM access_grants g JOIN users u ON u.id=g.user_id JOIN tenant_memberships m ON m.tenant_id=g.tenant_id AND m.user_id=g.user_id WHERE g.tenant_id=? AND g.role_id=? ORDER BY u.display_name,u.id", tenant, access.TenantAdminRole)
}

func (s *Server) addTenantAdministrator(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in accountChoice
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddAdministrator(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		send(w, 201, v)
	}
	return e
}

func (s *Server) tenantAdministrators(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Administrators(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}
