package control

import (
	"context"
	"database/sql"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/google/uuid"
	"net/http"
)

// Called only by trusted bootstrap against a fresh development registry.
func seedControlCatalog(ctx context.Context, db *sql.DB) error {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, code := range authz.Actions() {
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("lastapp:control:"+code)).String()
		if _, e = tx.ExecContext(ctx, "INSERT INTO permissions (id,code,domain,description) VALUES (?,?,'control',?) ON DUPLICATE KEY UPDATE description=VALUES(description)", id, code, code); e != nil {
			return e
		}
	}
	for _, r := range authz.Roles() {
		if _, e = tx.ExecContext(ctx, "INSERT INTO roles (id,owner_key,scope_kind,domain,code,display_name) VALUES (?,'builtin',?,'control',?,?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name)", r.ID, r.Scope, r.Code, r.Name); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM role_permissions WHERE role_id=?", r.ID); e != nil {
			return e
		}
		for _, code := range r.Permissions {
			if _, e = tx.ExecContext(ctx, "INSERT INTO role_permissions(role_id,permission_id,domain) SELECT ?,id,'control' FROM permissions WHERE domain='control' AND code=?", r.ID, code); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
func (s *Server) roleCatalog(w http.ResponseWriter, r *http.Request, p Principal) error {
	kind := authz.Kind(r.URL.Query().Get("scope_kind"))
	if kind != "" && !authz.ValidKind(kind) {
		return bad("Unsupported scope category.")
	}
	out := []authz.Role{}
	for _, role := range authz.Roles() {
		if kind == "" || role.Scope == kind {
			out = append(out, role)
		}
	}
	send(w, 200, Row{"items": out})
	return nil
}
func (s *Server) myRoles(w http.ResponseWriter, r *http.Request, p Principal) error {
	tx, e := s.DB.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	t := r.PathValue("tenant")
	if e = s.member(r.Context(), tx, p, t, false); e != nil {
		return e
	}
	roles, e := pageRows(r.Context(), tx, r, effectiveAssignmentSQL, p.ID, t)
	if e != nil {
		return e
	}
	send(w, 200, Row{"tenant_id": t, "user_id": p.ID, "items": roles, "business_acl_enforcement": true, "pagination": r.Context().Value(pageContextKey{})})
	return nil
}
