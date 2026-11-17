package dataapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

func (s *Runtime) dataReferenceOptions(w http.ResponseWriter, r *http.Request, rt dataRuntime) error {
	f, ok := rt.d.Field(r.URL.Query().Get("field"))
	if !ok || !rt.fieldExposed(f.Key) || (f.Type != "user" && f.Type != "team") {
		return registry.Missing()
	}
	out, e := s.identityOptions(r.Context(), rt, f.Type, r.URL.Query().Get("q"))
	if e == nil {
		httpapi.Send(w, 200, registry.Row{"items": out})
	}
	return e
}

// identityOptions is the one identity-directory rule of the data API, used by
// reference-field pickers and record-share recipient search: active Tenant
// users (kind "user") or teams ("team") matching query. Without the
// corresponding Tenant-directory control permission (membership.read or
// team.read, checked live) the caller sees only themselves and their own
// teams, so neither picker discovers other members' names or emails.
func (s *Runtime) identityOptions(ctx context.Context, rt dataRuntime, kind, query string) ([]roles.Named, error) {
	query = strings.TrimSpace(query)
	if len(query) > 160 {
		return nil, registry.Bad("Search is too long.")
	}
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query) + "%"
	action := "membership.read"
	if kind == "team" {
		action = "team.read"
	}
	sc, e := s.Access.Scopes(ctx, s.DB, access.Principal{ID: rt.actor.Actor}, rt.session.TenantID, action)
	if e != nil {
		return nil, e
	}
	if kind == "user" {
		where := " AND u.id=?"
		args := []any{rt.session.TenantID, pattern, pattern, query, rt.actor.Actor}
		if sc.All {
			where = ""
			args = args[:4]
		}
		return registry.ScanAll[roles.Named](ctx, s.DB, "SELECT u.display_name,u.id FROM tenant_memberships m JOIN users u ON u.id=m.user_id AND u.state='active' WHERE m.tenant_id=? AND m.state='active' AND (u.display_name LIKE ? OR u.email LIKE ? OR u.id=?)"+where+" ORDER BY u.display_name,u.id LIMIT 50", args...)
	}
	where := " AND EXISTS(SELECT 1 FROM group_memberships gm WHERE gm.tenant_id=g.tenant_id AND gm.group_id=g.id AND gm.user_id=? AND gm.state='active')"
	args := []any{rt.session.TenantID, pattern, query, rt.actor.Actor}
	if sc.All {
		where = ""
		args = args[:3]
	}
	return registry.ScanAll[roles.Named](ctx, s.DB, "SELECT g.display_name,g.id FROM groups g WHERE g.tenant_id=? AND g.state='active' AND (g.display_name LIKE ? OR g.id=?)"+where+" ORDER BY g.display_name,g.id LIMIT 50", args...)
}
