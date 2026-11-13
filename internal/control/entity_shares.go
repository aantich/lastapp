package control

import (
	"database/sql"
	"encoding/base64"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/entity"
	"net/http"
	"slices"
	"strings"
)

func base64Encode(b []byte) string          { return base64.RawURLEncoding.EncodeToString(b) }
func base64Decode(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
func (s *Server) dataCapabilities(w http.ResponseWriter, r *http.Request, rt dataRuntime) error {
	var in struct {
		IDs []string `json:"ids"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if len(in.IDs) > entity.Limits.Bulk {
		return bad("Too many records.")
	}
	db, release, e := s.runtimeDB(r.Context(), str(rt.session["database_resource_id"]))
	if e != nil {
		return e
	}
	defer release()
	tx, e := db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = rt.checkGate(r.Context(), tx, false); e != nil {
		return e
	}
	out := []any{}
	for _, id := range in.IDs {
		result := Row{"id": id, "operations": []string{}}
		raw, e := row(r.Context(), tx, "SELECT * FROM "+data.Table(rt.entityID)+" WHERE tenant_id=? AND space_id=? AND id=?", rt.session["tenant_id"], rt.session["space_id"], id)
		if e == nil {
			c := rt.actor
			c.Record = businessRecord(rt.d, raw)
			c.Old = c.Record
			shared, e := rt.shared(r.Context(), tx, id)
			if e != nil {
				return e
			}
			ops := []string{}
			fieldMap := Row{}
			for _, op := range entity.Operations {
				if op != "create" && rt.d.Allowed(rt.entityID, op, nil, c, rt.access, shared) {
					ops = append(ops, op)
				}
			}
			for _, op := range []string{"read", "update"} {
				keys := []string{}
				for _, f := range rt.d.Fields {
					if rt.d.Allowed(rt.entityID, op, []string{f.Key}, c, rt.access, shared) {
						keys = append(keys, f.Key)
					}
				}
				fieldMap[op] = keys
			}
			result["fields"] = fieldMap
			result["operations"] = ops
		} else {
			if ae, ok := e.(*APIError); !ok || ae.Status != 404 {
				return e
			}
		}
		out = append(out, result)
	}
	send(w, 200, Row{"items": out})
	return nil
}
func (s *Server) dataShares(w http.ResponseWriter, r *http.Request, rt dataRuntime, record string) error {
	if !rt.d.Sharing {
		return missing()
	}
	if r.Method != "GET" && r.Method != "POST" && r.Method != "DELETE" {
		return missing()
	}
	var in struct {
		Permission string `json:"permission_id"`
		Recipient  string `json:"recipient_id"`
		Kind       string `json:"recipient_kind"`
	}
	if r.Method != "GET" {
		if e := decode(w, r, &in); e != nil {
			return e
		}
		if id(in.Recipient) != nil || !slices.Contains([]string{"user", "team"}, in.Kind) {
			return bad("Choose a recipient user or team.")
		}
	}
	db, release, e := s.runtimeDB(r.Context(), str(rt.session["database_resource_id"]))
	if e != nil {
		return e
	}
	defer release()
	tx, e := db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = rt.checkGate(r.Context(), tx, true); e != nil {
		return e
	}
	raw, e := row(r.Context(), tx, "SELECT * FROM "+data.Table(rt.entityID)+" WHERE tenant_id=? AND space_id=? AND id=? FOR UPDATE", rt.session["tenant_id"], rt.session["space_id"], record)
	if e != nil {
		return e
	}
	c := rt.actor
	c.Record = businessRecord(rt.d, raw)
	c.Old = c.Record
	if !rt.d.Allowed(rt.entityID, "acl.manage", nil, c, rt.access, nil) {
		return missing()
	}
	shared, e := rt.shared(r.Context(), tx, record)
	if e != nil {
		return e
	}
	if r.Method == "GET" {
		if r.URL.Query().Get("view") == "options" {
			return s.shareOptions(w, r, rt, c, shared)
		}
		vs, e := rows(r.Context(), tx, "SELECT permission_id,security_revision,recipient_id,recipient_kind,grantor_id,created_at FROM _lastapp_record_acl WHERE entity_id=? AND record_id=? ORDER BY created_at LIMIT 201", rt.entityID, record)
		if e != nil {
			return e
		}
		for _, grant := range vs {
			grant["permission_current"] = integer(grant["security_revision"]) == rt.permissionRevisions[str(grant["permission_id"])]
		}
		send(w, 200, Row{"items": vs})
		return nil
	}
	if r.Method == "DELETE" { // Revocation must remain possible after a base permission is disabled/removed.
		if _, e = tx.ExecContext(r.Context(), "DELETE FROM _lastapp_record_acl WHERE entity_id=? AND record_id=? AND permission_id=? AND recipient_id=? AND recipient_kind=?", rt.entityID, record, in.Permission, in.Recipient, in.Kind); e != nil {
			return e
		}
		if e = businessAudit(r.Context(), tx, rt, record, "share.delete", integer(raw["revision"]), nil, r.Header.Get("X-Request-ID")); e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		send(w, 200, Row{"ok": true})
		return nil
	}
	var permission *entity.Permission
	for i := range rt.d.Permissions {
		p := &rt.d.Permissions[i]
		if entity.PermissionID(rt.entityID, p.Key) == in.Permission {
			permission = p
		}
	}
	if permission == nil || len(permission.Roles) == 0 || permission.Operation == "acl.manage" || permission.Operation == "create" {
		return bad("Permission cannot be shared.")
	}
	if !rt.canDelegate(*permission, c, shared) {
		return problem(403, "forbidden", "You must hold the exact permission and its delegation authority.")
	}

	var n int
	query := "SELECT COUNT(*) FROM groups WHERE id=? AND tenant_id=? AND state='active'"
	if in.Kind == "user" {
		query = "SELECT COUNT(*) FROM tenant_memberships m JOIN users u ON u.id=m.user_id AND u.state='active' WHERE m.user_id=? AND m.tenant_id=? AND m.state='active'"
	}
	if e = s.DB.QueryRowContext(r.Context(), query, in.Recipient, rt.session["tenant_id"]).Scan(&n); e != nil {
		return e
	}
	if n != 1 {
		return bad("Unavailable recipient.")
	}
	if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM _lastapp_record_acl WHERE entity_id=? AND record_id=?", rt.entityID, record).Scan(&n); e != nil {
		return e
	}
	if n >= 200 {
		return conflict("Record share limit reached.")
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO _lastapp_record_acl(tenant_id,space_id,entity_id,record_id,permission_id,security_revision,recipient_id,recipient_kind,grantor_id) VALUES(?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE security_revision=VALUES(security_revision),grantor_id=VALUES(grantor_id)", rt.session["tenant_id"], rt.session["space_id"], rt.entityID, record, in.Permission, rt.permissionRevisions[in.Permission], in.Recipient, in.Kind, c.Actor)
	if e != nil {
		return e
	}
	e = businessAudit(r.Context(), tx, rt, record, "share.post", integer(raw["revision"]), nil, r.Header.Get("X-Request-ID"))
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	send(w, 200, Row{"ok": true})
	return nil
}

func (rt dataRuntime) canDelegate(permission entity.Permission, c entity.Context, shared map[string]bool) bool {
	if len(permission.Roles) == 0 || permission.Operation == "create" || permission.Operation == "acl.manage" {
		return false
	}
	base := rt.d
	base.Permissions = []entity.Permission{permission}
	if !base.Allowed(rt.entityID, permission.Operation, permission.Fields, c, rt.access, shared) {
		return false
	}
	for _, manage := range rt.d.Permissions {
		if manage.Operation == "acl.manage" && slices.Contains(manage.Delegable, permission.Key) {
			one := rt.d
			one.Permissions = []entity.Permission{manage}
			if one.Allowed(rt.entityID, "acl.manage", nil, c, rt.access, nil) {
				return true
			}
		}
	}
	return false
}
func (s *Server) shareOptions(w http.ResponseWriter, r *http.Request, rt dataRuntime, c entity.Context, shared map[string]bool) error {
	permissions := []Row{}
	for _, p := range rt.d.Permissions {
		if rt.canDelegate(p, c, shared) {
			permissions = append(permissions, Row{"id": entity.PermissionID(rt.entityID, p.Key), "label": p.Label, "operation": p.Operation, "fields": p.Fields})
		}
	}
	kind := r.URL.Query().Get("recipient_kind")
	if kind == "" {
		kind = "user"
	}
	search := r.URL.Query().Get("q")
	if len(search) > 160 {
		return bad("Search is too long.")
	}
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(search) + "%"
	var recipients []Row
	var e error
	if kind == "user" {
		recipients, e = rows(r.Context(), s.DB, "SELECT u.id,u.display_name FROM tenant_memberships m JOIN users u ON u.id=m.user_id AND u.state='active' WHERE m.tenant_id=? AND m.state='active' AND (u.display_name LIKE ? OR u.email LIKE ? OR u.id=?) ORDER BY u.display_name,u.id LIMIT 50", rt.session["tenant_id"], pattern, pattern, search)
	} else if kind == "team" {
		recipients, e = rows(r.Context(), s.DB, "SELECT id,display_name FROM groups WHERE tenant_id=? AND state='active' AND (display_name LIKE ? OR id=?) ORDER BY display_name,id LIMIT 50", rt.session["tenant_id"], pattern, search)
	} else {
		return bad("Choose user or team.")
	}
	if e != nil {
		return e
	}
	send(w, 200, Row{"permissions": permissions, "recipients": recipients})
	return nil
}
