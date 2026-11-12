package control

import (
	"database/sql"
	"net/http"
	"strings"
)

func (s *Server) memberships(w http.ResponseWriter, r *http.Request, p Principal) error {
	t := r.PathValue("tenant")
	if e := s.authorize(r.Context(), s.DB, p, "membership.read", Resource{Tenant: t}, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT m.user_id,m.state,m.created_at,u.display_name,u.email,u.state AS user_state FROM tenant_memberships m JOIN users u ON u.id=m.user_id WHERE m.tenant_id=? ORDER BY u.display_name", t)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func (s *Server) addMembership(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		User string `json:"user_id"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if e := id(in.User); e != nil {
		return e
	}
	t := r.PathValue("tenant")
	e := s.mutate(r.Context(), p, "membership.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if e := metadataQuota(r.Context(), tx, t, "tenant_memberships"); e != nil {
			return e
		}
		var active int
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE id=? AND state='active'", in.User).Scan(&active); e != nil {
			return e
		}
		if active != 1 {
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
	t, u := r.PathValue("tenant"), r.PathValue("user")
	e := s.mutate(r.Context(), p, "membership.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		before, e := row(r.Context(), tx, "SELECT user_id,state FROM tenant_memberships WHERE tenant_id=? AND user_id=?", t, u)
		if e != nil {
			return e
		}
		res, e := tx.ExecContext(r.Context(), "UPDATE tenant_memberships SET state=? WHERE tenant_id=? AND user_id=?", in.State, t, u)
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
		return audit(r.Context(), tx, t, p.ID, "membership.updated", "user", u, r.Header.Get("X-Request-ID"), Row{"before": before, "after": Row{"user_id": u, "state": in.State}})
	})
	if e == nil {
		send(w, 200, Row{"ok": true})
	}
	return e
}
func (s *Server) groups(w http.ResponseWriter, r *http.Request, p Principal) error {
	t := r.PathValue("tenant")
	if e := s.authorize(r.Context(), s.DB, p, "authorization.read", Resource{Tenant: t}, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT id,slug,display_name,state,revision FROM groups WHERE tenant_id=? ORDER BY display_name", t)
	if e != nil {
		return e
	}
	byID := map[string]Row{}
	placeholders := []string{}
	args := []any{t}
	for _, g := range v {
		g["members"] = []Row{}
		g["members_count"] = 0
		byID[str(g["id"])] = g
		placeholders = append(placeholders, "?")
		args = append(args, g["id"])
	}
	if len(v) > 0 {
		members, e := rows(r.Context(), s.DB, "SELECT group_id,user_id,display_name,member_count FROM (SELECT gm.group_id,gm.user_id,u.display_name,COUNT(*) OVER (PARTITION BY gm.group_id) AS member_count,ROW_NUMBER() OVER (PARTITION BY gm.group_id ORDER BY u.display_name,gm.user_id) AS member_number FROM group_memberships gm JOIN users u ON u.id=gm.user_id WHERE gm.tenant_id=? AND gm.group_id IN ("+strings.Join(placeholders, ",")+")) AS ranked WHERE member_number<=10 ORDER BY group_id,member_number", args...)
		if e != nil {
			return e
		}
		for _, m := range members {
			g := byID[str(m["group_id"])]
			g["members_count"] = m["member_count"]
			g["members"] = append(g["members"].([]Row), Row{"user_id": m["user_id"], "display_name": m["display_name"]})
		}
	}
	sendPage(w, r, v)
	return nil
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
	e := s.mutate(r.Context(), p, "authorization.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
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
	e = s.mutate(r.Context(), p, "authorization.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
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
	e = s.mutate(r.Context(), p, "authorization.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
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
func (s *Server) roles(w http.ResponseWriter, r *http.Request, p Principal) error {
	t := r.PathValue("tenant")
	if e := s.authorize(r.Context(), s.DB, p, "authorization.read", Resource{Tenant: t}, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT id,tenant_id,code,display_name,state,revision,(tenant_id IS NULL) AS protected FROM roles WHERE kind='tenant' AND (tenant_id=? OR tenant_id IS NULL) ORDER BY tenant_id,display_name", t)
	if e != nil {
		return e
	}
	for _, role := range v {
		ps, e := rows(r.Context(), s.DB, "SELECT permission_code FROM role_permissions WHERE role_id=? ORDER BY permission_code", role["id"])
		if e != nil {
			return e
		}
		codes := []string{}
		for _, v := range ps {
			codes = append(codes, str(v["permission_code"]))
		}
		role["permissions"] = codes
	}
	sendPage(w, r, v)
	return nil
}

type roleInput struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"display_name"`
	State       string   `json:"state"`
	Permissions []string `json:"permissions"`
}

func setPermissions(tx *sql.Tx, r *http.Request, record string, codes []string) error {
	if len(codes) > 16 {
		return bad("Too many permissions.")
	}
	for _, code := range codes {
		if !controlAction(code) || code == "platform.admin" {
			return bad("Unsupported Tenant permission.")
		}
	}
	if _, e := tx.ExecContext(r.Context(), "DELETE FROM role_permissions WHERE role_id=?", record); e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if seen[code] {
			continue
		}
		seen[code] = true
		if _, e := tx.ExecContext(r.Context(), "INSERT INTO role_permissions (role_id,kind,permission_code) VALUES (?,'tenant',?)", record, code); e != nil {
			return e
		}
	}
	return nil
}
func (s *Server) createRole(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in roleInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if e := id(in.ID); e != nil {
		return e
	}
	if e := named(in.Code, in.Name); e != nil {
		return e
	}
	if in.State == "" {
		in.State = "active"
	}
	if in.State != "active" && in.State != "disabled" {
		return bad("Invalid role state.")
	}
	t := r.PathValue("tenant")
	e := s.mutate(r.Context(), p, "authorization.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if e := metadataQuota(r.Context(), tx, t, "roles"); e != nil {
			return e
		}
		if _, e := tx.ExecContext(r.Context(), "INSERT INTO roles (id,tenant_id,owner_key,kind,code,display_name,state) VALUES (?,?,?,'tenant',?,?,?)", in.ID, t, "t:"+t, in.Code, in.Name, in.State); e != nil {
			return e
		}
		if e := setPermissions(tx, r, in.ID, in.Permissions); e != nil {
			return e
		}
		return auditChange(r.Context(), tx, t, p.ID, "role.created", "roles", "role", in.ID, r.Header.Get("X-Request-ID"), nil)
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "roles", t, in.ID, 201)
}
func (s *Server) updateRole(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		Name        *string   `json:"display_name"`
		State       *string   `json:"state"`
		Permissions *[]string `json:"permissions"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	t, record := r.PathValue("tenant"), r.PathValue("id")
	e = s.mutate(r.Context(), p, "authorization.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		old, e := securitySnapshot(r.Context(), tx, "roles", t, record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), rev); e != nil {
			return e
		}
		if in.Name != nil && (strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 160) {
			return bad("Invalid name.")
		}
		if in.State != nil && *in.State != "active" && *in.State != "disabled" {
			return bad("Invalid state.")
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE roles SET display_name=COALESCE(?,display_name),state=COALESCE(?,state),revision=revision+1 WHERE id=? AND tenant_id=?", in.Name, in.State, record, t); e != nil {
			return e
		}
		if in.Permissions != nil {
			if e = setPermissions(tx, r, record, *in.Permissions); e != nil {
				return e
			}
		}
		return auditChange(r.Context(), tx, t, p.ID, "role.updated", "roles", "role", record, r.Header.Get("X-Request-ID"), old)
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "roles", t, record, 200)
}
func (s *Server) grants(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.listGrants(w, r, p, false)
}
func (s *Server) platformGrants(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.listGrants(w, r, p, true)
}
func (s *Server) listGrants(w http.ResponseWriter, r *http.Request, p Principal, platform bool) error {
	t := r.PathValue("tenant")
	action := "authorization.read"
	where := "g.tenant_id=?"
	args := []any{t}
	if platform {
		action = "platform.admin"
		where = "g.kind='platform'"
		args = nil
	}
	if e := s.authorize(r.Context(), s.DB, p, action, Resource{Tenant: t}, false); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT g.id,g.user_id,g.group_id,g.space_id,g.app_id,g.role_id,g.permission_code,g.revision,g.created_at,COALESCE(u.display_name,gr.display_name) AS principal_name,COALESCE(ro.display_name,g.permission_code) AS access_name,COALESCE(sp.display_name,a.display_name,'Entire tenant') AS scope_name FROM access_grants g LEFT JOIN users u ON u.id=g.user_id LEFT JOIN groups gr ON gr.id=g.group_id LEFT JOIN roles ro ON ro.id=g.role_id LEFT JOIN spaces sp ON sp.id=g.space_id LEFT JOIN apps a ON a.id=g.app_id WHERE "+where+" ORDER BY g.created_at DESC", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}

type grantInput struct {
	ID         string `json:"id"`
	User       string `json:"user_id"`
	Group      string `json:"group_id"`
	Space      string `json:"space_id"`
	App        string `json:"app_id"`
	Role       string `json:"role_id"`
	Permission string `json:"permission_code"`
}

func (s *Server) createGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.addGrant(w, r, p, false)
}
func (s *Server) createPlatformGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.addGrant(w, r, p, true)
}
func (s *Server) addGrant(w http.ResponseWriter, r *http.Request, p Principal, platform bool) error {
	var in grantInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	for _, v := range []string{in.ID, in.User, in.Group, in.Space, in.App, in.Role} {
		if v != "" {
			if e := id(v); e != nil {
				return e
			}
		}
	}
	if in.ID == "" || (in.User == "") == (in.Group == "") || (in.Role == "") == (in.Permission == "") || (in.Space != "" && in.App != "") {
		return bad("Choose exactly one principal and one role or permission, and at most one scope.")
	}
	t := r.PathValue("tenant")
	kind, action := "tenant", "authorization.manage"
	if platform {
		kind, action = "platform", "platform.admin"
		if in.Group != "" || in.Space != "" || in.App != "" {
			return bad("Platform grants must be direct user grants.")
		}
	}
	e := s.mutate(r.Context(), p, action, Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if !platform {
			if e := metadataQuota(r.Context(), tx, t, "access_grants"); e != nil {
				return e
			}
		}
		if in.Permission != "" && (!controlAction(in.Permission) || (platform != (in.Permission == "platform.admin"))) {
			return bad("Unsupported permission domain.")
		}
		var duplicates int
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM access_grants WHERE tenant_id <=> ? AND kind=? AND user_id <=> ? AND group_id <=> ? AND space_id <=> ? AND app_id <=> ? AND role_id <=> ? AND permission_code <=> ?", nullable(t), kind, nullable(in.User), nullable(in.Group), nullable(in.Space), nullable(in.App), nullable(in.Role), nullable(in.Permission)).Scan(&duplicates); e != nil {
			return e
		}
		if duplicates > 0 {
			return problem(409, "already_exists", "An equivalent access grant already exists.")
		}
		var owner any
		if in.Role != "" {
			role, e := row(r.Context(), tx, "SELECT owner_key FROM roles WHERE id=? AND kind=? AND (tenant_id IS NULL OR tenant_id=?)", in.Role, kind, t)
			if e != nil {
				return e
			}
			owner = role["owner_key"]
			if in.Role == TenantAdminRole && (in.Space != "" || in.App != "") {
				return bad("The Tenant administrator role must apply to the entire Tenant.")
			}
		}
		_, e := tx.ExecContext(r.Context(), "INSERT INTO access_grants (id,tenant_id,kind,user_id,group_id,space_id,app_id,role_id,role_owner_key,permission_code) VALUES (?,?,?,?,?,?,?,?,?,?)", in.ID, nullable(t), kind, nullable(in.User), nullable(in.Group), nullable(in.Space), nullable(in.App), nullable(in.Role), owner, nullable(in.Permission))
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, t, p.ID, "grant.created", "access_grant", in.ID, r.Header.Get("X-Request-ID"), Row{"before": nil, "after": in})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "access_grants", t, in.ID, 201)
}
func (s *Server) deleteGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.removeGrant(w, r, p, false)
}
func (s *Server) deletePlatformGrant(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.removeGrant(w, r, p, true)
}
func (s *Server) removeGrant(w http.ResponseWriter, r *http.Request, p Principal, platform bool) error {
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	t, record := r.PathValue("tenant"), r.PathValue("id")
	action, kind := "authorization.manage", "tenant"
	if platform {
		action, kind = "platform.admin", "platform"
	}
	e = s.mutate(r.Context(), p, action, Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var got int64
		query := "SELECT revision FROM access_grants WHERE id=? AND kind=? AND tenant_id=? FOR UPDATE"
		args := []any{record, kind, t}
		if platform {
			query = "SELECT revision FROM access_grants WHERE id=? AND kind=? AND tenant_id IS NULL FOR UPDATE"
			args = args[:2]
		}
		if e := tx.QueryRowContext(r.Context(), query, args...).Scan(&got); e != nil {
			return missing()
		}
		if e = checkRevision(got, rev); e != nil {
			return e
		}
		before, e := securitySnapshot(r.Context(), tx, "access_grants", t, record)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(r.Context(), "DELETE FROM access_grants WHERE id=?", record); e != nil {
			return e
		}
		if platform {
			e = ensurePlatformAdmin(r.Context(), tx)
		} else {
			e = ensureTenantAdmin(r.Context(), tx, t)
		}
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, t, p.ID, "grant.deleted", "access_grant", record, r.Header.Get("X-Request-ID"), Row{"before": before, "after": nil})
	})
	if e == nil {
		send(w, 200, Row{"ok": true})
	}
	return e
}

func (s *Server) groupMembers(w http.ResponseWriter, r *http.Request, p Principal) error {
	t, g := r.PathValue("tenant"), r.PathValue("id")
	if e := s.authorize(r.Context(), s.DB, p, "authorization.read", Resource{Tenant: t}, false); e != nil {
		return e
	}
	if _, e := row(r.Context(), s.DB, "SELECT id FROM groups WHERE tenant_id=? AND id=?", t, g); e != nil {
		return e
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT gm.user_id,u.display_name FROM group_memberships gm JOIN users u ON u.id=gm.user_id WHERE gm.tenant_id=? AND gm.group_id=? ORDER BY u.display_name,gm.user_id", t, g)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
