package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

// tenantProfile is the overview's Tenant with its effective default layout
// and its plan's limits and permissions.
type tenantProfile struct {
	AllowDedicatedCompute  registry.Bit `json:"allow_dedicated_compute"`
	AllowDedicatedDatabase registry.Bit `json:"allow_dedicated_database"`
	AllowManaged           registry.Bit `json:"allow_managed_database"`
	AllowRegistered        registry.Bit `json:"allow_registered_database"`
	AllowSharedDatabase    registry.Bit `json:"allow_shared_database"`
	DefaultLayout          string       `json:"default_layout"`
	Name                   string       `json:"display_name"`
	ID                     string       `json:"id"`
	MaxApps                *int64       `json:"max_apps"`
	MaxDatabases           *int64       `json:"max_databases"`
	MaxSpaces              *int64       `json:"max_spaces"`
	PlanID                 string       `json:"plan_id"`
	PlanName               string       `json:"plan_name"`
	Revision               int64        `json:"revision"`
	Slug                   string       `json:"slug"`
	State                  string       `json:"state"`
}

func (v *tenantProfile) ScanFields() []any {
	return []any{&v.AllowDedicatedCompute, &v.AllowDedicatedDatabase, &v.AllowManaged, &v.AllowRegistered, &v.AllowSharedDatabase, &v.DefaultLayout, &v.Name, &v.ID, &v.MaxApps, &v.MaxDatabases, &v.MaxSpaces, &v.PlanID, &v.PlanName, &v.Revision, &v.Slug, &v.State}
}

// recentApp is one of the overview's most recent production Apps.
type recentApp struct {
	Name      string `json:"display_name"`
	ID        string `json:"id"`
	SpaceID   string `json:"space_id"`
	SpaceName string `json:"space_name"`
	State     string `json:"state"`
}

func (v *recentApp) ScanFields() []any {
	return []any{&v.Name, &v.ID, &v.SpaceID, &v.SpaceName, &v.State}
}

// databaseAttention is a database that is not yet ready or has failed.
type databaseAttention struct {
	DatabaseName string  `json:"database_name"`
	ID           string  `json:"id"`
	LastError    *string `json:"last_error_message"`
	State        string  `json:"state"`
}

func (v *databaseAttention) ScanFields() []any {
	return []any{&v.DatabaseName, &v.ID, &v.LastError, &v.State}
}

func (s *Server) tenantOverview(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	t := r.PathValue("tenant")
	if !p.Platform {
		if e := s.Member(r.Context(), s.DB, p, t, false); e != nil {
			return e
		}
	}
	caps, e := s.EffectiveScopes(r.Context(), s.DB, p, t)
	if e != nil {
		return e
	}
	profile, e := registry.ScanOne[tenantProfile](r.Context(), s.DB, "SELECT p.allow_dedicated_compute,p.allow_dedicated_database,p.allow_managed_database,p.allow_registered_database,p.allow_shared_database,CASE WHEN t.default_layout='shared' AND p.allow_shared_database=1 THEN 'shared' WHEN t.default_layout='dedicated' AND p.allow_dedicated_database=1 THEN 'dedicated' ELSE p.default_layout END AS default_layout,t.display_name,t.id,p.max_apps,p.max_databases,p.max_spaces,t.plan_id,p.display_name AS plan_name,t.revision,t.slug,t.state FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", t)
	if e != nil {
		return e
	}
	out := registry.Row{"tenant": profile}
	for _, spec := range []struct{ key, action, query, space, app string }{
		{"spaces", "space.read", "FROM spaces sp WHERE sp.tenant_id=? AND sp.state<>'archived' AND sp.workspace_id IS NULL", "sp.id", ""},
		{"apps", "app.read", "FROM apps a WHERE a.tenant_id=? AND a.state<>'archived' AND NOT EXISTS(SELECT 1 FROM spaces dsp WHERE dsp.id=a.space_id AND dsp.workspace_id IS NOT NULL)", "a.space_id", "a.id"},
		{"microapps", "microapp.read", "FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.tenant_id=? AND m.state<>'archived'", "a.space_id", "m.app_id"},
	} {
		filter, args := access.ScopeSQL(caps[spec.action], spec.space, spec.app)
		args = append([]any{t}, args...)
		var count int
		if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) "+spec.query+" AND "+filter, args...).Scan(&count); e != nil {
			return e
		}
		out[spec.key] = count
	}
	filter, args := access.ScopeSQL(caps["app.read"], "a.space_id", "a.id")
	args = append([]any{t}, args...)
	recent, e := registry.ScanAll[recentApp](r.Context(), s.DB, "SELECT a.display_name,a.id,a.space_id,sp.display_name AS space_name,a.state FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND a.state<>'archived' AND NOT EXISTS(SELECT 1 FROM spaces dsp WHERE dsp.id=a.space_id AND dsp.workspace_id IS NOT NULL) AND "+filter+" ORDER BY a.created_at DESC,a.id LIMIT 6", args...)
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
		warnings, e := registry.ScanAll[databaseAttention](r.Context(), s.DB, "SELECT database_name,id,last_error_message,state FROM database_resources WHERE tenant_id=? AND state IN ('pending','provisioning','failed') ORDER BY created_at DESC LIMIT 6", t)
		if e != nil {
			return e
		}
		out["attention"] = warnings
	}
	httpapi.Send(w, 200, out)
	return nil
}
