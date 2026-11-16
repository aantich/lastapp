package control

import (
	"net/http"
	"strings"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/registry"
)

// Identity pickers expose only the caller and their teams unless the caller
// already has the corresponding Tenant-directory control permission.
func (s *Server) dataReferenceOptions(w http.ResponseWriter, r *http.Request, rt dataRuntime) error {
	f, ok := rt.d.Field(r.URL.Query().Get("field"))
	if !ok || !rt.fieldExposed(f.Key) || (f.Type != "user" && f.Type != "team") {
		return registry.Missing()
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 160 {
		return registry.Bad("Search is too long.")
	}
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query) + "%"
	action := "membership.read"
	if f.Type == "team" {
		action = "team.read"
	}
	sc, e := s.Scopes(r.Context(), s.DB, access.Principal{ID: rt.actor.Actor}, rt.session.TenantID, action)
	if e != nil {
		return e
	}
	var out []Named
	if f.Type == "user" {
		where := " AND u.id=?"
		args := []any{rt.session.TenantID, pattern, query}
		if sc.All {
			where = ""
		} else {
			args = append(args, rt.actor.Actor)
		}
		out, e = registry.ScanAll[Named](r.Context(), s.DB, "SELECT u.display_name,u.id FROM tenant_memberships m JOIN users u ON u.id=m.user_id AND u.state='active' WHERE m.tenant_id=? AND m.state='active' AND (u.display_name LIKE ? OR u.id=?)"+where+" ORDER BY u.display_name,u.id LIMIT 50", args...)
	} else {
		where := " AND EXISTS(SELECT 1 FROM group_memberships gm WHERE gm.tenant_id=g.tenant_id AND gm.group_id=g.id AND gm.user_id=? AND gm.state='active')"
		args := []any{rt.session.TenantID, pattern, query, rt.actor.Actor}
		if sc.All {
			where = ""
			args = args[:3]
		}
		out, e = registry.ScanAll[Named](r.Context(), s.DB, "SELECT g.display_name,g.id FROM groups g WHERE g.tenant_id=? AND g.state='active' AND (g.display_name LIKE ? OR g.id=?)"+where+" ORDER BY g.display_name,g.id LIMIT 50", args...)
	}
	if e == nil {
		send(w, 200, registry.Row{"items": out})
	}
	return e
}
