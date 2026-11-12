package control

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Principal struct {
	ID, Name, Email string
	Platform        bool
}
type Resource struct{ Tenant, Space, App string }
type oidcAuth struct {
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
}
type loginFlow struct {
	ReturnOrigin           string
	State, Nonce, Verifier string
	Issued                 int64
}
type scopeSet struct {
	All          bool
	Spaces, Apps []string
}

func (s *Server) InitAuth(ctx context.Context) error {
	if s.Config.OIDCIssuer == "" {
		return fmt.Errorf("oidc_issuer is required")
	}
	if !s.Config.Development && !secureEndpoint(s.Config.OIDCIssuer) {
		return fmt.Errorf("OIDC issuer requires HTTPS")
	}
	client := s.oidcClient()
	ctx = oidc.ClientContext(ctx, client)
	provider, e := oidc.NewProvider(ctx, s.Config.OIDCIssuer)
	if e != nil {
		return fmt.Errorf("OIDC provider discovery failed: %w", e)
	}
	if !s.Config.Development {
		var metadata struct {
			Auth  string `json:"authorization_endpoint"`
			Token string `json:"token_endpoint"`
			JWKS  string `json:"jwks_uri"`
		}
		if e = provider.Claims(&metadata); e != nil || !secureEndpoint(metadata.Auth) || !secureEndpoint(metadata.Token) || !secureEndpoint(metadata.JWKS) {
			return fmt.Errorf("OIDC endpoints must all use HTTPS")
		}
	}
	s.auth = &oidcAuth{config: oauth2.Config{ClientID: s.Config.OIDCClientID, ClientSecret: s.Config.OIDCClientSecret, Endpoint: provider.Endpoint(), RedirectURL: s.Config.PublicURL + "/auth/callback", Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}, verifier: provider.Verifier(&oidc.Config{ClientID: s.Config.OIDCClientID})}
	return nil
}
func (s *Server) cookie(w http.ResponseWriter, name, value string, age int, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(name), Value: value, Path: "/", MaxAge: age, HttpOnly: httpOnly, Secure: strings.HasPrefix(s.Config.PublicURL, "https:"), SameSite: http.SameSiteLaxMode})
}
func hash(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	origin := r.URL.Query().Get("return_origin")
	if origin != "" && !s.allowedOrigin(origin) {
		writeError(w, r, bad("Invalid return origin."))
		return
	}
	flow := loginFlow{ReturnOrigin: origin, State: token(32), Nonce: token(32), Verifier: oauth2.GenerateVerifier(), Issued: time.Now().Unix()}
	b, e := s.seal("oidc_flow", "browser", "", flow)
	if e != nil {
		writeError(w, r, e)
		return
	}
	s.cookie(w, "lastapp_flow", base64.RawURLEncoding.EncodeToString(b), 300, true)
	http.Redirect(w, r, s.auth.config.AuthCodeURL(flow.State, oidc.Nonce(flow.Nonce), oauth2.S256ChallengeOption(flow.Verifier)), http.StatusFound)
}
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	cookie, e := r.Cookie(s.cookieName("lastapp_flow"))
	if e != nil {
		writeError(w, r, problem(400, "login_expired", "Start login again."))
		return
	}
	s.cookie(w, "lastapp_flow", "", -1, true)
	b, e := base64.RawURLEncoding.DecodeString(cookie.Value)
	var flow loginFlow
	if e != nil || s.unseal("oidc_flow", "browser", "", b, &flow) != nil || time.Now().Unix()-flow.Issued > 300 || flow.Issued > time.Now().Unix()+30 || subtle.ConstantTimeCompare([]byte(flow.State), []byte(r.URL.Query().Get("state"))) != 1 {
		writeError(w, r, bad("Invalid or expired login state."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ctx = oidc.ClientContext(ctx, s.oidcClient())
	tok, e := s.auth.config.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(flow.Verifier))
	if e != nil {
		writeError(w, r, problem(401, "login_failed", "The identity provider rejected this login."))
		return
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		writeError(w, r, problem(401, "login_failed", "Provider did not return an identity token."))
		return
	}
	ident, e := s.auth.verifier.Verify(ctx, raw)
	if e != nil || subtle.ConstantTimeCompare([]byte(ident.Nonce), []byte(flow.Nonce)) != 1 {
		writeError(w, r, problem(401, "login_failed", "Invalid identity token."))
		return
	}
	var claims struct{ Name, Email string }
	if e = ident.Claims(&claims); e != nil {
		writeError(w, r, e)
		return
	}
	if claims.Name == "" {
		claims.Name = ident.Subject
	}
	if len(claims.Name) > 160 || len(claims.Email) > 320 || len(ident.Subject) > 255 || ident.Subject == "" || len(ident.Issuer) > 255 {
		writeError(w, r, bad("Provider identity exceeds supported limits."))
		return
	}
	uid := newID()
	_, e = s.DB.ExecContext(ctx, "INSERT INTO users (id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name),email=VALUES(email)", uid, ident.Issuer, ident.Subject, claims.Name, nullable(claims.Email))
	if e != nil {
		writeError(w, r, e)
		return
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		writeError(w, r, e)
		return
	}
	defer tx.Rollback()
	var state string
	e = tx.QueryRowContext(ctx, "SELECT id,state FROM users WHERE oidc_issuer=? AND oidc_subject=? FOR UPDATE", ident.Issuer, ident.Subject).Scan(&uid, &state)
	if e != nil {
		writeError(w, r, e)
		return
	}
	if state != "active" {
		writeError(w, r, problem(403, "user_disabled", "This user has been disabled."))
		return
	}
	session, csrf := token(32), token(32)
	_, e = tx.ExecContext(ctx, "INSERT INTO sessions (token_hash,user_id,csrf_hash,expires_at) VALUES (?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 8 HOUR))", hash(session), uid, hash(csrf))
	if e != nil {
		writeError(w, r, e)
		return
	}
	if e = tx.Commit(); e != nil {
		writeError(w, r, e)
		return
	}
	s.cookie(w, "lastapp_session", session, 8*3600, true)
	s.cookie(w, "lastapp_csrf", csrf, 8*3600, false)
	if flow.ReturnOrigin != "" && !s.allowedOrigin(flow.ReturnOrigin) {
		flow.ReturnOrigin = ""
	}
	http.Redirect(w, r, flow.ReturnOrigin+"/console/", http.StatusFound)
}
func (s *Server) principal(ctx context.Context, r *http.Request) (Principal, error) {
	var p Principal
	c, e := r.Cookie(s.cookieName("lastapp_session"))
	if e != nil {
		return p, problem(401, "unauthenticated", "Sign in to continue.")
	}
	e = s.DB.QueryRowContext(ctx, "SELECT u.id,u.display_name,COALESCE(u.email,'') FROM sessions se JOIN users u ON u.id=se.user_id WHERE se.token_hash=? AND se.expires_at>UTC_TIMESTAMP(6) AND u.state='active'", hash(c.Value)).Scan(&p.ID, &p.Name, &p.Email)
	if e == sql.ErrNoRows {
		return p, problem(401, "unauthenticated", "Sign in to continue.")
	}
	if e != nil {
		return p, e
	}
	p.Platform, e = isPlatform(ctx, s.DB, p.ID)
	return p, e
}
func (s *Server) csrf(ctx context.Context, r *http.Request) error {
	if !s.allowedOrigin(r.Header.Get("Origin")) {
		return problem(403, "invalid_origin", "Request origin is not allowed.")
	}
	cookie, e := r.Cookie(s.cookieName("lastapp_session"))
	if e != nil {
		return problem(401, "unauthenticated", "Sign in to continue.")
	}
	value := r.Header.Get("X-CSRF-Token")
	if value == "" {
		return problem(403, "csrf_failed", "Missing CSRF token.")
	}
	var expected []byte
	e = s.DB.QueryRowContext(ctx, "SELECT csrf_hash FROM sessions WHERE token_hash=? AND expires_at>UTC_TIMESTAMP(6)", hash(cookie.Value)).Scan(&expected)
	if e != nil || subtle.ConstantTimeCompare(expected, hash(value)) != 1 {
		return problem(403, "csrf_failed", "Invalid CSRF token.")
	}
	return nil
}
func isPlatform(ctx context.Context, q Queryer, user string) (bool, error) {
	var n int
	e := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM access_grants g JOIN users u ON u.id=g.user_id
 LEFT JOIN roles ro ON ro.id=g.role_id AND ro.state='active' LEFT JOIN role_permissions rp ON rp.role_id=ro.id
 WHERE g.kind='platform' AND g.user_id=? AND u.state='active'
 AND ((g.role_id IS NULL AND g.permission_code='platform.admin') OR rp.permission_code='platform.admin')`, user).Scan(&n)
	return n > 0, e
}
func (s *Server) member(ctx context.Context, q Queryer, p Principal, tenant string, mutating bool) error {
	var state string
	e := q.QueryRowContext(ctx, "SELECT t.state FROM tenants t JOIN tenant_memberships m ON m.tenant_id=t.id JOIN users u ON u.id=m.user_id WHERE t.id=? AND m.user_id=? AND m.state='active' AND u.state='active'", tenant, p.ID).Scan(&state)
	if e == sql.ErrNoRows {
		return missing()
	}
	if e != nil {
		return e
	}
	if mutating && state != "active" {
		return problem(403, "tenant_inactive", "This Tenant is suspended or archived.")
	}
	return nil
}
func (s *Server) scopes(ctx context.Context, q Queryer, p Principal, tenant, action string) (scopeSet, error) {
	if !controlAction(action) {
		return scopeSet{}, problem(403, "unknown_action", "Unsupported control-plane action.")
	}
	global, e := isPlatform(ctx, q, p.ID)
	if e != nil {
		return scopeSet{}, e
	}
	if global {
		return scopeSet{All: true}, nil
	}
	if e = s.member(ctx, q, p, tenant, false); e != nil {
		return scopeSet{}, e
	}
	p.Platform = false // global authority was rechecked above, inside the mutation transaction
	all, e := s.effectiveScopes(ctx, q, p, tenant)
	return all[action], e
}

func (s *Server) effectiveScopes(ctx context.Context, q Queryer, p Principal, tenant string) (map[string]scopeSet, error) {
	out := map[string]scopeSet{}
	for _, code := range []string{"tenant.read", "tenant.manage", "membership.read", "membership.manage", "authorization.read", "authorization.manage", "database.read", "database.manage", "space.read", "space.manage", "app.read", "app.manage", "microapp.read", "microapp.manage", "audit.read"} {
		out[code] = scopeSet{All: p.Platform, Spaces: []string{}, Apps: []string{}}
	}
	if p.Platform {
		return out, nil
	}
	rs, e := rows(ctx, q, `SELECT DISTINCT COALESCE(g.permission_code,rp.permission_code) AS code,g.space_id,g.app_id FROM access_grants g
 LEFT JOIN roles ro ON ro.id=g.role_id AND ro.state='active'
 LEFT JOIN role_permissions rp ON rp.role_id=ro.id
 LEFT JOIN groups team ON team.id=g.group_id AND team.state='active'
 WHERE g.tenant_id=? AND g.kind='tenant'
 AND (g.user_id=? OR (team.id IS NOT NULL AND EXISTS(SELECT 1 FROM group_memberships gm WHERE gm.tenant_id=g.tenant_id AND gm.group_id=g.group_id AND gm.user_id=?)))
 AND (g.role_id IS NULL OR ro.id IS NOT NULL)`, tenant, p.ID, p.ID)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	for _, r := range rs {
		code, space, app := str(r["code"]), str(r["space_id"]), str(r["app_id"])
		sc, ok := out[code]
		if !ok {
			continue
		}
		if space == "" && app == "" {
			sc.All = true
		}
		key := code + "/" + space + "/" + app
		if !seen[key] {
			if space != "" {
				sc.Spaces = append(sc.Spaces, space)
			}
			if app != "" {
				sc.Apps = append(sc.Apps, app)
			}
			seen[key] = true
		}
		out[code] = sc
	}
	return out, nil
}

func (s *Server) authorize(ctx context.Context, q Queryer, p Principal, action string, res Resource, mutating bool) error {
	// This is exclusively the inventory/control-plane policy boundary. Unknown
	// domains fail closed even for superadmins; business APIs must use their own
	// membership- and active-resource-bound policy, never this bypass.
	if !controlAction(action) {
		return problem(403, "unknown_action", "Unsupported control-plane action.")
	}
	if res.App != "" {
		parent, e := row(ctx, q, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", res.Tenant, res.App)
		if e != nil {
			return e
		}
		if res.Space != "" && res.Space != str(parent["space_id"]) {
			return missing()
		}
		res.Space = str(parent["space_id"])
	}
	if res.Space != "" {
		if _, e := row(ctx, q, "SELECT id FROM spaces WHERE tenant_id=? AND id=?", res.Tenant, res.Space); e != nil {
			return e
		}
	}
	global, e := isPlatform(ctx, q, p.ID)
	if e != nil {
		return e
	}
	if global {
		return nil
	}
	if action == "platform.admin" || res.Tenant == "" {
		return problem(403, "forbidden", "Platform administrator permission is required.")
	}
	if e = s.member(ctx, q, p, res.Tenant, mutating); e != nil {
		return e
	}
	allowed, e := s.scopes(ctx, q, p, res.Tenant, action)
	if e != nil {
		return e
	}
	if allowed.All {
		return nil
	}
	for _, v := range allowed.Spaces {
		if res.Space != "" && res.Space == v {
			return nil
		}
	}
	for _, v := range allowed.Apps {
		if res.App != "" && res.App == v {
			return nil
		}
	}
	return problem(403, "forbidden", "This action is not permitted in this scope.")
}
func scopeSQL(sc scopeSet, spaceColumn, appColumn string) (string, []any) {
	if sc.All {
		return "1=1", nil
	}
	clauses := []string{"1=0"}
	args := []any{}
	if spaceColumn != "" {
		for _, v := range sc.Spaces {
			clauses = append(clauses, spaceColumn+"=?")
			args = append(args, v)
		}
	}
	if appColumn != "" {
		for _, v := range sc.Apps {
			clauses = append(clauses, appColumn+"=?")
			args = append(args, v)
		}
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}
func (s *Server) Bootstrap(ctx context.Context, issuer, subject, name, email string) (string, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var locked string
	if e = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE id=? FOR UPDATE", SuperRole).Scan(&locked); e != nil {
		return "", e
	}
	uid := newID()
	_, e = tx.ExecContext(ctx, "INSERT INTO users (id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE id=id", uid, issuer, subject, name, nullable(email))
	if e != nil {
		return "", e
	}
	if e = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject=?", issuer, subject).Scan(&uid); e != nil {
		return "", e
	}
	ok, e := isPlatform(ctx, tx, uid)
	if e != nil {
		return "", e
	}
	if !ok {
		_, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,kind,user_id,role_id,role_owner_key) VALUES (?,'platform',?,?,'platform')", newID(), uid, SuperRole)
		if e != nil {
			return "", e
		}
		if e = audit(ctx, tx, "", uid, "platform.bootstrap", "user", uid, newID(), Row{}); e != nil {
			return "", e
		}
	}
	return uid, tx.Commit()
}
