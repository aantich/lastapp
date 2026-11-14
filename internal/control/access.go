package control

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/aantich/lastapp/internal/authz"
)

func (s *Server) memberships(w http.ResponseWriter, r *http.Request, p Principal) error {
	t := r.PathValue("tenant")
	if e := s.authorize(r.Context(), s.DB, p, "membership.read", Resource{Tenant: t}, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT m.user_id,m.state,m.revision,m.created_at,u.display_name,u.email,u.state AS user_state,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) AS platform_admin,ag.id AS administrator_id,ag.revision AS administrator_revision FROM tenant_memberships m JOIN users u ON u.id=m.user_id LEFT JOIN access_grants ag ON ag.user_id=m.user_id AND ag.tenant_id=m.tenant_id AND ag.role_id=? WHERE m.tenant_id=? ORDER BY u.display_name,u.id", SuperRole, TenantAdminRole, t)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func (s *Server) addMembership(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		User    string        `json:"user_id"`
		Account *accountInput `json:"account"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	t := r.PathValue("tenant")
	if (in.User == "") == (in.Account == nil) {
		return bad("Select an existing user or supply an account.")
	}
	var account preparedAccount
	if in.Account != nil {
		if e := s.authorize(r.Context(), s.DB, p, "membership.manage", Resource{Tenant: t}, false); e != nil {
			return e
		}
		if in.Account.Password != "" {
			if e := s.authorize(r.Context(), s.DB, p, "platform.admin", Resource{}, false); e != nil {
				return e
			}
		}
		var e error
		account, e = s.prepareAccount(*in.Account)
		if e != nil {
			return e
		}
	} else if e := id(in.User); e != nil {
		return e
	}
	e := s.mutate(r.Context(), p, "membership.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if e := metadataQuota(r.Context(), tx, t, "tenant_memberships"); e != nil {
			return e
		}
		if in.Account != nil {
			platform, e := isPlatform(r.Context(), tx, p.ID)
			if e != nil {
				return e
			}
			in.User, e = localAccount(r.Context(), tx, account, platform)
			if e != nil {
				return e
			}
		}
		var state string
		if e := tx.QueryRowContext(r.Context(), "SELECT state FROM users WHERE id=? FOR UPDATE", in.User).Scan(&state); e != nil {
			if e == sql.ErrNoRows {
				return bad("An active existing user is required.")
			}
			return e
		}
		if state != "active" {
			return bad("An active existing user is required. They must sign in once first.")
		}
		if _, e := tx.ExecContext(r.Context(), "INSERT INTO tenant_memberships (tenant_id,user_id) VALUES (?,?)", t, in.User); e != nil {
			return e
		}
		return audit(r.Context(), tx, t, p.ID, "membership.created", "user", in.User, r.Header.Get("X-Request-ID"), Row{"before": nil, "after": Row{"user_id": in.User, "state": "active"}})
	})
	if e == nil {
		send(w, 201, Row{"user_id": in.User})
	}
	return e
}
func (s *Server) updateMembership(w http.ResponseWriter, r *http.Request, p Principal) error {
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
	t, u := r.PathValue("tenant"), r.PathValue("user")
	e = s.mutate(r.Context(), p, "membership.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		before, e := row(r.Context(), tx, "SELECT user_id,state,revision FROM tenant_memberships WHERE tenant_id=? AND user_id=? FOR UPDATE", t, u)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(before["revision"]), rev); e != nil {
			return e
		}
		res, e := tx.ExecContext(r.Context(), "UPDATE tenant_memberships SET state=?,revision=revision+1 WHERE tenant_id=? AND user_id=?", in.State, t, u)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			var found int
			tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id=?", t, u).Scan(&found)
			if found == 0 {
				return missing()
			}
		}
		if e = ensureTenantAdmin(r.Context(), tx, t); e != nil {
			return e
		}
		if in.State == "disabled" {
			if _, e = tx.ExecContext(r.Context(), "UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE tenant_id=? AND user_id=? AND state='active'", t, u); e != nil {
				return e
			}
		}
		return audit(r.Context(), tx, t, p.ID, "membership.updated", "user", u, r.Header.Get("X-Request-ID"), Row{"before": before, "after": Row{"user_id": u, "state": in.State, "revision": rev + 1}})
	})
	if e == nil {
		send(w, 200, Row{"ok": true})
	}
	return e
}
func (s *Server) groups(w http.ResponseWriter, r *http.Request, p Principal) error {
	t := r.PathValue("tenant")
	sc, e := s.scopes(r.Context(), s.DB, p, t, "team.read")
	if e != nil {
		return e
	}
	filter, args := teamSQL(sc, "g.id")
	args = append([]any{t}, args...)
	v, e := pageRows(r.Context(), s.DB, r, "SELECT g.id,g.slug,g.display_name,g.state,g.revision FROM groups g WHERE g.tenant_id=? AND "+filter+" ORDER BY g.display_name,g.id", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func teamSQL(sc scopeSet, column string) (string, []any) {
	if sc.All {
		return "1=1", nil
	}
	parts := []string{"1=0"}
	args := []any{}
	for _, v := range sc.Teams {
		parts = append(parts, column+"=?")
		args = append(args, v)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}
func (s *Server) createGroup(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in namedInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if e := id(in.ID); e != nil {
		return e
	}
	if e := named(in.Slug, in.Name); e != nil {
		return e
	}
	t := r.PathValue("tenant")
	e := s.mutate(r.Context(), p, "team.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if e := metadataQuota(r.Context(), tx, t, "groups"); e != nil {
			return e
		}
		if _, e := tx.ExecContext(r.Context(), "INSERT INTO groups (id,tenant_id,slug,display_name) VALUES (?,?,?,?)", in.ID, t, in.Slug, in.Name); e != nil {
			return e
		}
		return audit(r.Context(), tx, t, p.ID, "group.created", "group", in.ID, r.Header.Get("X-Request-ID"), Row{"before": nil, "after": Row{"display_name": in.Name, "state": "active"}})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "groups", t, in.ID, 201)
}
func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		Name  *string `json:"display_name"`
		State *string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	t, record := r.PathValue("tenant"), r.PathValue("id")
	e = s.mutate(r.Context(), p, "team.manage", Resource{Tenant: t, Team: record}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		old, e := securitySnapshot(r.Context(), tx, "groups", t, record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), rev); e != nil {
			return e
		}
		name, state := in.Name, in.State
		if name != nil && (strings.TrimSpace(*name) == "" || len(*name) > 160) {
			return bad("Invalid name.")
		}
		if state != nil && *state != "active" && *state != "disabled" {
			return bad("Invalid state.")
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE groups SET display_name=COALESCE(?,display_name),state=COALESCE(?,state),revision=revision+1 WHERE id=? AND tenant_id=?", name, state, record, t); e != nil {
			return e
		}
		return auditChange(r.Context(), tx, t, p.ID, "group.updated", "groups", "group", record, r.Header.Get("X-Request-ID"), old)
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "groups", t, record, 200)
}
func (s *Server) addGroupMember(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.changeGroupMember(w, r, p, true)
}
func (s *Server) removeGroupMember(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.changeGroupMember(w, r, p, false)
}
func (s *Server) changeGroupMember(w http.ResponseWriter, r *http.Request, p Principal, add bool) error {
	u := r.PathValue("user")
	if add {
		var in struct {
			User string `json:"user_id"`
		}
		if e := decode(w, r, &in); e != nil {
			return e
		}
		u = in.User
	}
	if e := id(u); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	t, g := r.PathValue("tenant"), r.PathValue("id")
	e = s.mutate(r.Context(), p, "team.manage", Resource{Tenant: t, Team: g}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		old, e := lockRecord(r.Context(), tx, "groups", t, g)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), rev); e != nil {
			return e
		}
		var wasMember int
		if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM group_memberships WHERE tenant_id=? AND group_id=? AND user_id=?", t, g, u).Scan(&wasMember); e != nil {
			return e
		}
		if !add && wasMember == 0 {
			return missing()
		}
		if add {
			if e := activeTenantUser(r.Context(), tx, t, u); e != nil {
				return e
			}
		}
		if !add {
			removed, e := rows(r.Context(), tx, "SELECT id,role_id FROM access_grants WHERE tenant_id=? AND scope_team_id=? AND user_id=?", t, g, u)
			if e != nil {
				return e
			}
			if _, e = tx.ExecContext(r.Context(), "DELETE FROM access_grants WHERE tenant_id=? AND scope_team_id=? AND user_id=?", t, g, u); e != nil {
				return e
			}
			if len(removed) > 0 {
				if e = audit(r.Context(), tx, t, p.ID, "team.roles.revoked_on_leave", "group", g, r.Header.Get("X-Request-ID"), Row{"user_id": u, "assignments": removed}); e != nil {
					return e
				}
			}
		}
		query := "DELETE FROM group_memberships WHERE tenant_id=? AND group_id=? AND user_id=?"
		if add {
			if e := metadataQuota(r.Context(), tx, t, "group_memberships"); e != nil {
				return e
			}
			query = "INSERT INTO group_memberships (tenant_id,group_id,user_id) VALUES (?,?,?)"
		}
		if _, e = tx.ExecContext(r.Context(), query, t, g, u); e != nil {
			return e
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE groups SET revision=revision+1 WHERE id=?", g); e != nil {
			return e
		}
		return audit(r.Context(), tx, t, p.ID, "group.members_changed", "group", g, r.Header.Get("X-Request-ID"), Row{"before": Row{"user_id": u, "member": wasMember > 0}, "after": Row{"user_id": u, "member": add}})
	})
	if e == nil {
		send(w, 200, Row{"ok": true})
	}
	return e
}
func (s *Server) groupMembers(w http.ResponseWriter, r *http.Request, p Principal) error {
	t, g := r.PathValue("tenant"), r.PathValue("id")
	if e := s.authorize(r.Context(), s.DB, p, "team.manage", Resource{Tenant: t, Team: g}, false); e != nil {
		return e
	}
	if _, e := row(r.Context(), s.DB, "SELECT id FROM groups WHERE tenant_id=? AND id=?", t, g); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT gm.user_id,gm.state,u.display_name,u.state AS user_state,tm.state AS tenant_state,ga.id AS administrator_id,ga.revision AS administrator_revision,gr.id AS reviewer_id,gr.revision AS reviewer_revision,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) AS platform_admin FROM group_memberships gm JOIN users u ON u.id=gm.user_id JOIN tenant_memberships tm ON tm.tenant_id=gm.tenant_id AND tm.user_id=gm.user_id LEFT JOIN access_grants ga ON ga.tenant_id=gm.tenant_id AND ga.scope_team_id=gm.group_id AND ga.user_id=gm.user_id AND ga.role_id=? LEFT JOIN access_grants gr ON gr.tenant_id=gm.tenant_id AND gr.scope_team_id=gm.group_id AND gr.user_id=gm.user_id AND gr.role_id=? WHERE gm.tenant_id=? AND gm.group_id=? ORDER BY u.display_name,gm.user_id", SuperRole, authz.TeamAdminRole, authz.TeamReviewerRole, t, g)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request, p Principal) error {
	t, g := r.PathValue("tenant"), r.PathValue("id")
	if e := s.authorize(r.Context(), s.DB, p, "team.read", Resource{Tenant: t, Team: g}, false); e != nil {
		return e
	}
	v, e := row(r.Context(), s.DB, "SELECT id,slug,display_name,state,revision FROM groups WHERE tenant_id=? AND id=?", t, g)
	if e == nil {
		send(w, 200, v)
	}
	return e
}
func (s *Server) getApp(w http.ResponseWriter, r *http.Request, p Principal) error {
	t, a := r.PathValue("tenant"), r.PathValue("app")
	if e := s.authorize(r.Context(), s.DB, p, "app.read", Resource{Tenant: t, App: a}, false); e != nil {
		return e
	}
	v, e := row(r.Context(), s.DB, "SELECT a.id,a.tenant_id,t.display_name AS tenant_name,a.space_id,sp.workspace_id,a.active_release_id,a.slug,a.display_name,a.state,a.compute_mode,a.revision,sp.display_name AS space_name FROM apps a JOIN tenants t ON t.id=a.tenant_id JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND a.id=?", t, a)
	if e == nil {
		send(w, 200, v)
	}
	return e
}
