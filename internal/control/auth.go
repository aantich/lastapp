package control

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/registry"
)

type Principal struct {
	ID, Name, Email        string
	Platform               bool
	AuthMethod             string
	PasswordChangeRequired bool
	Agent                  *agentGrant
}
type (
	Resource struct{ Tenant, Team, Space, App string }
	oidcAuth struct {
		config   oauth2.Config
		verifier *oidc.IDTokenVerifier
	}
)

type loginFlow struct {
	ReturnOrigin           string
	State, Nonce, Verifier string
	Issued                 int64
}
type scopeSet struct {
	All                 bool
	Spaces, Apps, Teams []string
}

func (s *Server) InitAuth(ctx context.Context) error {
	if s.Config.OIDCIssuer == "" {
		return nil // Local email/password accounts do not require an OIDC provider.
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
		if e = provider.Claims(&metadata); e != nil {
			return fmt.Errorf("OIDC provider metadata is invalid: %w", e)
		}
		if !secureEndpoint(metadata.Auth) || !secureEndpoint(metadata.Token) || !secureEndpoint(metadata.JWKS) {
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
	if s.auth == nil {
		writeError(w, r, registry.Problem(404, "oidc_unavailable", "Single sign-on is not configured."))
		return
	}
	origin := r.URL.Query().Get("return_origin")
	if origin != "" && !s.allowedOrigin(origin) {
		writeError(w, r, registry.Bad("Invalid return origin."))
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
	if s.auth == nil {
		writeError(w, r, registry.Problem(404, "oidc_unavailable", "Single sign-on is not configured."))
		return
	}
	cookie, e := r.Cookie(s.cookieName("lastapp_flow"))
	if e != nil {
		writeError(w, r, registry.Problem(400, "login_expired", "Start login again."))
		return
	}
	s.cookie(w, "lastapp_flow", "", -1, true)
	b, e := base64.RawURLEncoding.DecodeString(cookie.Value)
	var flow loginFlow
	if e != nil || s.unseal("oidc_flow", "browser", "", b, &flow) != nil || time.Now().Unix()-flow.Issued > 300 || flow.Issued > time.Now().Unix()+30 || subtle.ConstantTimeCompare([]byte(flow.State), []byte(r.URL.Query().Get("state"))) != 1 {
		writeError(w, r, registry.Bad("Invalid or expired login state."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ctx = oidc.ClientContext(ctx, s.oidcClient())
	tok, e := s.auth.config.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(flow.Verifier))
	if e != nil {
		writeError(w, r, registry.Problem(401, "login_failed", "The identity provider rejected this login."))
		return
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		writeError(w, r, registry.Problem(401, "login_failed", "Provider did not return an identity token."))
		return
	}
	ident, e := s.auth.verifier.Verify(ctx, raw)
	if e != nil || subtle.ConstantTimeCompare([]byte(ident.Nonce), []byte(flow.Nonce)) != 1 {
		writeError(w, r, registry.Problem(401, "login_failed", "Invalid identity token."))
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
		writeError(w, r, registry.Bad("Provider identity exceeds supported limits."))
		return
	}
	uid := registry.NewID()
	_, e = s.DB.ExecContext(ctx, "INSERT INTO users (id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name),email=VALUES(email)", uid, ident.Issuer, ident.Subject, claims.Name, registry.Nullable(claims.Email))
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
	if state != registry.StateActive {
		writeError(w, r, registry.Problem(403, "user_disabled", "This user has been disabled."))
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
		return p, registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	e = s.DB.QueryRowContext(ctx, "SELECT u.id,u.display_name,COALESCE(u.email,''),se.auth_method,COALESCE(la.must_change_password,0) FROM sessions se JOIN users u ON u.id=se.user_id LEFT JOIN local_accounts la ON la.user_id=u.id AND se.auth_method='password' WHERE se.token_hash=? AND se.expires_at>UTC_TIMESTAMP(6) AND se.purpose='console' AND se.state='active' AND u.state='active' AND (se.auth_method='oidc' OR la.user_id IS NOT NULL)", hash(c.Value)).Scan(&p.ID, &p.Name, &p.Email, &p.AuthMethod, &p.PasswordChangeRequired)
	if errors.Is(e, sql.ErrNoRows) {
		return p, registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	if e != nil {
		return p, e
	}
	p.Platform, e = isPlatform(ctx, s.DB, p.ID)
	return p, e
}

func (s *Server) csrf(ctx context.Context, r *http.Request) error {
	if !s.allowedOrigin(r.Header.Get("Origin")) {
		return registry.Problem(403, "invalid_origin", "Request origin is not allowed.")
	}
	cookie, e := r.Cookie(s.cookieName("lastapp_session"))
	if e != nil {
		return registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	value := r.Header.Get("X-CSRF-Token")
	if value == "" {
		return registry.Problem(403, "csrf_failed", "Missing CSRF token.")
	}
	var expected []byte
	e = s.DB.QueryRowContext(ctx, "SELECT csrf_hash FROM sessions WHERE token_hash=? AND purpose='console' AND state='active' AND expires_at>UTC_TIMESTAMP(6)", hash(cookie.Value)).Scan(&expected)
	if e != nil || subtle.ConstantTimeCompare(expected, hash(value)) != 1 {
		return registry.Problem(403, "csrf_failed", "Invalid CSRF token.")
	}
	return nil
}

func isPlatform(ctx context.Context, q registry.Queryer, user string) (bool, error) {
	var n int
	e := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM access_grants g JOIN users u ON u.id=g.user_id AND u.state='active' JOIN roles ro ON ro.id=g.role_id AND ro.state='active' WHERE g.scope_kind='platform' AND g.domain='control' AND g.user_id=? AND g.role_id=?`, user, SuperRole).Scan(&n)
	return n > 0, e
}

func (s *Server) member(ctx context.Context, q registry.Queryer, p Principal, tenant string, mutating bool) error {
	var state string
	e := q.QueryRowContext(ctx, "SELECT t.state FROM tenants t JOIN tenant_memberships m ON m.tenant_id=t.id JOIN users u ON u.id=m.user_id WHERE t.id=? AND m.user_id=? AND m.state='active' AND u.state='active'", tenant, p.ID).Scan(&state)
	if errors.Is(e, sql.ErrNoRows) {
		return registry.Missing()
	}
	if e != nil {
		return e
	}
	if mutating && state != registry.StateActive {
		return registry.Problem(403, "tenant_inactive", "This Tenant is suspended or archived.")
	}
	return nil
}

func (s *Server) scopes(ctx context.Context, q registry.Queryer, p Principal, tenant, action string) (scopeSet, error) {
	if p.Agent != nil {
		if e := s.checkAgent(ctx, q, p, Resource{Tenant: tenant}, false); e != nil {
			return scopeSet{}, e
		}
	}
	if !controlAction(action) {
		return scopeSet{}, registry.Problem(403, "unknown_action", "Unsupported control-plane action.")
	}
	global, e := isPlatform(ctx, q, p.ID)
	if e != nil {
		return scopeSet{}, e
	}
	if global {
		return s.agentScope(ctx, q, p, scopeSet{All: strings.HasSuffix(action, ".read")}), nil
	}
	if e = s.member(ctx, q, p, tenant, false); e != nil {
		return scopeSet{}, e
	}
	p.Platform = false // global authority was rechecked above, inside the mutation transaction
	all, e := s.effectiveScopes(ctx, q, p, tenant)
	return s.agentScope(ctx, q, p, all[action]), e
}

// effectiveAssignments is shared by management checks and the caller context.
// Data roles are relationships only in this foundation, never management grants.
const effectiveAssignmentSQL = `SELECT g.app_id,ro.code,ro.display_name,ro.domain,g.group_id,g.id,g.role_id,g.scope_kind,g.scope_team_id,g.space_id
 FROM access_grants g JOIN roles ro ON ro.id=g.role_id AND ro.state='active'
 JOIN users u ON u.id=? AND u.state='active'
 JOIN tenant_memberships tm ON tm.tenant_id=g.tenant_id AND tm.user_id=u.id AND tm.state='active'
 JOIN tenants t ON t.id=g.tenant_id
 LEFT JOIN groups recipient ON recipient.id=g.group_id AND recipient.tenant_id=g.tenant_id AND recipient.state='active'
 LEFT JOIN groups scoped ON scoped.id=g.scope_team_id AND scoped.tenant_id=g.tenant_id AND scoped.state='active'
 LEFT JOIN group_memberships scoped_member ON scoped_member.tenant_id=g.tenant_id AND scoped_member.group_id=scoped.id AND scoped_member.user_id=u.id AND scoped_member.state='active'
 LEFT JOIN apps a ON a.id=g.app_id AND a.tenant_id=g.tenant_id
 LEFT JOIN spaces sp ON sp.id=COALESCE(g.space_id,a.space_id) AND sp.tenant_id=g.tenant_id
 WHERE g.tenant_id=? AND (g.user_id=u.id OR (recipient.id IS NOT NULL AND EXISTS(SELECT 1 FROM group_memberships gm WHERE gm.tenant_id=g.tenant_id AND gm.group_id=recipient.id AND gm.user_id=u.id AND gm.state='active')))
 AND (g.scope_kind<>'team' OR scoped_member.user_id IS NOT NULL)
 AND (ro.domain='control' OR (t.state='active' AND sp.state='active' AND (g.app_id IS NULL OR a.state='active')))
 ORDER BY g.scope_kind,g.id`

// EffectiveAssignment is an assignment that currently applies to a user,
// directly or through a team, with its role's code, name and domain.
type EffectiveAssignment struct {
	AppID       *string `json:"app_id"`
	Code        string  `json:"code"`
	Name        string  `json:"display_name"`
	Domain      string  `json:"domain"`
	GroupID     *string `json:"group_id"`
	ID          string  `json:"id"`
	RoleID      string  `json:"role_id"`
	ScopeKind   string  `json:"scope_kind"`
	ScopeTeamID *string `json:"scope_team_id"`
	SpaceID     *string `json:"space_id"`
}

func (v *EffectiveAssignment) ScanFields() []any {
	return []any{&v.AppID, &v.Code, &v.Name, &v.Domain, &v.GroupID, &v.ID, &v.RoleID, &v.ScopeKind, &v.ScopeTeamID, &v.SpaceID}
}

// deref is the value of an optional column, or its zero value for NULL.
func deref[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}
	return *v
}

func effectiveAssignments(ctx context.Context, q registry.Queryer, user, tenant string) ([]EffectiveAssignment, error) {
	return registry.ScanAll[EffectiveAssignment](ctx, q, effectiveAssignmentSQL, user, tenant)
}

func (s *Server) effectiveScopes(ctx context.Context, q registry.Queryer, p Principal, tenant string) (map[string]scopeSet, error) {
	global, e := isPlatform(ctx, q, p.ID)
	if e != nil {
		return nil, e
	}
	out := map[string]scopeSet{}
	for _, code := range authz.Actions() {
		if code != "platform.admin" {
			out[code] = scopeSet{All: global && platformControlAction(code, Resource{Tenant: tenant}, strings.HasSuffix(code, ".manage")), Spaces: []string{}, Apps: []string{}, Teams: []string{}}
		}
	}
	if global {
		return out, nil
	}
	assignments, e := effectiveAssignments(ctx, q, p.ID, tenant)
	if e != nil {
		return nil, e
	}
	// The control catalog is closed and platform-owned; custom data roles cannot
	// contribute actions, even if their display names imitate protected roles.
	for _, g := range assignments {
		if g.Domain != authz.Control {
			continue
		}
		role, ok := authz.Builtin(g.RoleID)
		if !ok {
			continue
		}
		for _, code := range role.Permissions {
			sc, ok := out[code]
			if !ok {
				continue
			}
			switch authz.Kind(g.ScopeKind) {
			case authz.Tenant:
				sc.All = true
			case authz.Team:
				sc.Teams = appendUnique(sc.Teams, deref(g.ScopeTeamID))
			case authz.Space:
				sc.Spaces = appendUnique(sc.Spaces, deref(g.SpaceID))
			case authz.App:
				sc.Apps = appendUnique(sc.Apps, deref(g.AppID))
			}
			out[code] = sc
		}
	}
	return out, nil
}

func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

func (s *Server) authorize(ctx context.Context, q registry.Queryer, p Principal, action string, res Resource, mutating bool) error {
	if !controlAction(action) {
		return registry.Problem(403, "unknown_action", "Unsupported control-plane action.")
	}
	if res.Team != "" {
		if res.Space != "" || res.App != "" {
			return registry.Bad("Team scope cannot be combined with Space or App scope.")
		}
		var found string
		if e := registry.Lookup(ctx, q, "SELECT id FROM groups WHERE tenant_id=? AND id=?", []any{res.Tenant, res.Team}, &found); e != nil {
			return e
		}
	}
	if res.App != "" {
		var parent string
		if e := registry.Lookup(ctx, q, "SELECT space_id FROM apps WHERE tenant_id=? AND id=?", []any{res.Tenant, res.App}, &parent); e != nil {
			return e
		}
		if res.Space != "" && res.Space != parent {
			return registry.Missing()
		}
		res.Space = parent
	}
	if res.Space != "" {
		var found string
		if e := registry.Lookup(ctx, q, "SELECT id FROM spaces WHERE tenant_id=? AND id=?", []any{res.Tenant, res.Space}, &found); e != nil {
			return e
		}
	}
	if e := s.workspaceGuard(ctx, q, p, res.Space); e != nil {
		return e
	}
	if p.Agent != nil {
		if e := s.checkAgent(ctx, q, p, res, true); e != nil {
			return e
		}
	}
	global, e := isPlatform(ctx, q, p.ID)
	if e != nil {
		return e
	}
	if global {
		if platformControlAction(action, res, mutating) {
			return nil
		}
		return registry.Problem(403, "tenant_authority_required", "Tenant administrators and delegated managers manage Tenant resources.")
	}
	if action == "platform.admin" || res.Tenant == "" {
		return registry.Problem(403, "forbidden", "Platform administrator permission is required.")
	}
	if e = s.member(ctx, q, p, res.Tenant, mutating); e != nil {
		return e
	}
	all, e := s.effectiveScopes(ctx, q, p, res.Tenant)
	if e != nil {
		return e
	}
	allowed := all[action]
	if allowed.All {
		return nil
	}
	for _, v := range allowed.Teams {
		if res.Team != "" && res.Team == v {
			return nil
		}
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
	return registry.Problem(403, "forbidden", "This action is not permitted in this scope.")
}

// Platform operators administer the service and Tenant account recovery. They
// can inspect operational metadata, but never author Tenant Spaces/Apps or policy.
func platformControlAction(action string, res Resource, mutating bool) bool {
	if action == "platform.admin" {
		return true
	}
	if !mutating && strings.HasSuffix(action, ".read") {
		return true
	}
	return res.Tenant != "" && res.Team == "" && res.Space == "" && res.App == "" && (action == "membership.manage" || action == "authorization.manage")
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
	uid := registry.NewID()
	_, e = tx.ExecContext(ctx, "INSERT INTO users (id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE id=id", uid, issuer, subject, name, registry.Nullable(email))
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
		if e = platformCandidate(ctx, tx, uid); e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'platform','control',?,?,'builtin')", registry.NewID(), uid, SuperRole)
		if e != nil {
			return "", e
		}
		if e = audit(ctx, tx, "", uid, "platform.bootstrap", "user", uid, registry.NewID(), registry.Row{}); e != nil {
			return "", e
		}
	}
	return uid, tx.Commit()
}
