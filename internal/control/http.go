package control

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/authz"
	admin "github.com/aantich/lastapp/web/admin"
)

type endpoint func(http.ResponseWriter, *http.Request, Principal) error

func (s *Server) api(pattern string, fn endpoint) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		ctx = context.WithValue(ctx, pageContextKey{}, &pageInfo{})
		r = r.WithContext(ctx)
		p, e := s.principal(ctx, r)
		if e == nil {
			e = s.limitRequest(w, p, r)
		}
		if e == nil && p.PasswordChangeRequired && r.URL.Path != "/v1/me" && r.URL.Path != "/auth/password" && r.URL.Path != "/auth/logout" {
			e = problem(403, "password_change_required", "Change your initial password before using this account.")
		}
		if e == nil && r.Method != "GET" && r.Method != "HEAD" {
			e = s.csrf(ctx, r)
		}
		if e == nil {
			e = fn(w, r, p)
		}
		if e != nil {
			writeError(w, r, e)
		}
	})
}
func (s *Server) Handler() http.Handler {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { send(w, 200, Row{"status": "ok"}) })
	s.mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if s.DB.PingContext(ctx) != nil {
			send(w, 503, Row{"status": "unavailable"})
			return
		}
		send(w, 200, Row{"status": "ready"})
	})
	s.mux.HandleFunc("GET /v1/config", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, Row{"name": "Lastapp", "version": "0.1.0-dev", "development": s.Config.Development, "oidc_enabled": s.Config.OIDCIssuer != "", "login_url": "/auth/login", "csrf_cookie": s.cookieName("lastapp_csrf"), "development_policy": developmentPolicy(), "password_policy": publicPasswordPolicy(), "role_catalog": authz.Roles()})
	})
	s.mux.HandleFunc("GET /auth/login", s.login)
	s.mux.HandleFunc("GET /auth/callback", s.callback)
	s.mux.HandleFunc("POST /auth/login/password", s.passwordLogin)
	s.api("POST /auth/password", s.changePassword)
	s.api("POST /auth/logout", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		c, _ := r.Cookie(s.cookieName("lastapp_session"))
		_, e := s.DB.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", hash(c.Value))
		if e != nil {
			return e
		}
		s.cookie(w, "lastapp_session", "", -1, true)
		s.cookie(w, "lastapp_csrf", "", -1, false)
		send(w, 200, Row{"ok": true})
		return nil
	})
	s.api("GET /v1/me", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		ts, e := pageRows(r.Context(), s.DB, r, "SELECT t.id,t.slug,t.display_name,t.state FROM tenant_memberships m JOIN tenants t ON t.id=m.tenant_id WHERE m.user_id=? AND m.state='active' ORDER BY t.display_name", p.ID)
		if e != nil {
			return e
		}
		send(w, 200, Row{"id": p.ID, "display_name": p.Name, "email": p.Email, "platform_admin": p.Platform, "auth_method": p.AuthMethod, "password_change_required": p.PasswordChangeRequired, "tenants": ts, "pagination": r.Context().Value(pageContextKey{})})
		return nil
	})
	s.api("GET /v1/my-tenants", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		v, e := pageRows(r.Context(), s.DB, r, "SELECT t.id,t.slug,t.display_name,t.state FROM tenant_memberships m JOIN tenants t ON t.id=m.tenant_id WHERE m.user_id=? AND m.state='active' ORDER BY t.display_name", p.ID)
		if e == nil {
			sendPage(w, r, v)
		}
		return e
	})
	s.entityRoutes()
	s.agentRoutes()
	s.workspaceRoutes()
	s.releaseRoutes()
	s.api("GET /v1/roles/catalog", s.roleCatalog)
	s.api("GET /v1/tenants/{tenant}/my-role-assignments", s.myRoles)
	s.api("GET /v1/tenants/{tenant}/capabilities", s.capabilities)
	s.api("GET /admin/v1/overview", s.overview)
	s.api("GET /admin/v1/users", s.users)
	s.api("POST /admin/v1/users", s.createLocalUser)
	s.api("PATCH /admin/v1/users/{id}", s.updateUser)
	s.api("GET /admin/v1/plans", s.plans)
	s.api("POST /admin/v1/plans", s.createPlan)
	s.api("PATCH /admin/v1/plans/{id}", s.updatePlan)
	s.api("GET /admin/v1/tenants", s.tenants)
	s.api("POST /admin/v1/tenants", s.createTenant)
	s.api("PATCH /admin/v1/tenants/{id}", s.updateTenant)
	s.api("GET /v1/tenants/{tenant}", s.getTenant)
	s.api("PATCH /v1/tenants/{tenant}", s.updateTenantProfile)
	s.api("GET /v1/tenants/{tenant}/overview", s.tenantOverview)
	s.api("GET /admin/v1/tenants/{tenant}/administrators", s.tenantAdministrators)
	s.api("POST /admin/v1/tenants/{tenant}/administrators", s.addTenantAdministrator)
	s.api("GET /admin/v1/database-targets", s.targets)
	s.api("POST /admin/v1/database-targets", s.createTarget)
	s.api("PATCH /admin/v1/database-targets/{id}", s.updateTarget)
	s.api("GET /v1/tenants/{tenant}/database-targets", s.availableTargets)
	s.api("GET /v1/tenants/{tenant}/database-resources", s.databases)
	s.api("POST /v1/tenants/{tenant}/database-resources", s.createDatabase)
	s.api("GET /v1/tenants/{tenant}/database-resources/{id}", s.getDatabase)
	s.api("POST /v1/tenants/{tenant}/database-resources/{id}/retry", s.retryDatabase)
	s.api("PATCH /v1/tenants/{tenant}/database-resources/{id}", s.updateDatabase)
	s.api("GET /v1/tenants/{tenant}/spaces", s.spaces)
	s.api("GET /v1/tenants/{tenant}/spaces/{space}", s.getSpace)
	s.api("POST /v1/tenants/{tenant}/spaces", s.createSpace)
	s.api("PATCH /v1/tenants/{tenant}/spaces/{id}", s.updateSpace)
	s.api("GET /v1/tenants/{tenant}/apps", s.apps)
	s.api("GET /v1/tenants/{tenant}/apps/{app}", s.getApp)
	s.api("POST /v1/tenants/{tenant}/spaces/{space}/apps", s.createApp)
	s.api("PATCH /v1/tenants/{tenant}/apps/{id}", s.updateApp)
	s.api("GET /v1/tenants/{tenant}/microapps", s.microapps)
	s.api("POST /v1/tenants/{tenant}/apps/{app}/microapps", s.createMicroapp)
	s.api("PATCH /v1/tenants/{tenant}/microapps/{id}", s.updateMicroapp)
	s.api("GET /v1/tenants/{tenant}/memberships", s.memberships)
	s.api("POST /v1/tenants/{tenant}/memberships", s.addMembership)
	s.api("PATCH /v1/tenants/{tenant}/memberships/{user}", s.updateMembership)
	s.api("GET /v1/tenants/{tenant}/teams", s.groups)
	s.api("GET /v1/tenants/{tenant}/teams/{id}", s.getGroup)
	s.api("POST /v1/tenants/{tenant}/teams", s.createGroup)
	s.api("PATCH /v1/tenants/{tenant}/teams/{id}", s.updateGroup)
	s.api("GET /v1/tenants/{tenant}/teams/{id}/members", s.groupMembers)
	s.api("POST /v1/tenants/{tenant}/teams/{id}/members", s.addGroupMember)
	s.api("DELETE /v1/tenants/{tenant}/teams/{id}/members/{user}", s.removeGroupMember)
	for _, path := range []string{"/v1/tenants/{tenant}", "/v1/tenants/{tenant}/teams/{team}", "/v1/tenants/{tenant}/spaces/{space}", "/v1/tenants/{tenant}/apps/{app}"} {
		s.api("GET "+path+"/recipients", s.recipientOptions)
		s.api("GET "+path+"/roles", s.roles)
		if strings.Contains(path, "{space}") || strings.Contains(path, "{app}") {
			s.api("POST "+path+"/roles", s.createRole)
			s.api("PATCH "+path+"/roles/{id}", s.updateRole)
		}
		s.api("GET "+path+"/role-assignments", s.grants)
		s.api("POST "+path+"/role-assignments", s.createGrant)
		s.api("DELETE "+path+"/role-assignments/{id}", s.deleteGrant)
	}
	s.api("GET /admin/v1/role-assignments", s.grants)
	s.api("POST /admin/v1/role-assignments", s.createGrant)
	s.api("DELETE /admin/v1/role-assignments/{id}", s.deleteGrant)
	s.api("PATCH /v1/tenants/{tenant}/teams/{id}/members/{user}", s.updateGroupMember)
	s.api("GET /v1/tenants/{tenant}/audit-events", s.audits)
	s.api("GET /admin/v1/audit-events", s.audits)
	assets, _ := fs.Sub(admin.Files, "dist")
	files := http.FileServer(http.FS(assets))
	s.mux.HandleFunc("GET /console/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/console/")
		if name == "/console" || name == "." {
			name = "index.html"
		}
		if _, e := fs.Stat(assets, name); e != nil {
			if strings.Contains(path.Base(name), ".") {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}
		if name == "index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			b, err := fs.ReadFile(assets, name)
			if err != nil {
				http.Error(w, "Build the admin UI before compiling Lastapp (make build).", 503)
				return
			}
			w.Write(b)
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/" + name
		files.ServeHTTP(w, clone)
	})
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/console/", 302) })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.serveAppHost(w, r) {
			return
		}
		rid := newID()
		r.Header.Set("X-Request-ID", rid)
		w.Header().Set("X-Request-ID", rid)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		s.mux.ServeHTTP(w, r)
	})
}
func (s *Server) capabilities(w http.ResponseWriter, r *http.Request, p Principal) error {
	tenant := r.PathValue("tenant")
	if !p.Platform {
		if e := s.member(r.Context(), s.DB, p, tenant, false); e != nil {
			return e
		}
	}
	scopes, e := s.effectiveScopes(r.Context(), s.DB, p, tenant)
	if e != nil {
		return e
	}
	out := Row{}
	for code, sc := range scopes {
		out[code] = Row{"all": sc.All, "available": sc.All || len(sc.Spaces)+len(sc.Apps)+len(sc.Teams) > 0, "spaces": sc.Spaces, "apps": sc.Apps, "teams": sc.Teams}
	}
	send(w, 200, Row{"permissions": out})
	return nil
}
func (s *Server) overview(w http.ResponseWriter, r *http.Request, p Principal) error {
	if e := s.authorize(r.Context(), s.DB, p, "platform.admin", Resource{}, false); e != nil {
		return e
	}
	out := Row{}
	for _, table := range []string{"tenants", "spaces", "apps", "database_resources", "users"} {
		var n int
		if e := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+table).Scan(&n); e != nil {
			return e
		}
		out[table] = n
	}
	status, e := rows(r.Context(), s.DB, "SELECT state,COUNT(*) AS count FROM database_resources GROUP BY state")
	if e != nil {
		return e
	}
	out["database_states"] = status
	recent, e := rows(r.Context(), s.DB, "SELECT d.id,d.tenant_id,d.database_name,d.state,d.last_error_message,t.display_name AS tenant_name FROM database_resources d JOIN tenants t ON t.id=d.tenant_id ORDER BY d.created_at DESC LIMIT 6")
	if e != nil {
		return e
	}
	out["recent_databases"] = recent
	send(w, 200, out)
	return e
}
func (s *Server) audits(w http.ResponseWriter, r *http.Request, p Principal) error {
	tenant := r.PathValue("tenant")
	action := "audit.read"
	if tenant == "" {
		action = "platform.admin"
	}
	if e := s.authorize(r.Context(), s.DB, p, action, Resource{Tenant: tenant}, false); e != nil {
		return e
	}
	args := []any{}
	where := "1=1"
	if tenant != "" {
		where = "a.tenant_id=?"
		args = append(args, tenant)
	}
	v, e := pageRows(r.Context(), s.DB, r, "SELECT a.id,a.tenant_id,a.action,a.resource_type,a.resource_id,a.request_id,a.details_json,a.created_at,COALESCE(u.display_name,'System worker') AS actor_name FROM audit_events a LEFT JOIN users u ON u.id=a.actor_user_id WHERE "+where+" ORDER BY a.id DESC", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func result(w http.ResponseWriter, r *http.Request, s *Server, table, tenant, record string, status int) error {
	query := "SELECT id,revision FROM " + table + " WHERE id=?"
	args := []any{record}
	if tenant != "" {
		query += " AND tenant_id=?"
		args = append(args, tenant)
	}
	v, e := row(r.Context(), s.DB, query, args...)
	if e == nil {
		w.Header().Set("ETag", str(v["revision"]))
		send(w, status, v)
	}
	return e
}
func lockRecord(ctx context.Context, tx *sql.Tx, table, tenant, record string) (Row, error) {
	query := "SELECT id,revision,state FROM " + table + " WHERE id=?"
	args := []any{record}
	if tenant != "" {
		query += " AND tenant_id=?"
		args = append(args, tenant)
	}
	return row(ctx, tx, query+" FOR UPDATE", args...)
}
