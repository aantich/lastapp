package session

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// Password login budgets: per client address and per (hashed) login email.
const (
	loginIPBurst     = 12
	loginIPRefill    = 1.0 / 5
	loginEmailBurst  = 8
	loginEmailRefill = 1.0 / 30
)

// LoginDatabaseTimeout bounds each database phase of a password login. The
// PBKDF2 derivation between them cannot observe a context; its cost is bounded
// by the shared membership.Passwords pool and membership.MaxPasswordInputBytes.
// A deadline spanning it only turned a completed derivation into a 500 when
// the CPU was slow (for example under the race detector on a loaded test
// machine).
var LoginDatabaseTimeout = 15 * time.Second

func invalidLogin() error {
	return registry.Problem(401, "invalid_credentials", "Email or password is incorrect.")
}

// LoginEmailKey is the login limiter key of a normalized email.
func LoginEmailKey(email string) string {
	return "email:" + hex.EncodeToString(registry.TokenHash(email))
}

// allowLogin spends the client and email login budgets.
func (m *Manager) allowLogin(peer, email string, now time.Time) bool {
	return m.LoginLimiter.AllowRate("ip:"+peer, now, loginIPBurst, loginIPRefill) && m.LoginLimiter.AllowRate(LoginEmailKey(email), now, loginEmailBurst, loginEmailRefill)
}

// PasswordLogin signs in a local account with an exact allowed Origin,
// generic invalid-credential responses and bounded per-client/per-email
// throttling.
func (m *Manager) PasswordLogin(w http.ResponseWriter, r *http.Request) {
	e := func() error {
		if !httpapi.AllowedOrigin(m.Config, r.Header.Get("Origin")) {
			return registry.Problem(403, "invalid_origin", "Request origin is not allowed.")
		}
		var in struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if e := httpapi.Decode(w, r, &in); e != nil {
			return e
		}
		// Forwarded addresses are used only through explicitly trusted proxy peers.
		email, e := membership.LoginEmail(in.Email)
		if !m.allowLogin(m.Proxies.ClientIP(r), email, time.Now()) {
			return httpapi.RateLimited(w)
		}
		if e != nil || len(in.Password) > membership.MaxPasswordInputBytes {
			return invalidLogin()
		}
		release, e := m.Passwords.Work()
		if e != nil {
			return e
		}
		defer release()
		var user, state string
		var expected, salt []byte
		var iterations int
		var revision int64
		lookup, cancelLookup := context.WithTimeout(r.Context(), LoginDatabaseTimeout)
		e = m.DB.QueryRowContext(lookup, "SELECT a.user_id,a.password_hash,a.password_salt,a.password_iterations,a.revision,u.state FROM local_accounts a JOIN users u ON u.id=a.user_id WHERE a.login_email=?", email).Scan(&user, &expected, &salt, &iterations, &revision, &state)
		cancelLookup()
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if errors.Is(e, sql.ErrNoRows) {
			salt, expected, iterations = make([]byte, 16), make([]byte, 32), membership.PasswordIterations
		}
		got := membership.PasswordKey(in.Password, salt, iterations)
		if subtle.ConstantTimeCompare(got, expected) != 1 || user == "" || state != registry.StateActive {
			return invalidLogin()
		}
		ctx, cancel := context.WithTimeout(r.Context(), LoginDatabaseTimeout)
		defer cancel()
		tx, e := m.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if e = tx.QueryRowContext(ctx, "SELECT state FROM users WHERE id=? FOR UPDATE", user).Scan(&state); e != nil {
			return e
		}
		var current int64
		if e = tx.QueryRowContext(ctx, "SELECT revision FROM local_accounts WHERE user_id=? FOR UPDATE", user).Scan(&current); e != nil {
			return e
		}
		if state != registry.StateActive || current != revision {
			return invalidLogin()
		}
		session, csrf, e := issue(ctx, tx, user, "password")
		if e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		m.setSessionCookies(w, session, csrf)
		httpapi.Send(w, 200, registry.Row{"ok": true})
		return nil
	}()
	if e != nil {
		httpapi.WriteError(w, r, e)
	}
}

// ChangePassword replaces the signed-in local account's password under the
// password policy, revokes its sessions and agent delegations and issues a
// new session/CSRF pair.
func (m *Manager) ChangePassword(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	if p.AuthMethod != "password" {
		return registry.Bad("This account uses single sign-on. Manage its password with your identity provider.")
	}
	var in struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if len(in.Current) > membership.MaxPasswordInputBytes {
		return invalidLogin()
	}
	if e := membership.ValidatePassword(in.Next); e != nil {
		return e
	}
	if in.Current == in.Next {
		return registry.Bad("Choose a different password.")
	}
	release, e := m.Passwords.Work()
	if e != nil {
		return e
	}
	defer release()
	var expected, salt []byte
	var iterations int
	var revision int64
	if e := m.DB.QueryRowContext(r.Context(), "SELECT password_hash,password_salt,password_iterations,revision FROM local_accounts WHERE user_id=?", p.ID).Scan(&expected, &salt, &iterations, &revision); e != nil {
		return e
	}
	if subtle.ConstantTimeCompare(membership.PasswordKey(in.Current, salt, iterations), expected) != 1 {
		return invalidLogin()
	}
	if _, e := rand.Read(salt); e != nil {
		return e
	}
	next := membership.PasswordKey(in.Next, salt, membership.PasswordIterations)
	ctx, cancel := op.RenewDeadline(r.Context())
	defer cancel()
	tx, e := m.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var state string
	if e = tx.QueryRowContext(ctx, "SELECT state FROM users WHERE id=? FOR UPDATE", p.ID).Scan(&state); e != nil {
		return e
	}
	if state != registry.StateActive {
		return invalidLogin()
	}
	var current int64
	if e = tx.QueryRowContext(ctx, "SELECT revision FROM local_accounts WHERE user_id=? FOR UPDATE", p.ID).Scan(&current); e != nil {
		return e
	}
	if e = op.CheckRevision(current, revision); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE local_accounts SET password_hash=?,password_salt=?,password_iterations=?,must_change_password=0,revision=revision+1 WHERE user_id=?", next, salt, membership.PasswordIterations, p.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", p.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE user_id=? AND state='active'", p.ID); e != nil {
		return e
	}
	session, csrf, e := issue(ctx, tx, p.ID, "password")
	if e != nil {
		return e
	}
	if e = op.Audit(ctx, tx, "", p.ID, "user.password_changed", "user", p.ID, r.Header.Get("X-Request-ID"), registry.Row{}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	m.setSessionCookies(w, session, csrf)
	httpapi.Send(w, 200, registry.Row{"ok": true})
	return nil
}
