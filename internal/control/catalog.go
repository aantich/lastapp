package control

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/google/uuid"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
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

func (s *Server) roleCatalog(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	kind := authz.Kind(r.URL.Query().Get("scope_kind"))
	if kind != "" && !authz.ValidKind(kind) {
		return registry.Bad("Unsupported scope category.")
	}
	out := []authz.Role{}
	for _, role := range authz.Roles() {
		if kind == "" || role.Scope == kind {
			out = append(out, role)
		}
	}
	send(w, 200, registry.Row{"items": out})
	return nil
}

func (s *Server) myRoles(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	tx, e := s.DB.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	t := r.PathValue("tenant")
	if e = s.Member(r.Context(), tx, p, t, false); e != nil {
		return e
	}
	roles, e := op.PageScan[access.EffectiveAssignment](r.Context(), tx, pageRequest(r.URL.Query()), op.CollectionFor("my-role-assignments"), access.EffectiveAssignmentSQL, p.ID, t)
	if e != nil {
		return e
	}
	send(w, 200, registry.Row{"tenant_id": t, "user_id": p.ID, "items": roles.Items, "business_acl_enforcement": true, "pagination": roles.Pagination})
	return nil
}
