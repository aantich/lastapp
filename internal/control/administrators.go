package control

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/aantich/lastapp/internal/authz"
)

// Account, membership and authority are one operation for the platform operator.
// Existing credentials are never replaced; no Tenant-authoring authority is added.
func (s *Server) addTenantAdministrator(w http.ResponseWriter, r *http.Request, p Principal) error {
	if e := s.authorize(r.Context(), s.DB, p, "platform.admin", Resource{}, false); e != nil {
		return e
	}
	var in struct {
		User    string        `json:"user_id"`
		Account *accountInput `json:"account"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if (in.User == "") == (in.Account == nil) {
		return bad("Choose an existing account or enter account details.")
	}
	var account preparedAccount
	var e error
	if in.Account != nil {
		account, e = s.prepareAccount(*in.Account)
	} else {
		e = id(in.User)
	}
	if e != nil {
		return e
	}
	t := r.PathValue("tenant")
	e = s.mutate(r.Context(), p, "authorization.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if e := s.authorize(r.Context(), tx, p, "platform.admin", Resource{}, true); e != nil {
			return e
		}
		if in.Account != nil {
			var e error
			in.User, e = localAccount(r.Context(), tx, account, true)
			if e != nil {
				return e
			}
		}
		var state string
		if e := tx.QueryRowContext(r.Context(), "SELECT state FROM users WHERE id=? FOR UPDATE", in.User).Scan(&state); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return missing()
			}
			return e
		}
		if state != stateActive {
			return bad("Enable this account before assigning Tenant administration.")
		}
		platform, e := isPlatform(r.Context(), tx, in.User)
		if e != nil {
			return e
		}
		if platform {
			return bad("Choose an account without platform administration privileges.")
		}
		var memberState string
		e = tx.QueryRowContext(r.Context(), "SELECT state FROM tenant_memberships WHERE tenant_id=? AND user_id=? FOR UPDATE", t, in.User).Scan(&memberState)
		if errors.Is(e, sql.ErrNoRows) {
			if e = metadataQuota(r.Context(), tx, t, "tenant_memberships"); e != nil {
				return e
			}
			if _, e = tx.ExecContext(r.Context(), "INSERT INTO tenant_memberships(tenant_id,user_id) VALUES (?,?)", t, in.User); e != nil {
				return e
			}
			if e = audit(r.Context(), tx, t, p.ID, "membership.created", "user", in.User, r.Header.Get("X-Request-ID"), Row{"state": stateActive}); e != nil {
				return e
			}
		} else if e != nil {
			return e
		} else if memberState != stateActive {
			if _, e = tx.ExecContext(r.Context(), "UPDATE tenant_memberships SET state='active',revision=revision+1 WHERE tenant_id=? AND user_id=?", t, in.User); e != nil {
				return e
			}
			if e = audit(r.Context(), tx, t, p.ID, "membership.updated", "user", in.User, r.Header.Get("X-Request-ID"), Row{"before": Row{"state": memberState}, "after": Row{"state": stateActive}}); e != nil {
				return e
			}
		}
		var count int
		if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM access_grants WHERE tenant_id=? AND user_id=? AND role_id=?", t, in.User, TenantAdminRole).Scan(&count); e != nil {
			return e
		}
		if count == 0 {
			if e = metadataQuota(r.Context(), tx, t, "access_grants"); e != nil {
				return e
			}
			grant := newID()
			if _, e = tx.ExecContext(r.Context(), "INSERT INTO access_grants(id,tenant_id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,?,'tenant','control',?,?,'builtin')", grant, t, in.User, TenantAdminRole); e != nil {
				return e
			}
			return auditChange(r.Context(), tx, t, p.ID, "grant.created", "access_grants", "access_grant", grant, r.Header.Get("X-Request-ID"), nil)
		}
		return nil
	})
	if e == nil {
		send(w, 201, Row{"user_id": in.User})
	}
	return e
}

func (s *Server) tenantAdministrators(w http.ResponseWriter, r *http.Request, p Principal) error {
	t := r.PathValue("tenant")
	if e := s.authorize(r.Context(), s.DB, p, "platform.admin", Resource{}, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT g.id,g.user_id,g.revision,u.display_name,u.email,u.state AS user_state,m.state FROM access_grants g JOIN users u ON u.id=g.user_id JOIN tenant_memberships m ON m.tenant_id=g.tenant_id AND m.user_id=g.user_id WHERE g.tenant_id=? AND g.role_id=? ORDER BY u.display_name,u.id", t, TenantAdminRole)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}

// Scope managers can search eligible recipients without reading the global user
// directory or receiving broader membership-management authority.
func (s *Server) recipientOptions(w http.ResponseWriter, r *http.Request, p Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	if scope.Kind == authz.Platform {
		return bad("A Tenant scope is required.")
	}
	if e = s.authorize(r.Context(), s.DB, p, "authorization.manage", res, false); e != nil {
		return e
	}
	var v []Row
	if r.URL.Query().Get("type") == "team" {
		v, e = pageRows(r.Context(), s.DB, r, "SELECT id,display_name FROM groups WHERE tenant_id=? AND state='active' ORDER BY display_name,id", scope.Tenant)
	} else {
		v, e = pageRows(r.Context(), s.DB, r, "SELECT u.id,u.id AS user_id,u.display_name,u.email FROM users u JOIN tenant_memberships m ON m.user_id=u.id WHERE m.tenant_id=? AND m.state='active' AND u.state='active' AND NOT EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) ORDER BY u.display_name,u.id", scope.Tenant, SuperRole)
	}
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
