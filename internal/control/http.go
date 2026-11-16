package control

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/authz"
	admin "github.com/aantich/lastapp/web/admin"
)

type endpoint func(http.ResponseWriter, *http.Request, Principal) error

// requestTimeout bounds the database work of one console API or MCP request.
var requestTimeout = 25 * time.Second

type requestBaseKey struct{}

// withRequestDeadline starts a request's deadline and remembers the request
// context without it, so renewDeadline can give a later phase a fresh one.
func withRequestDeadline(base context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(base, requestTimeout)
	return context.WithValue(ctx, requestBaseKey{}, base), cancel
}

// renewDeadline gives the database phase after a PBKDF2 derivation its own
// request deadline. The derivation cannot observe a context; its cost is
// bounded by passwordSlots and maxPasswordInputBytes. A deadline spanning it
// only turned a completed derivation into a 500 when the CPU was slow (for
// example under the race detector on a loaded test machine). The renewed
// context keeps ctx's values and the request's own cancellation.
func renewDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	base, ok := ctx.Value(requestBaseKey{}).(context.Context)
	if !ok {
		return ctx, func() {}
	}
	next, cancel := context.WithTimeout(context.WithoutCancel(ctx), requestTimeout)
	stop := context.AfterFunc(base, cancel)
	return next, func() { stop(); cancel() }
}

func (s *Server) api(pattern string, fn endpoint) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := withRequestDeadline(r.Context())
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
		ts, e := s.myTenants(r, p)
		if e != nil {
			return e
		}
		send(w, 200, Row{"id": p.ID, "display_name": p.Name, "email": p.Email, "platform_admin": p.Platform, "auth_method": p.AuthMethod, "password_change_required": p.PasswordChangeRequired, "tenants": ts, "pagination": r.Context().Value(pageContextKey{})})
		return nil
	})
	s.api("GET /v1/my-tenants", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		v, e := s.myTenants(r, p)
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
			_, _ = w.Write(b) // client disconnects are not actionable
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/" + name
		files.ServeHTTP(w, clone)
	})
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/console/", 302) })
	return s.observe(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.serveAppHost(w, r) {
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		s.mux.ServeHTTP(w, r)
	}))
}

// TenantRef is a Tenant the principal belongs to.
type TenantRef struct {
	Name  string `json:"display_name"`
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	State string `json:"state"`
}

func (v *TenantRef) fields() []any { return []any{&v.Name, &v.ID, &v.Slug, &v.State} }

// myTenants pages the Tenants where the principal has an active membership.
// It is the one query behind both /v1/me and /v1/my-tenants.
func (s *Server) myTenants(r *http.Request, p Principal) ([]TenantRef, error) {
	return pageRows[TenantRef](r.Context(), s.DB, r, "SELECT t.display_name,t.id,t.slug,t.state FROM tenant_memberships m JOIN tenants t ON t.id=m.tenant_id WHERE m.user_id=? AND m.state='active' ORDER BY t.display_name", p.ID)
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

// stateCount is how many database resources are in a state.
type stateCount struct {
	Count int64  `json:"count"`
	State string `json:"state"`
}

func (v *stateCount) fields() []any { return []any{&v.Count, &v.State} }

// recentDatabase is a recently created database resource on the platform
// overview.
type recentDatabase struct {
	Name        string  `json:"database_name"`
	ID          string  `json:"id"`
	LastMessage *string `json:"last_error_message"`
	State       string  `json:"state"`
	TenantID    string  `json:"tenant_id"`
	TenantName  string  `json:"tenant_name"`
}

func (v *recentDatabase) fields() []any {
	return []any{&v.Name, &v.ID, &v.LastMessage, &v.State, &v.TenantID, &v.TenantName}
}

// AuditEvent is an audit trail entry with its actor's name.
type AuditEvent struct {
	Action       string    `json:"action"`
	ActorName    string    `json:"actor_name"`
	CreatedAt    time.Time `json:"created_at"`
	Details      jsonValue `json:"details_json"`
	ID           int64     `json:"id"`
	RequestID    string    `json:"request_id"`
	ResourceID   *string   `json:"resource_id"`
	ResourceType string    `json:"resource_type"`
	TenantID     *string   `json:"tenant_id"`
}

func (v *AuditEvent) fields() []any {
	return []any{&v.Action, &v.ActorName, &v.CreatedAt, &v.Details, &v.ID, &v.RequestID, &v.ResourceID, &v.ResourceType, &v.TenantID}
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
	status, e := scanAll[stateCount](r.Context(), s.DB, "SELECT COUNT(*),state FROM database_resources GROUP BY state")
	if e != nil {
		return e
	}
	out["database_states"] = status
	recent, e := scanAll[recentDatabase](r.Context(), s.DB, "SELECT d.database_name,d.id,d.last_error_message,d.state,d.tenant_id,t.display_name FROM database_resources d JOIN tenants t ON t.id=d.tenant_id ORDER BY d.created_at DESC LIMIT 6")
	if e != nil {
		return e
	}
	out["recent_databases"] = recent
	send(w, 200, out)
	return e
}

func (s *Server) Audits(ctx context.Context, actor Actor, tenant string, page PageRequest) (Page[AuditEvent], error) {
	p := actor.Principal
	action := "audit.read"
	if tenant == "" {
		action = "platform.admin"
	}
	if e := s.authorize(ctx, s.DB, p, action, Resource{Tenant: tenant}, false); e != nil {
		return Page[AuditEvent]{}, e
	}
	args := []any{}
	where := "1=1"
	if tenant != "" {
		where = "a.tenant_id=?"
		args = append(args, tenant)
	}
	v, e := pageScan[AuditEvent](ctx, s.DB, page, collectionFor("audit-events"), "SELECT a.action,COALESCE(u.display_name,'System worker') AS actor_name,a.created_at,a.details_json,a.id,a.request_id,a.resource_id,a.resource_type,a.tenant_id FROM audit_events a LEFT JOIN users u ON u.id=a.actor_user_id WHERE "+where+" ORDER BY a.id DESC", args...)
	return v, e
}

// httpActor is the console caller of an operation.
func httpActor(r *http.Request, p Principal) Actor {
	return actorFrom(r.Context(), p, r.Header.Get("X-Request-ID"))
}

// sendRef sends a mutation result with its revision as the ETag.
func sendRef(w http.ResponseWriter, status int, v Ref) {
	w.Header().Set("ETag", strconv.FormatInt(v.Revision, 10))
	send(w, status, v)
}

func result(w http.ResponseWriter, r *http.Request, s *Server, table, tenant, record string, status int) error {
	query := "SELECT id,revision FROM " + table + " WHERE id=?"
	args := []any{record}
	if tenant != "" {
		query += " AND tenant_id=?"
		args = append(args, tenant)
	}
	var v Ref
	e := lookup(r.Context(), s.DB, query, args, &v.ID, &v.Revision)
	if e == nil {
		w.Header().Set("ETag", strconv.FormatInt(v.Revision, 10))
		send(w, status, v)
	}
	return e
}

// lockedRecord is the revision and lifecycle state of a record locked for
// update.
type lockedRecord struct {
	Revision int64
	State    string
}

func lockRecord(ctx context.Context, tx *sql.Tx, table, tenant, record string) (lockedRecord, error) {
	query := "SELECT revision,state FROM " + table + " WHERE id=?"
	args := []any{record}
	if tenant != "" {
		query += " AND tenant_id=?"
		args = append(args, tenant)
	}
	var v lockedRecord
	return v, lookup(ctx, tx, query+" FOR UPDATE", args, &v.Revision, &v.State)
}

func (s *Server) audits(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.Audits(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	return reply(w, 200, v, e)
}

// reply sends an operation's successful result with status.
func reply[T any](w http.ResponseWriter, status int, v T, e error) error {
	if e == nil {
		send(w, status, v)
	}
	return e
}
