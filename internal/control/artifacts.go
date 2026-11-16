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

	"github.com/aantich/lastapp/internal/artifact"
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

func (s *Server) ownedDevelopmentApp(ctx context.Context, q Queryer, p Principal, app string) (Row, Row, error) {
	a, e := s.appRow(ctx, q, app)
	if e != nil {
		return nil, nil, e
	}
	if a["state"] != stateActive {
		return nil, nil, conflict("Restore the App before developing or releasing it.")
	}
	w, e := row(ctx, q, "SELECT * FROM development_workspaces WHERE backing_space_id=? AND state='ready'", a["space_id"])
	if e != nil {
		return nil, nil, missing()
	}
	if e = s.workspaceOwner(ctx, q, p, w); e != nil {
		return nil, nil, e
	}
	if e = s.authorize(ctx, q, p, "release.manage", Resource{Tenant: str(a["tenant_id"]), App: app}, true); e != nil {
		return nil, nil, e
	}
	return a, w, nil
}

func (s *Server) artifactTicket(w http.ResponseWriter, r *http.Request, p Principal) error {
	if s.artifactDirectory() == "" {
		return conflict("Configure persistent artifact storage first.")
	}
	a, _, e := s.ownedDevelopmentApp(r.Context(), s.DB, p, r.PathValue("app"))
	if e != nil {
		return e
	}
	if p.Agent != nil {
		if e = requireAgent(p.Agent, "releases.write"); e != nil {
			return e
		}
	}
	secret := token(32)
	e = s.mutate(r.Context(), p, "release.manage", Resource{Tenant: str(a["tenant_id"]), App: str(a["id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM artifact_uploads WHERE user_id=? AND expires_at>UTC_TIMESTAMP(6) AND build_id IS NULL", p.ID).Scan(&n); e != nil {
			return e
		}
		if n >= maxOutstandingUploads {
			return conflict("Too many outstanding uploads. Finish an upload or wait for expiry.")
		}
		var rev any
		if p.Agent != nil {
			rev = p.Agent.Revision
		}
		_, e := tx.ExecContext(r.Context(), "INSERT INTO artifact_uploads(token_hash,tenant_id,app_id,user_id,agent_grant_id,agent_grant_revision,expires_at) VALUES(?,?,?,?,?,?,?)", hash(secret), a["tenant_id"], a["id"], p.ID, agentID(p), rev, time.Now().Add(uploadTicketLifetime))
		return e
	})
	if e != nil {
		return e
	}
	send(w, 201, Row{"upload_url": s.Config.PublicURL + "/artifact/v1/upload", "upload_token": secret, "expires_in": int(uploadTicketLifetime.Seconds()), "limits": artifact.Limits, "instructions": "POST {entrypoint:'index.html',files:[{path:'...',content_base64:'...'}]} using Authorization: Bearer <upload_token>. A ticket uploads one immutable build; identical retries return its build ID. Do not put tokens or local development credentials in builds."})
	return nil
}

func (s *Server) uploadPrincipal(ctx context.Context, q Queryer, v Row) (Principal, error) {
	p := Principal{ID: str(v["user_id"])}
	if str(v["agent_grant_id"]) != "" {
		g, e := s.readAgentGrant(ctx, q, str(v["agent_grant_id"]))
		if e != nil {
			return p, e
		}
		if g.Revision != integer(v["agent_grant_revision"]) {
			return p, missing()
		}
		if e = requireAgent(g, "releases.write"); e != nil {
			return p, e
		}
		p.Agent = g
	}
	if _, _, e := s.ownedDevelopmentApp(ctx, q, p, str(v["app_id"])); e != nil {
		return p, e
	}
	return p, nil
}

func (s *Server) artifactUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	run := func() error {
		if !s.limiter.allowRate("upload:"+s.loginClientIP(r), time.Now(), 20, 4) {
			return rateLimited(w)
		}
		select {
		case s.uploadSlots <- struct{}{}:
			defer func() { <-s.uploadSlots }()
		default:
			return rateLimited(w)
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(raw) != 43 {
			return problem(401, "invalid_upload", "Upload ticket required.")
		}
		v, e := row(ctx, s.DB, "SELECT * FROM artifact_uploads WHERE token_hash=? AND expires_at>UTC_TIMESTAMP(6)", hash(raw))
		if e != nil {
			return problem(401, "invalid_upload", "Upload ticket expired or invalid.")
		}
		if _, e = s.uploadPrincipal(ctx, s.DB, v); e != nil {
			return e
		}
		if origin := r.Header.Get("Origin"); origin != "" && !s.allowedOrigin(origin) {
			return problem(403, "invalid_origin", "Upload is restricted to trusted authoring clients.")
		}
		var in struct {
			Files []artifact.File `json:"files"`
			Entry string          `json:"entrypoint"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, artifact.MaxBody))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil {
			return bad("Invalid or oversized static build upload.")
		}
		var more any
		if !errors.Is(d.Decode(&more), io.EOF) {
			return bad("One upload object required.")
		}
		b, e := artifact.Inspect(in.Files, in.Entry)
		if e != nil {
			return bad(e.Error())
		}
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if e = lockTenant(ctx, tx, str(v["tenant_id"])); e != nil {
			return e
		}
		v, e = row(ctx, tx, "SELECT * FROM artifact_uploads WHERE token_hash=? AND expires_at>UTC_TIMESTAMP(6) FOR UPDATE", hash(raw))
		if e != nil {
			return missing()
		}
		p, e := s.uploadPrincipal(ctx, tx, v)
		if e != nil {
			return e
		}
		if v["build_id"] != nil {
			existing, e := row(ctx, tx, "SELECT id,content_hash FROM app_builds WHERE id=?", v["build_id"])
			if e != nil {
				return e
			}
			if str(existing["content_hash"]) != b.Hash {
				return conflict("Upload ticket already used for different content.")
			}
			send(w, 200, existing)
			return nil
		}
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM app_builds WHERE app_id=?", v["app_id"]).Scan(&n); e != nil {
			return e
		}
		if n >= maxAppBuilds {
			return conflict("App build limit reached. Create a fresh workspace.")
		}
		if e = artifact.Store(s.artifactDirectory(), in.Files, b); e != nil {
			s.logger(ctx).Warn("artifact storage failed", diagnostic(e)...)
			return problem(503, "artifact_storage", "Could not persist the build.")
		}
		build := newID()
		files, _ := json.Marshal(b.Files)
		if _, e = tx.ExecContext(ctx, "INSERT INTO app_builds(id,tenant_id,app_id,content_hash,files_json,entrypoint,size_bytes,created_by) VALUES(?,?,?,?,?,?,?,?)", build, v["tenant_id"], v["app_id"], b.Hash, files, b.Entry, b.Size, p.ID); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE artifact_uploads SET build_id=? WHERE token_hash=?", build, hash(raw)); e != nil {
			return e
		}
		ctx = agentAuditContext(ctx, str(v["agent_grant_id"]), "")
		if e = audit(ctx, tx, str(v["tenant_id"]), p.ID, "build.uploaded", "app", str(v["app_id"]), r.Header.Get("X-Request-ID"), Row{"build_id": build, "content_hash": b.Hash, "size_bytes": b.Size}); e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		send(w, 201, Row{"id": build, "content_hash": b.Hash, "size_bytes": b.Size})
		return nil
	}
	if e := run(); e != nil {
		writeError(w, r, e)
	}
}

func (s *Server) buildList(w http.ResponseWriter, r *http.Request, p Principal) error {
	a, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if e = s.authorize(r.Context(), s.DB, p, "release.manage", Resource{Tenant: str(a["tenant_id"]), App: str(a["id"])}, false); e != nil {
		return e
	}
	vs, e := pageRows(r.Context(), s.DB, r, "SELECT id,content_hash,entrypoint,size_bytes,created_at FROM app_builds WHERE app_id=? ORDER BY created_at DESC,id", a["id"])
	if e == nil {
		sendPage(w, r, vs)
	}
	return e
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
	if id(app) != nil {
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
	a, e := row(ctx, s.DB, "SELECT a.*,sp.workspace_id FROM apps a JOIN spaces sp ON sp.id=a.space_id AND sp.state='active' JOIN tenants t ON t.id=a.tenant_id AND t.state='active' LEFT JOIN development_workspaces ws ON ws.id=sp.workspace_id WHERE a.id=? AND a.state='active' AND (sp.workspace_id IS NULL OR ws.state='ready')", app)
	if e != nil {
		http.NotFound(w, r)
		return true
	}
	if r.URL.Path == "/_lastapp/config.json" {
		send(w, 200, Row{"apiOrigin": s.Config.PublicURL, "appID": app, "environment": map[bool]string{true: "development", false: "production"}[a["workspace_id"] != nil]})
		return true
	}
	if a["active_release_id"] == nil {
		http.NotFound(w, r)
		return true
	}
	prefix := "/_lastapp/builds/"
	var build Row
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		build, e = row(ctx, s.DB, "SELECT b.id,b.entrypoint FROM app_releases r JOIN app_builds b ON b.id=r.build_id WHERE r.id=?", a["active_release_id"])
		if e != nil {
			http.NotFound(w, r)
			return true
		}
		http.Redirect(w, r, prefix+str(build["id"])+"/"+str(build["entrypoint"]), 302)
		return true
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, prefix), "/", 2)
	if !strings.HasPrefix(r.URL.Path, prefix) || len(parts) != 2 || id(parts[0]) != nil || !artifact.ValidPath(parts[1]) {
		http.NotFound(w, r)
		return true
	}
	// Historical builds remain addressable for already-open clients, but only if
	// a release was actually deployed to this App (never arbitrary workspace builds).
	build, e = row(ctx, s.DB, "SELECT b.* FROM app_builds b WHERE b.id=? AND (EXISTS(SELECT 1 FROM app_releases rel WHERE rel.build_id=b.id AND rel.app_id=?) OR EXISTS(SELECT 1 FROM release_deployments d JOIN app_releases rel ON rel.id=d.release_id WHERE rel.build_id=b.id AND d.target_app_id=? AND d.state='active'))", parts[0], app, app)
	if e != nil {
		http.NotFound(w, r)
		return true
	}
	var files []artifact.FileInfo
	encoded, _ := json.Marshal(build["files_json"])
	if json.Unmarshal(encoded, &files) != nil {
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
	file, e := artifact.Read(s.artifactDirectory(), str(build["content_hash"]), parts[1])
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

func (s *Server) ensureReleaseBuild(ctx context.Context, q Queryer, release string) error {
	v, e := row(ctx, q, "SELECT b.* FROM app_releases rel JOIN app_builds b ON b.id=rel.build_id WHERE rel.id=?", release)
	if e != nil {
		return e
	}
	var files []artifact.FileInfo
	b, _ := json.Marshal(v["files_json"])
	if e = json.Unmarshal(b, &files); e != nil {
		return e
	}
	build := artifact.Build{Hash: str(v["content_hash"]), Files: files, Size: int(integer(v["size_bytes"])), Entry: str(v["entrypoint"])}
	if e = artifact.Check(s.artifactDirectory(), build); e != nil {
		return conflict("Release files are unavailable or corrupt on this server. Restore the shared artifact storage before deploying.")
	}
	return nil
}
