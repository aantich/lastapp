package control

import (
	"database/sql"
	"net/http"
	"strings"
)

func (s *Server) getSpace(w http.ResponseWriter, r *http.Request, p Principal) error {
	t, space := r.PathValue("tenant"), r.PathValue("space")
	if e := s.authorize(r.Context(), s.DB, p, "space.read", Resource{Tenant: t, Space: space}, false); e != nil {
		return e
	}
	v, e := row(r.Context(), s.DB, "SELECT sp.id,sp.tenant_id,t.display_name AS tenant_name,sp.slug,sp.display_name,sp.state,sp.revision,sp.database_resource_id,sp.database_layout,d.database_name,d.state AS database_state FROM spaces sp JOIN tenants t ON t.id=sp.tenant_id JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.id=?", t, space)
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) updateTenantProfile(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		Name string `json:"display_name"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 160 {
		return bad("Enter a company name (maximum 160 bytes).")
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	t := r.PathValue("tenant")
	e = s.mutate(r.Context(), p, "tenant.manage", Resource{Tenant: t}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		before, e := lockRecord(r.Context(), tx, "tenants", "", t)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(before["revision"]), rev); e != nil {
			return e
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE tenants SET display_name=?,revision=revision+1 WHERE id=?", in.Name, t); e != nil {
			return e
		}
		return audit(r.Context(), tx, t, p.ID, "tenant.profile_updated", "tenant", t, r.Header.Get("X-Request-ID"), Row{"display_name": in.Name})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "tenants", "", t, 200)
}

func (s *Server) tenantOverview(w http.ResponseWriter, r *http.Request, p Principal) error {
	t := r.PathValue("tenant")
	if !p.Platform {
		if e := s.member(r.Context(), s.DB, p, t, false); e != nil {
			return e
		}
	}
	caps, e := s.effectiveScopes(r.Context(), s.DB, p, t)
	if e != nil {
		return e
	}
	profile, e := row(r.Context(), s.DB, "SELECT t.id,t.slug,t.display_name,t.state,t.revision,t.plan_id,CASE WHEN t.default_layout='shared' AND p.allow_shared_database=1 THEN 'shared' WHEN t.default_layout='dedicated' AND p.allow_dedicated_database=1 THEN 'dedicated' ELSE p.default_layout END AS default_layout,p.display_name AS plan_name,p.max_spaces,p.max_apps,p.max_databases,p.allow_shared_database,p.allow_dedicated_database,p.allow_managed_database,p.allow_registered_database,p.allow_dedicated_compute FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", t)
	if e != nil {
		return e
	}
	out := Row{"tenant": profile}
	for _, spec := range []struct{ key, action, query, space, app string }{
		{"spaces", "space.read", "FROM spaces sp WHERE sp.tenant_id=? AND sp.state<>'archived'", "sp.id", ""},
		{"apps", "app.read", "FROM apps a WHERE a.tenant_id=? AND a.state<>'archived'", "a.space_id", "a.id"},
		{"microapps", "microapp.read", "FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.tenant_id=? AND m.state<>'archived'", "a.space_id", "m.app_id"},
	} {
		filter, args := scopeSQL(caps[spec.action], spec.space, spec.app)
		args = append([]any{t}, args...)
		var count int
		if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) "+spec.query+" AND "+filter, args...).Scan(&count); e != nil {
			return e
		}
		out[spec.key] = count
	}
	filter, args := scopeSQL(caps["app.read"], "a.space_id", "a.id")
	args = append([]any{t}, args...)
	recent, e := rows(r.Context(), s.DB, "SELECT a.id,a.space_id,a.display_name,a.state,sp.display_name AS space_name FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND a.state<>'archived' AND "+filter+" ORDER BY a.created_at DESC,a.id LIMIT 6", args...)
	if e != nil {
		return e
	}
	out["recent_apps"] = recent
	if caps["database.read"].All {
		var count int
		if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM database_resources WHERE tenant_id=?", t).Scan(&count); e != nil {
			return e
		}
		out["databases"] = count
		warnings, e := rows(r.Context(), s.DB, "SELECT id,database_name,state,last_error_message FROM database_resources WHERE tenant_id=? AND state IN ('pending','provisioning','failed') ORDER BY created_at DESC LIMIT 6", t)
		if e != nil {
			return e
		}
		out["attention"] = warnings
	}
	send(w, 200, out)
	return nil
}
