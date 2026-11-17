package control

import (
	"context"
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

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

type (
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
	http.SetCookie(w, &http.Cookie{Name: httpapi.CookieName(s.Config.PublicURL, name), Value: value, Path: "/", MaxAge: age, HttpOnly: httpOnly, Secure: strings.HasPrefix(s.Config.PublicURL, "https:"), SameSite: http.SameSiteLaxMode})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		httpapi.WriteError(w, r, registry.Problem(404, "oidc_unavailable", "Single sign-on is not configured."))
		return
	}
	origin := r.URL.Query().Get("return_origin")
	if origin != "" && !httpapi.AllowedOrigin(&s.Config, origin) {
		httpapi.WriteError(w, r, registry.Bad("Invalid return origin."))
		return
	}
	flow := loginFlow{ReturnOrigin: origin, State: registry.Token(32), Nonce: registry.Token(32), Verifier: oauth2.GenerateVerifier(), Issued: time.Now().Unix()}
	b, e := s.sealer.Seal("oidc_flow", "browser", "", flow)
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	s.cookie(w, "lastapp_flow", base64.RawURLEncoding.EncodeToString(b), 300, true)
	http.Redirect(w, r, s.auth.config.AuthCodeURL(flow.State, oidc.Nonce(flow.Nonce), oauth2.S256ChallengeOption(flow.Verifier)), http.StatusFound)
}

func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		httpapi.WriteError(w, r, registry.Problem(404, "oidc_unavailable", "Single sign-on is not configured."))
		return
	}
	cookie, e := r.Cookie(httpapi.CookieName(s.Config.PublicURL, "lastapp_flow"))
	if e != nil {
		httpapi.WriteError(w, r, registry.Problem(400, "login_expired", "Start login again."))
		return
	}
	s.cookie(w, "lastapp_flow", "", -1, true)
	b, e := base64.RawURLEncoding.DecodeString(cookie.Value)
	var flow loginFlow
	if e != nil || s.sealer.Unseal("oidc_flow", "browser", "", b, &flow) != nil || time.Now().Unix()-flow.Issued > 300 || flow.Issued > time.Now().Unix()+30 || subtle.ConstantTimeCompare([]byte(flow.State), []byte(r.URL.Query().Get("state"))) != 1 {
		httpapi.WriteError(w, r, registry.Bad("Invalid or expired login state."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ctx = oidc.ClientContext(ctx, s.oidcClient())
	tok, e := s.auth.config.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(flow.Verifier))
	if e != nil {
		httpapi.WriteError(w, r, registry.Problem(401, "login_failed", "The identity provider rejected this login."))
		return
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		httpapi.WriteError(w, r, registry.Problem(401, "login_failed", "Provider did not return an identity token."))
		return
	}
	ident, e := s.auth.verifier.Verify(ctx, raw)
	if e != nil || subtle.ConstantTimeCompare([]byte(ident.Nonce), []byte(flow.Nonce)) != 1 {
		httpapi.WriteError(w, r, registry.Problem(401, "login_failed", "Invalid identity token."))
		return
	}
	var claims struct{ Name, Email string }
	if e = ident.Claims(&claims); e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	if claims.Name == "" {
		claims.Name = ident.Subject
	}
	if len(claims.Name) > 160 || len(claims.Email) > 320 || len(ident.Subject) > 255 || ident.Subject == "" || len(ident.Issuer) > 255 {
		httpapi.WriteError(w, r, registry.Bad("Provider identity exceeds supported limits."))
		return
	}
	uid := registry.NewID()
	_, e = s.DB.ExecContext(ctx, "INSERT INTO users (id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name),email=VALUES(email)", uid, ident.Issuer, ident.Subject, claims.Name, registry.Nullable(claims.Email))
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	defer tx.Rollback()
	var state string
	e = tx.QueryRowContext(ctx, "SELECT id,state FROM users WHERE oidc_issuer=? AND oidc_subject=? FOR UPDATE", ident.Issuer, ident.Subject).Scan(&uid, &state)
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	if state != registry.StateActive {
		httpapi.WriteError(w, r, registry.Problem(403, "user_disabled", "This user has been disabled."))
		return
	}
	session, csrf := registry.Token(32), registry.Token(32)
	_, e = tx.ExecContext(ctx, "INSERT INTO sessions (token_hash,user_id,csrf_hash,expires_at) VALUES (?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 8 HOUR))", registry.TokenHash(session), uid, registry.TokenHash(csrf))
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	if e = tx.Commit(); e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	s.cookie(w, "lastapp_session", session, 8*3600, true)
	s.cookie(w, "lastapp_csrf", csrf, 8*3600, false)
	if flow.ReturnOrigin != "" && !httpapi.AllowedOrigin(&s.Config, flow.ReturnOrigin) {
		flow.ReturnOrigin = ""
	}
	http.Redirect(w, r, flow.ReturnOrigin+"/console/", http.StatusFound)
}

func (s *Server) principal(ctx context.Context, r *http.Request) (access.Principal, error) {
	var p access.Principal
	c, e := r.Cookie(httpapi.CookieName(s.Config.PublicURL, "lastapp_session"))
	if e != nil {
		return p, registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	e = s.DB.QueryRowContext(ctx, "SELECT u.id,u.display_name,COALESCE(u.email,''),se.auth_method,COALESCE(la.must_change_password,0) FROM sessions se JOIN users u ON u.id=se.user_id LEFT JOIN local_accounts la ON la.user_id=u.id AND se.auth_method='password' WHERE se.token_hash=? AND se.expires_at>UTC_TIMESTAMP(6) AND se.purpose='console' AND se.state='active' AND u.state='active' AND (se.auth_method='oidc' OR la.user_id IS NOT NULL)", registry.TokenHash(c.Value)).Scan(&p.ID, &p.Name, &p.Email, &p.AuthMethod, &p.PasswordChangeRequired)
	if errors.Is(e, sql.ErrNoRows) {
		return p, registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	if e != nil {
		return p, e
	}
	p.Platform, e = access.IsPlatform(ctx, s.DB, p.ID)
	return p, e
}

func (s *Server) csrf(ctx context.Context, r *http.Request) error {
	if !httpapi.AllowedOrigin(&s.Config, r.Header.Get("Origin")) {
		return registry.Problem(403, "invalid_origin", "Request origin is not allowed.")
	}
	cookie, e := r.Cookie(httpapi.CookieName(s.Config.PublicURL, "lastapp_session"))
	if e != nil {
		return registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	value := r.Header.Get("X-CSRF-Token")
	if value == "" {
		return registry.Problem(403, "csrf_failed", "Missing CSRF token.")
	}
	var expected []byte
	e = s.DB.QueryRowContext(ctx, "SELECT csrf_hash FROM sessions WHERE token_hash=? AND purpose='console' AND state='active' AND expires_at>UTC_TIMESTAMP(6)", registry.TokenHash(cookie.Value)).Scan(&expected)
	if e != nil || subtle.ConstantTimeCompare(expected, registry.TokenHash(value)) != 1 {
		return registry.Problem(403, "csrf_failed", "Invalid CSRF token.")
	}
	return nil
}

func (s *Server) Bootstrap(ctx context.Context, issuer, subject, name, email string) (string, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var locked string
	if e = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE id=? FOR UPDATE", access.SuperRole).Scan(&locked); e != nil {
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
	ok, e := access.IsPlatform(ctx, tx, uid)
	if e != nil {
		return "", e
	}
	if !ok {
		if e = roles.PlatformCandidate(ctx, tx, uid); e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'platform','control',?,?,'builtin')", registry.NewID(), uid, access.SuperRole)
		if e != nil {
			return "", e
		}
		if e = op.Audit(ctx, tx, "", uid, "platform.bootstrap", "user", uid, registry.NewID(), registry.Row{}); e != nil {
			return "", e
		}
	}
	return uid, tx.Commit()
}
