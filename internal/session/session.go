// Package session owns console sessions: password and OIDC login, the OIDC
// flow cookie, session issue and lookup, principal resolution, CSRF and
// Origin checks for console writes, logout and password change. It is
// HTTP-facing like internal/dataapi; internal/control registers its routes.
package session

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/sealer"
)

const (
	// flowCookie carries the sealed OIDC login flow between login and callback.
	flowCookie = "lastapp_flow"
	// sessionSeconds is the lifetime of a console session and its cookies.
	sessionSeconds = 8 * 3600
)

// Manager issues, resolves and ends console sessions. It is wired in
// control.Open; Config and Proxies point at the server's own values.
type Manager struct {
	DB        *sql.DB
	Config    *config.Config
	Sealer    *sealer.Sealer
	Passwords *membership.Passwords
	Proxies   *httpapi.TrustedProxies
	// LoginLimiter bounds password login attempts per client and per email.
	LoginLimiter httpapi.Limiter
	// OIDC is set by InitAuth when an issuer is configured; nil disables
	// single sign-on.
	OIDC *OIDC
}

func (m *Manager) cookie(w http.ResponseWriter, name, value string, age int, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{Name: httpapi.CookieName(m.Config.PublicURL, name), Value: value, Path: "/", MaxAge: age, HttpOnly: httpOnly, Secure: strings.HasPrefix(m.Config.PublicURL, "https:"), SameSite: http.SameSiteLaxMode})
}

// issue creates a console session for user in tx and returns the session and
// CSRF tokens; only their hashes are stored.
func issue(ctx context.Context, tx *sql.Tx, user, method string) (string, string, error) {
	session, csrf := registry.Token(32), registry.Token(32)
	_, e := tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_hash,expires_at,auth_method) VALUES (?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 8 HOUR),?)", registry.TokenHash(session), user, registry.TokenHash(csrf), method)
	return session, csrf, e
}

func (m *Manager) setSessionCookies(w http.ResponseWriter, session, csrf string) {
	m.cookie(w, httpapi.SessionCookie, session, sessionSeconds, true)
	m.cookie(w, httpapi.CSRFCookie, csrf, sessionSeconds, false)
}

// Principal resolves the live console session cookie to its active user.
func (m *Manager) Principal(ctx context.Context, r *http.Request) (access.Principal, error) {
	var p access.Principal
	c, e := r.Cookie(httpapi.CookieName(m.Config.PublicURL, httpapi.SessionCookie))
	if e != nil {
		return p, registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	e = m.DB.QueryRowContext(ctx, "SELECT u.id,u.display_name,COALESCE(u.email,''),se.auth_method,COALESCE(la.must_change_password,0) FROM sessions se JOIN users u ON u.id=se.user_id LEFT JOIN local_accounts la ON la.user_id=u.id AND se.auth_method='password' WHERE se.token_hash=? AND se.expires_at>UTC_TIMESTAMP(6) AND se.purpose='console' AND se.state='active' AND u.state='active' AND (se.auth_method='oidc' OR la.user_id IS NOT NULL)", registry.TokenHash(c.Value)).Scan(&p.ID, &p.Name, &p.Email, &p.AuthMethod, &p.PasswordChangeRequired)
	if errors.Is(e, sql.ErrNoRows) {
		return p, registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	if e != nil {
		return p, e
	}
	p.Platform, e = access.IsPlatform(ctx, m.DB, p.ID)
	return p, e
}

// CSRF checks a console write: the exact allowed Origin and the session's
// CSRF token.
func (m *Manager) CSRF(ctx context.Context, r *http.Request) error {
	if !httpapi.AllowedOrigin(m.Config, r.Header.Get("Origin")) {
		return registry.Problem(403, "invalid_origin", "Request origin is not allowed.")
	}
	cookie, e := r.Cookie(httpapi.CookieName(m.Config.PublicURL, httpapi.SessionCookie))
	if e != nil {
		return registry.Problem(401, "unauthenticated", "Sign in to continue.")
	}
	value := r.Header.Get("X-CSRF-Token")
	if value == "" {
		return registry.Problem(403, "csrf_failed", "Missing CSRF token.")
	}
	var expected []byte
	e = m.DB.QueryRowContext(ctx, "SELECT csrf_hash FROM sessions WHERE token_hash=? AND purpose='console' AND state='active' AND expires_at>UTC_TIMESTAMP(6)", registry.TokenHash(cookie.Value)).Scan(&expected)
	if e != nil || subtle.ConstantTimeCompare(expected, registry.TokenHash(value)) != 1 {
		return registry.Problem(403, "csrf_failed", "Invalid CSRF token.")
	}
	return nil
}

// Logout deletes the current session and clears its cookies.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request, _ access.Principal) error {
	c, _ := r.Cookie(httpapi.CookieName(m.Config.PublicURL, httpapi.SessionCookie))
	_, e := m.DB.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", registry.TokenHash(c.Value))
	if e != nil {
		return e
	}
	m.cookie(w, httpapi.SessionCookie, "", -1, true)
	m.cookie(w, httpapi.CSRFCookie, "", -1, false)
	httpapi.Send(w, 200, registry.Row{"ok": true})
	return nil
}
