package session

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

// OIDC is a discovered identity provider: the authorization-code client and
// the ID token verifier.
type OIDC struct {
	Config   oauth2.Config
	Verifier *oidc.IDTokenVerifier
}

// loginFlow is sealed into the flow cookie between login and callback.
type loginFlow struct {
	ReturnOrigin           string
	State, Nonce, Verifier string
	Issued                 int64
}

// flowSeconds is the lifetime of an OIDC login flow.
const flowSeconds = 300

// InitAuth discovers the configured OIDC issuer. Without an issuer only local
// email/password accounts sign in.
func (m *Manager) InitAuth(ctx context.Context) error {
	if m.Config.OIDCIssuer == "" {
		return nil // Local email/password accounts do not require an OIDC provider.
	}
	if !m.Config.Development && !secureEndpoint(m.Config.OIDCIssuer) {
		return fmt.Errorf("OIDC issuer requires HTTPS")
	}
	client := m.oidcClient()
	ctx = oidc.ClientContext(ctx, client)
	provider, e := oidc.NewProvider(ctx, m.Config.OIDCIssuer)
	if e != nil {
		return fmt.Errorf("OIDC provider discovery failed: %w", e)
	}
	if !m.Config.Development {
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
	m.OIDC = &OIDC{Config: oauth2.Config{ClientID: m.Config.OIDCClientID, ClientSecret: m.Config.OIDCClientSecret, Endpoint: provider.Endpoint(), RedirectURL: m.Config.PublicURL + "/auth/callback", Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}, Verifier: provider.Verifier(&oidc.Config{ClientID: m.Config.OIDCClientID})}
	return nil
}

// Login starts an OIDC authorization-code flow with state, nonce and PKCE.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request) {
	if m.OIDC == nil {
		httpapi.WriteError(w, r, registry.Problem(404, "oidc_unavailable", "Single sign-on is not configured."))
		return
	}
	origin := r.URL.Query().Get("return_origin")
	if origin != "" && !httpapi.AllowedOrigin(m.Config, origin) {
		httpapi.WriteError(w, r, registry.Bad("Invalid return origin."))
		return
	}
	flow := loginFlow{ReturnOrigin: origin, State: registry.Token(32), Nonce: registry.Token(32), Verifier: oauth2.GenerateVerifier(), Issued: time.Now().Unix()}
	b, e := m.Sealer.Seal("oidc_flow", "browser", "", flow)
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	m.cookie(w, flowCookie, base64.RawURLEncoding.EncodeToString(b), flowSeconds, true)
	http.Redirect(w, r, m.OIDC.Config.AuthCodeURL(flow.State, oidc.Nonce(flow.Nonce), oauth2.S256ChallengeOption(flow.Verifier)), http.StatusFound)
}

// Callback completes the OIDC flow: it checks the sealed flow, state, code
// exchange and nonce, upserts the provider identity and issues a session.
func (m *Manager) Callback(w http.ResponseWriter, r *http.Request) {
	if m.OIDC == nil {
		httpapi.WriteError(w, r, registry.Problem(404, "oidc_unavailable", "Single sign-on is not configured."))
		return
	}
	cookie, e := r.Cookie(httpapi.CookieName(m.Config.PublicURL, flowCookie))
	if e != nil {
		httpapi.WriteError(w, r, registry.Problem(400, "login_expired", "Start login again."))
		return
	}
	m.cookie(w, flowCookie, "", -1, true)
	b, e := base64.RawURLEncoding.DecodeString(cookie.Value)
	var flow loginFlow
	if e != nil || m.Sealer.Unseal("oidc_flow", "browser", "", b, &flow) != nil || time.Now().Unix()-flow.Issued > flowSeconds || flow.Issued > time.Now().Unix()+30 || subtle.ConstantTimeCompare([]byte(flow.State), []byte(r.URL.Query().Get("state"))) != 1 {
		httpapi.WriteError(w, r, registry.Bad("Invalid or expired login state."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ctx = oidc.ClientContext(ctx, m.oidcClient())
	tok, e := m.OIDC.Config.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(flow.Verifier))
	if e != nil {
		httpapi.WriteError(w, r, registry.Problem(401, "login_failed", "The identity provider rejected this login."))
		return
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		httpapi.WriteError(w, r, registry.Problem(401, "login_failed", "Provider did not return an identity token."))
		return
	}
	ident, e := m.OIDC.Verifier.Verify(ctx, raw)
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
	_, e = m.DB.ExecContext(ctx, "INSERT INTO users (id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name),email=VALUES(email)", uid, ident.Issuer, ident.Subject, claims.Name, registry.Nullable(claims.Email))
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	tx, e := m.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
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
	session, csrf, e := issue(ctx, tx, uid, "oidc")
	if e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	if e = tx.Commit(); e != nil {
		httpapi.WriteError(w, r, e)
		return
	}
	m.setSessionCookies(w, session, csrf)
	if flow.ReturnOrigin != "" && !httpapi.AllowedOrigin(m.Config, flow.ReturnOrigin) {
		flow.ReturnOrigin = ""
	}
	http.Redirect(w, r, flow.ReturnOrigin+"/console/", http.StatusFound)
}

func secureEndpoint(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}

type secureTransport struct{ base http.RoundTripper }

func (t secureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if !secureEndpoint(r.URL.String()) {
		return nil, fmt.Errorf("OIDC requests require HTTPS")
	}
	return t.base.RoundTrip(r)
}

// oidcClient is the HTTP client for provider discovery, keys and code
// exchange; outside development it refuses non-HTTPS requests and redirects.
func (m *Manager) oidcClient() *http.Client {
	c := &http.Client{Timeout: 10 * time.Second}
	if !m.Config.Development {
		c.Transport = secureTransport{http.DefaultTransport}
		c.CheckRedirect = func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !secureEndpoint(r.URL.String()) {
				return fmt.Errorf("unsafe OIDC redirect")
			}
			return nil
		}
	}
	return c
}
