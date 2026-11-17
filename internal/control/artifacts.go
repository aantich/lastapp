package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/artifact"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/inventory"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

func (s *Server) artifactDirectory() string {
	if s.Config.ArtifactDirectory != "" {
		return s.Config.ArtifactDirectory
	}
	if s.Config.Development {
		return filepath.Join(".local", "artifacts")
	}
	return ""
}

func (s *Server) appHostingBase() string {
	base := s.Config.AppOrigin
	if base == "" && s.Config.Development {
		u, _ := url.Parse(s.Config.PublicURL)
		port := u.Port()
		u.Host = "localhost"
		if port != "" {
			u.Host += ":" + port
		}
		base = u.String()
	}
	return base
}

// Shared by startup validation and routing: every hostname below this suffix
// belongs to static Apps, including malformed App IDs and alternate ports.
func appHostNamespace(origin string) string {
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" {
		return ""
	}
	return "." + strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
}

func (s *Server) appHostedOrigin(app string) string {
	base := s.appHostingBase()
	u, e := url.Parse(base)
	if e != nil || u.Host == "" {
		return ""
	}
	port := u.Port()
	u.Host = app + appHostNamespace(base)
	if port != "" && !(u.Scheme == "https" && port == "443") && !(u.Scheme == "http" && port == "80") {
		u.Host = net.JoinHostPort(u.Host, port)
	}
	return strings.TrimSuffix(u.String(), "/")
}

func (s *Server) releaseRoutes() {
	s.api("POST /v1/apps/{app}/upload-tickets", s.artifactTicket)
	s.mux.HandleFunc("POST /artifact/v1/upload", s.artifactUpload)
	s.api("GET /v1/apps/{app}/builds", s.buildList)
	s.api("GET /v1/apps/{app}/releases", s.releaseList)
	s.api("GET /v1/apps/{app}/deployment", s.appDeployment)
	s.api("POST /v1/apps/{app}/releases", s.releaseCreate)
	s.api("GET /v1/releases/{release}", s.releaseGet)
	s.api("POST /v1/releases/{release}/preview", s.releasePreview)
	s.api("POST /v1/releases/{release}/deploy", s.releaseDeploy)
	s.api("GET /v1/deployments/{deployment}", s.deploymentGet)
	s.api("GET /v1/apps/{app}/starter", s.appStarter)
	s.api("GET /v1/apps/{app}/starter.zip", s.appStarterArchive)
}

func (s *Server) ownedDevelopmentApp(ctx context.Context, q registry.Queryer, p access.Principal, app string) (registry.AppRecord, registry.Workspace, error) {
	a, e := inventory.AppRow(ctx, q, app)
	if e != nil {
		return registry.AppRecord{}, registry.Workspace{}, e
	}
	if a.State != registry.StateActive {
		return registry.AppRecord{}, registry.Workspace{}, registry.Conflict("Restore the App before developing or releasing it.")
	}
	w, e := registry.ScanOne[registry.Workspace](ctx, q, "SELECT "+registry.WorkspaceColumns+" FROM development_workspaces w WHERE w.backing_space_id=? AND w.state='ready'", a.SpaceID)
	if e != nil {
		return registry.AppRecord{}, registry.Workspace{}, registry.Missing()
	}
	if e = s.WorkspaceOwner(ctx, q, p, w); e != nil {
		return registry.AppRecord{}, registry.Workspace{}, e
	}
	if e = s.Authorize(ctx, q, p, "release.manage", access.Resource{Tenant: a.TenantID, App: app}, true); e != nil {
		return registry.AppRecord{}, registry.Workspace{}, e
	}
	return a, w, nil
}

func (s *Server) UploadTicket(ctx context.Context, actor op.Actor, app string) (registry.Row, error) {
	p := actor.Principal
	if s.artifactDirectory() == "" {
		return nil, registry.Conflict("Configure persistent artifact storage first.")
	}
	a, _, e := s.ownedDevelopmentApp(ctx, s.DB, p, app)
	if e != nil {
		return nil, e
	}
	if p.Agent != nil {
		if e = access.RequireAgent(p.Agent, "releases.write"); e != nil {
			return nil, e
		}
	}
	secret := registry.Token(32)
	e = s.run.Mutate(ctx, actor, "release.manage", access.Resource{Tenant: a.TenantID, App: a.ID}, func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM artifact_uploads WHERE user_id=? AND expires_at>UTC_TIMESTAMP(6) AND build_id IS NULL", p.ID).Scan(&n); e != nil {
			return e
		}
		if n >= registry.MaxOutstandingUploads {
			return registry.Conflict("Too many outstanding uploads. Finish an upload or wait for expiry.")
		}
		var rev any
		if p.Agent != nil {
			rev = p.Agent.Revision
		}
		_, e := tx.ExecContext(ctx, "INSERT INTO artifact_uploads(token_hash,tenant_id,app_id,user_id,agent_grant_id,agent_grant_revision,expires_at) VALUES(?,?,?,?,?,?,?)", registry.TokenHash(secret), a.TenantID, a.ID, p.ID, agentID(p), rev, time.Now().Add(registry.UploadTicketLifetime))
		return e
	})
	if e != nil {
		return nil, e
	}
	return registry.Row{"upload_url": s.Config.PublicURL + "/artifact/v1/upload", "upload_token": secret, "expires_in": int(registry.UploadTicketLifetime.Seconds()), "limits": artifact.Limits, "instructions": "POST {entrypoint:'index.html',files:[{path:'...',content_base64:'...'}]} using Authorization: Bearer <upload_token>. A ticket uploads one immutable build; identical retries return its build ID. Do not put tokens or local development credentials in builds."}, nil
}

// uploadTicket is a bounded build-upload credential and the build it
// produced, if it was already used.
type uploadTicket struct {
	TenantID, AppID, UserID, AgentGrant string // AgentGrant is empty for a person
	AgentRevision                       int64
	BuildID                             *string
}

const uploadTicketColumns = "tenant_id,app_id,user_id,COALESCE(agent_grant_id,''),COALESCE(agent_grant_revision,0),build_id"

func (v *uploadTicket) ScanFields() []any {
	return []any{&v.TenantID, &v.AppID, &v.UserID, &v.AgentGrant, &v.AgentRevision, &v.BuildID}
}

// storedBuild is an uploaded static build and its file inventory.
type storedBuild struct {
	ID, Hash, Entry string
	Size            int64
	Files           registry.JSONValue
}

const storedBuildColumns = "b.id,b.content_hash,b.entrypoint,b.size_bytes,b.files_json"

func (v *storedBuild) ScanFields() []any { return []any{&v.ID, &v.Hash, &v.Entry, &v.Size, &v.Files} }

// files decodes the build's file inventory.
func (v storedBuild) files() ([]artifact.FileInfo, error) {
	var files []artifact.FileInfo
	b, _ := json.Marshal(v.Files)
	return files, json.Unmarshal(b, &files)
}

// Build is an uploaded build in an App's collection.
type Build struct {
	Hash      string    `json:"content_hash"`
	CreatedAt time.Time `json:"created_at"`
	Entry     string    `json:"entrypoint"`
	ID        string    `json:"id"`
	Size      int64     `json:"size_bytes"`
}

func (v *Build) ScanFields() []any { return []any{&v.Hash, &v.CreatedAt, &v.Entry, &v.ID, &v.Size} }

func (s *Server) uploadPrincipal(ctx context.Context, q registry.Queryer, v uploadTicket) (access.Principal, error) {
	p := access.Principal{ID: v.UserID}
	if v.AgentGrant != "" {
		g, e := s.ReadAgentGrant(ctx, q, v.AgentGrant)
		if e != nil {
			return p, e
		}
		if g.Revision != v.AgentRevision {
			return p, registry.Missing()
		}
		if e = access.RequireAgent(g, "releases.write"); e != nil {
			return p, e
		}
		p.Agent = g
	}
	if _, _, e := s.ownedDevelopmentApp(ctx, q, p, v.AppID); e != nil {
		return p, e
	}
	return p, nil
}

func (s *Server) artifactUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	run := func() error {
		if !s.limiter.AllowRate("upload:"+s.trustedProxies.ClientIP(r), time.Now(), 20, 4) {
			return httpapi.RateLimited(w)
		}
		select {
		case s.uploadSlots <- struct{}{}:
			defer func() { <-s.uploadSlots }()
		default:
			return httpapi.RateLimited(w)
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(raw) != 43 {
			return registry.Problem(401, "invalid_upload", "Upload ticket required.")
		}
		v, e := registry.ScanOne[uploadTicket](ctx, s.DB, "SELECT "+uploadTicketColumns+" FROM artifact_uploads WHERE token_hash=? AND expires_at>UTC_TIMESTAMP(6)", registry.TokenHash(raw))
		if e != nil {
			return registry.Problem(401, "invalid_upload", "Upload ticket expired or invalid.")
		}
		if _, e = s.uploadPrincipal(ctx, s.DB, v); e != nil {
			return e
		}
		if origin := r.Header.Get("Origin"); origin != "" && !httpapi.AllowedOrigin(&s.Config, origin) {
			return registry.Problem(403, "invalid_origin", "Upload is restricted to trusted authoring clients.")
		}
		var in struct {
			Files []artifact.File `json:"files"`
			Entry string          `json:"entrypoint"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, artifact.MaxBody))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil {
			return registry.Bad("Invalid or oversized static build upload.")
		}
		var more any
		if !errors.Is(d.Decode(&more), io.EOF) {
			return registry.Bad("One upload object required.")
		}
		b, e := artifact.Inspect(in.Files, in.Entry)
		if e != nil {
			return registry.Bad(e.Error())
		}
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if e = lockTenant(ctx, tx, v.TenantID); e != nil {
			return e
		}
		v, e = registry.ScanOne[uploadTicket](ctx, tx, "SELECT "+uploadTicketColumns+" FROM artifact_uploads WHERE token_hash=? AND expires_at>UTC_TIMESTAMP(6) FOR UPDATE", registry.TokenHash(raw))
		if e != nil {
			return registry.Missing()
		}
		p, e := s.uploadPrincipal(ctx, tx, v)
		if e != nil {
			return e
		}
		if v.BuildID != nil {
			var existing string
			if e = registry.Lookup(ctx, tx, "SELECT content_hash FROM app_builds WHERE id=?", []any{*v.BuildID}, &existing); e != nil {
				return e
			}
			if existing != b.Hash {
				return registry.Conflict("Upload ticket already used for different content.")
			}
			httpapi.Send(w, 200, registry.Row{"content_hash": existing, "id": *v.BuildID})
			return nil
		}
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM app_builds WHERE app_id=?", v.AppID).Scan(&n); e != nil {
			return e
		}
		if n >= registry.MaxAppBuilds {
			return registry.Conflict("App build limit reached. Create a fresh workspace.")
		}
		if e = artifact.Store(s.artifactDirectory(), in.Files, b); e != nil {
			httpapi.Logger(ctx, s.Log).Warn("artifact storage failed", httpapi.Diagnostic(e)...)
			return registry.Problem(503, "artifact_storage", "Could not persist the build.")
		}
		build := registry.NewID()
		files, _ := json.Marshal(b.Files)
		if _, e = tx.ExecContext(ctx, "INSERT INTO app_builds(id,tenant_id,app_id,content_hash,files_json,entrypoint,size_bytes,created_by) VALUES(?,?,?,?,?,?,?,?)", build, v.TenantID, v.AppID, b.Hash, files, b.Entry, b.Size, p.ID); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE artifact_uploads SET build_id=? WHERE token_hash=?", build, registry.TokenHash(raw)); e != nil {
			return e
		}
		ctx = op.AgentAuditContext(ctx, v.AgentGrant, "")
		if e = op.Audit(ctx, tx, v.TenantID, p.ID, "build.uploaded", "app", v.AppID, r.Header.Get("X-Request-ID"), registry.Row{"build_id": build, "content_hash": b.Hash, "size_bytes": b.Size}); e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		httpapi.Send(w, 201, registry.Row{"id": build, "content_hash": b.Hash, "size_bytes": b.Size})
		return nil
	}
	if e := run(); e != nil {
		httpapi.WriteError(w, r, e)
	}
}

func (s *Server) Builds(ctx context.Context, actor op.Actor, app string, page op.PageRequest) (op.Page[Build], error) {
	p := actor.Principal
	a, e := inventory.AppRow(ctx, s.DB, app)
	if e != nil {
		return op.Page[Build]{}, e
	}
	if e = s.Authorize(ctx, s.DB, p, "release.manage", access.Resource{Tenant: a.TenantID, App: a.ID}, false); e != nil {
		return op.Page[Build]{}, e
	}
	vs, e := op.PageScan[Build](ctx, s.DB, page, op.CollectionFor("builds"), "SELECT content_hash,created_at,entrypoint,id,size_bytes FROM app_builds WHERE app_id=? ORDER BY created_at DESC,id", a.ID)
	return vs, e
}

// App hosts are selected before the console router. A hostile App can never
// serve active content on the console origin or another App's origin.
func (s *Server) serveAppHost(w http.ResponseWriter, r *http.Request) bool {
	suffix := appHostNamespace(s.appHostingBase())
	if suffix == "" {
		return false
	}
	// The whole App hostname namespace is static-only, regardless of the port
	// spelling used by a browser or reverse proxy. Never fall back to management.
	host := strings.TrimSuffix(strings.ToLower((&url.URL{Host: r.Host}).Hostname()), ".")
	if !strings.HasSuffix(host, suffix) {
		return false
	}
	app := strings.TrimSuffix(host, suffix)
	if registry.ID(app) != nil {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' "+s.Config.PublicURL+"; worker-src 'self' blob:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var workspace, release *string
	e := registry.Lookup(ctx, s.DB, "SELECT sp.workspace_id,a.active_release_id FROM apps a JOIN spaces sp ON sp.id=a.space_id AND sp.state='active' JOIN tenants t ON t.id=a.tenant_id AND t.state='active' LEFT JOIN development_workspaces ws ON ws.id=sp.workspace_id WHERE a.id=? AND a.state='active' AND (sp.workspace_id IS NULL OR ws.state='ready')", []any{app}, &workspace, &release)
	if e != nil {
		http.NotFound(w, r)
		return true
	}
	if r.URL.Path == "/_lastapp/config.json" {
		httpapi.Send(w, 200, registry.Row{"apiOrigin": s.Config.PublicURL, "appID": app, "environment": map[bool]string{true: "development", false: "production"}[workspace != nil]})
		return true
	}
	if release == nil {
		http.NotFound(w, r)
		return true
	}
	prefix := "/_lastapp/builds/"
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		var build, entry string
		if registry.Lookup(ctx, s.DB, "SELECT b.id,b.entrypoint FROM app_releases r JOIN app_builds b ON b.id=r.build_id WHERE r.id=?", []any{*release}, &build, &entry) != nil {
			http.NotFound(w, r)
			return true
		}
		http.Redirect(w, r, prefix+build+"/"+entry, 302)
		return true
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, prefix), "/", 2)
	if !strings.HasPrefix(r.URL.Path, prefix) || len(parts) != 2 || registry.ID(parts[0]) != nil || !artifact.ValidPath(parts[1]) {
		http.NotFound(w, r)
		return true
	}
	// Historical builds remain addressable for already-open clients, but only if
	// a release was actually deployed to this App (never arbitrary workspace builds).
	build, e := registry.ScanOne[storedBuild](ctx, s.DB, "SELECT "+storedBuildColumns+" FROM app_builds b WHERE b.id=? AND (EXISTS(SELECT 1 FROM app_releases rel WHERE rel.build_id=b.id AND rel.app_id=?) OR EXISTS(SELECT 1 FROM release_deployments d JOIN app_releases rel ON rel.id=d.release_id WHERE rel.build_id=b.id AND d.target_app_id=? AND d.state='active'))", parts[0], app, app)
	if e != nil {
		http.NotFound(w, r)
		return true
	}
	files, e := build.files()
	if e != nil {
		http.NotFound(w, r)
		return true
	}
	allowed := false
	size := 0
	for _, f := range files {
		if f.Path == parts[1] {
			allowed = true
			size = f.Size
			break
		}
	}
	if !allowed {
		http.NotFound(w, r)
		return true
	}
	file, e := artifact.Read(s.artifactDirectory(), build.Hash, parts[1])
	if e != nil {
		http.NotFound(w, r)
		return true
	}
	defer file.Close()
	contentType := mime.TypeByExtension(path.Ext(parts[1]))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(size))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if r.Method == "GET" {
		_, _ = io.Copy(w, file)
	}
	return true
}

func (s *Server) ensureReleaseBuild(ctx context.Context, q registry.Queryer, release string) error {
	v, e := registry.ScanOne[storedBuild](ctx, q, "SELECT "+storedBuildColumns+" FROM app_releases rel JOIN app_builds b ON b.id=rel.build_id WHERE rel.id=?", release)
	if e != nil {
		return e
	}
	files, e := v.files()
	if e != nil {
		return e
	}
	build := artifact.Build{Hash: v.Hash, Files: files, Size: int(v.Size), Entry: v.Entry}
	if e = artifact.Check(s.artifactDirectory(), build); e != nil {
		return registry.Conflict("Release files are unavailable or corrupt on this server. Restore the shared artifact storage before deploying.")
	}
	return nil
}

func (s *Server) artifactTicket(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.UploadTicket(r.Context(), httpapi.Actor(r, p), r.PathValue("app"))
	return httpapi.Reply(w, 201, v, e)
}

func (s *Server) buildList(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.Builds(r.Context(), httpapi.Actor(r, p), r.PathValue("app"), httpapi.PageRequest(r.URL.Query()))
	return httpapi.Reply(w, 200, v, e)
}
