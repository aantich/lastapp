package control

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

const (
	appTokenLifetime = 30 * time.Minute
	launchLifetime   = 2 * time.Minute
)

var (
	verifierRE = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	pkceRE     = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

func (s *Server) appRow(ctx context.Context, q registry.Queryer, app string) (registry.AppRecord, error) {
	return registry.ScanOne[registry.AppRecord](ctx, q, "SELECT "+registry.AppRecordColumns+" FROM apps WHERE id=?", app)
}

func (s *Server) appLaunch(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		Challenge string `json:"challenge"`
		Nonce     string `json:"nonce"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if !pkceRE.MatchString(in.Challenge) || !pkceRE.MatchString(in.Nonce) {
		return registry.Bad("Valid S256 challenge and nonce required.")
	}
	v, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if v.State != registry.StateActive {
		return registry.Missing()
	}
	if e = s.WorkspaceGuard(r.Context(), s.DB, p, v.SpaceID); e != nil {
		return e
	}
	if e = s.Member(r.Context(), s.DB, p, v.TenantID, true); e != nil {
		return e
	}
	var sp string
	if e = s.DB.QueryRowContext(r.Context(), "SELECT state FROM spaces WHERE id=?", v.SpaceID).Scan(&sp); e != nil || sp != registry.StateActive {
		return registry.Missing()
	}
	a, e := registry.AccessOf(v.Access)
	if e != nil {
		return e
	}
	if a.Origin == "" {
		return registry.Conflict("A Space manager must approve the App origin first.")
	}
	cookie, e := r.Cookie(httpapi.CookieName(s.Config.PublicURL, "lastapp_session"))
	if e != nil {
		return e
	}
	code := registry.Token(32)
	_, e = s.DB.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,parent_hash,tenant_id,space_id,app_id,origin,origin_generation,challenge,nonce,expires_at) VALUES(?,?,?,'launch',?,?,?,?,?,?,?,?,?)", hash(code), p.ID, hash(""), hash(cookie.Value), v.TenantID, v.SpaceID, v.ID, a.Origin, v.OriginGeneration, in.Challenge, in.Nonce, time.Now().Add(launchLifetime))
	if e != nil {
		return e
	}
	httpapi.Send(w, 200, registry.Row{"code": code, "origin": a.Origin, "nonce": in.Nonce, "expires_in": int(launchLifetime.Seconds())})
	return nil
}

func (s *Server) exchange(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code     string `json:"code"`
		Verifier string `json:"verifier"`
		Nonce    string `json:"nonce"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if !verifierRE.MatchString(in.Verifier) || len(in.Code) > 128 {
		return registry.Bad("Invalid launch exchange.")
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := registry.ScanOne[appSession](r.Context(), tx, "SELECT "+appSessionColumns+" FROM sessions l JOIN users u ON u.id=l.user_id AND u.state='active' JOIN apps a ON a.id=l.app_id AND a.state='active' AND a.origin_generation=l.origin_generation JOIN spaces sp ON sp.id=l.space_id AND sp.state='active' JOIN tenants t ON t.id=l.tenant_id AND t.state='active' JOIN tenant_memberships m ON m.tenant_id=t.id AND m.user_id=u.id AND m.state='active' WHERE l.token_hash=? AND l.purpose='launch' AND l.state='active' AND l.expires_at>UTC_TIMESTAMP(6) FOR UPDATE", hash(in.Code))
	if e != nil {
		return registry.Problem(401, "invalid_exchange", "Launch code is invalid or expired.")
	}
	if e = s.appSessionParent(r.Context(), tx, v); e != nil {
		return e
	}
	challenge := base64.RawURLEncoding.EncodeToString(hash(in.Verifier))
	if subtle.ConstantTimeCompare([]byte(challenge), []byte(v.Challenge)) != 1 || subtle.ConstantTimeCompare([]byte(in.Nonce), []byte(v.Nonce)) != 1 || r.Header.Get("Origin") != access.Deref(v.Origin) {
		return registry.Problem(401, "invalid_exchange", "Launch challenge or origin mismatch.")
	}
	raw := registry.Token(32)
	_, e = tx.ExecContext(r.Context(), "UPDATE sessions SET state='consumed' WHERE token_hash=?", hash(in.Code))
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,parent_hash,tenant_id,space_id,app_id,origin,origin_generation,agent_grant_id,agent_grant_revision,expires_at) VALUES(?,?,?,'app',?,?,?,?,?,?,?,?,?)", hash(raw), v.UserID, hash(""), v.ParentHash, v.TenantID, v.SpaceID, v.AppID, v.Origin, v.OriginGeneration, v.AgentGrant, v.AgentRevision, time.Now().Add(appTokenLifetime))
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	w.Header().Set("Access-Control-Allow-Origin", access.Deref(v.Origin))
	w.Header().Add("Vary", "Origin")
	httpapi.Send(w, 200, registry.Row{"access_token": raw, "expires_in": int(appTokenLifetime.Seconds()), "token_type": "Bearer"})
	return nil
}

// appSession is a launch code or App data session with the database
// resource of its Space.
type appSession struct {
	UserID, TenantID, SpaceID, AppID, Challenge, Nonce, Resource string
	OriginGeneration                                             int64
	Origin, AgentGrant                                           *string
	AgentRevision                                                *int64
	ParentHash                                                   []byte
}

// appSessionColumns selects an appSession from sessions l joined to spaces sp.
const appSessionColumns = "l.user_id,l.tenant_id,l.space_id,l.app_id,COALESCE(l.challenge,''),COALESCE(l.nonce,''),sp.database_resource_id,l.origin_generation,l.origin,l.agent_grant_id,l.agent_grant_revision,l.parent_hash"

func (v *appSession) ScanFields() []any {
	return []any{&v.UserID, &v.TenantID, &v.SpaceID, &v.AppID, &v.Challenge, &v.Nonce, &v.Resource, &v.OriginGeneration, &v.Origin, &v.AgentGrant, &v.AgentRevision, &v.ParentHash}
}

func (s *Server) dataSession(ctx context.Context, r *http.Request) (appSession, error) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if raw == "" || raw == r.Header.Get("Authorization") || len(raw) > 128 {
		return appSession{}, registry.Problem(401, "unauthenticated", "An App bearer token is required.")
	}
	v, e := registry.ScanOne[appSession](ctx, s.DB, "SELECT "+appSessionColumns+" FROM sessions l JOIN users u ON u.id=l.user_id AND u.state='active' LEFT JOIN local_accounts la ON la.user_id=u.id JOIN apps a ON a.id=l.app_id AND a.tenant_id=l.tenant_id AND a.space_id=l.space_id AND a.state='active' AND a.origin_generation=l.origin_generation JOIN spaces sp ON sp.id=l.space_id AND sp.state='active' JOIN tenants t ON t.id=l.tenant_id AND t.state='active' JOIN tenant_memberships m ON m.tenant_id=t.id AND m.user_id=u.id AND m.state='active' WHERE l.token_hash=? AND l.purpose='app' AND l.state='active' AND l.expires_at>UTC_TIMESTAMP(6)", hash(raw))
	if e != nil {
		return v, registry.Problem(401, "unauthenticated", "App session expired or revoked.")
	}
	if e = s.appSessionParent(ctx, s.DB, v); e != nil {
		return v, e
	}
	if r.Header.Get("Origin") != access.Deref(v.Origin) {
		return v, registry.Problem(403, "invalid_origin", "App origin mismatch.")
	}
	return v, nil
}

func (s *Server) appLaunchInfo(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var appID, name, space, spaceName, tenant string
	var access registry.JSONValue
	e := registry.Lookup(r.Context(), s.DB, "SELECT a.id,a.display_name,a.data_access_json,a.space_id,sp.display_name AS space_name,a.tenant_id FROM apps a JOIN spaces sp ON sp.id=a.space_id AND sp.state='active' WHERE a.id=? AND a.state='active'", []any{r.PathValue("app")}, &appID, &name, &access, &space, &spaceName, &tenant)
	if e != nil {
		return e
	}
	if e = s.WorkspaceGuard(r.Context(), s.DB, p, space); e != nil {
		return e
	}
	if e = s.Member(r.Context(), s.DB, p, tenant, true); e != nil {
		return e
	}
	a, e := registry.AccessOf(access)
	if e != nil {
		return e
	}
	if a.Origin == "" {
		return registry.Conflict("A Space manager must approve this App's origin.")
	}
	httpapi.Send(w, 200, registry.Row{"id": appID, "name": name, "space": spaceName, "origin": a.Origin})
	return nil
}
