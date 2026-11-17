package control

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

func (s *Server) prepareAccount(in membership.AccountInput) (membership.PreparedAccount, error) {
	email, e := membership.LoginEmail(in.Email)
	if e != nil {
		return membership.PreparedAccount{}, e
	}
	out := membership.PreparedAccount{Name: strings.TrimSpace(in.Name), Email: email}
	if in.Password == "" {
		return out, nil
	} // Existing local account; never replace its credentials.
	if out.Name == "" || len(out.Name) > 160 {
		return out, registry.Bad("Enter the account holder's name (maximum 160 bytes).")
	}
	if e = membership.ValidatePassword(in.Password); e != nil {
		return out, e
	}
	release, e := s.passwordWork()
	if e != nil {
		return out, e
	}
	defer release()
	out.Salt = make([]byte, 16)
	if _, e = rand.Read(out.Salt); e != nil {
		return out, e
	}
	out.Hash = membership.PasswordKey(in.Password, out.Salt, membership.PasswordIterations)
	return out, nil
}

func newConsoleSession(ctx context.Context, tx *sql.Tx, user, method string) (string, string, error) {
	session, csrf := registry.Token(32), registry.Token(32)
	_, e := tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_hash,expires_at,auth_method) VALUES (?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 8 HOUR),?)", registry.TokenHash(session), user, registry.TokenHash(csrf), method)
	return session, csrf, e
}

func (s *Server) sessionCookies(w http.ResponseWriter, session, csrf string) {
	s.cookie(w, "lastapp_session", session, 8*3600, true)
	s.cookie(w, "lastapp_csrf", csrf, 8*3600, false)
}

func invalidLogin() error {
	return registry.Problem(401, "invalid_credentials", "Email or password is incorrect.")
}

func (s *Server) passwordWork() (func(), error) {
	select {
	case s.passwordSlots <- struct{}{}:
		return func() { <-s.passwordSlots }, nil
	default:
		return nil, registry.Problem(429, "authentication_busy", "Password processing is busy. Retry shortly.")
	}
}

func (s *Server) createLocalUser(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	if e := s.Authorize(r.Context(), s.DB, p, "platform.admin", access.Resource{}, false); e != nil {
		return e
	}
	var in membership.AccountInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if in.Password == "" {
		return registry.Bad("A new account requires a name and initial password.")
	}
	account, e := s.prepareAccount(in)
	if e != nil {
		return e
	}
	ctx, cancel := op.RenewDeadline(r.Context())
	defer cancel()
	var user string
	e = s.mutate(ctx, p, "platform.admin", access.Resource{}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var e error
		user, e = membership.LocalAccount(ctx, tx, account, true)
		if e != nil {
			return e
		}
		return op.Audit(ctx, tx, "", p.ID, "user.created", "user", user, r.Header.Get("X-Request-ID"), registry.Row{"display_name": account.Name, "email": account.Email})
	})
	if e == nil {
		httpapi.Send(w, 201, registry.Row{"id": user, "display_name": account.Name, "email": account.Email, "state": registry.StateActive})
	}
	return e
}

// Trusted CLI bootstrap, never an anonymous registration endpoint.
func (s *Server) BootstrapLocal(ctx context.Context, name, email, password string) (string, error) {
	account, e := s.prepareAccount(membership.AccountInput{Name: name, Email: email, Password: password})
	if e != nil {
		return "", e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var locked string
	if e = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE id=? FOR UPDATE", access.SuperRole).Scan(&locked); e != nil {
		return "", e
	}
	user, e := membership.LocalAccount(ctx, tx, account, true)
	if e != nil {
		return "", e
	}
	ok, e := access.IsPlatform(ctx, tx, user)
	if e != nil {
		return "", e
	}
	if !ok {
		if e = roles.PlatformCandidate(ctx, tx, user); e != nil {
			return "", e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants(id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'platform','control',?,?,'builtin')", registry.NewID(), user, access.SuperRole); e != nil {
			return "", e
		}
		if e = op.Audit(ctx, tx, "", user, "platform.bootstrap", "user", user, registry.NewID(), registry.Row{}); e != nil {
			return "", e
		}
	}
	return user, tx.Commit()
}

// loginDatabaseTimeout bounds each database phase of a password login. The
// PBKDF2 derivation between them cannot observe a context; its cost is bounded
// by passwordSlots and maxPasswordInputBytes. A deadline spanning it only
// turned a completed derivation into a 500 when the CPU was slow (for example
// under the race detector on a loaded test machine).
var loginDatabaseTimeout = 15 * time.Second

func (s *Server) passwordLogin(w http.ResponseWriter, r *http.Request) {
	e := func() error {
		if !httpapi.AllowedOrigin(&s.Config, r.Header.Get("Origin")) {
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
		peer := s.trustedProxies.ClientIP(r)
		now := time.Now()
		if !s.loginLimiter.AllowRate("ip:"+peer, now, 12, 1.0/5) || !s.loginLimiter.AllowRate("email:"+hex.EncodeToString(registry.TokenHash(email)), now, 8, 1.0/30) {
			return httpapi.RateLimited(w)
		}
		if e != nil || len(in.Password) > membership.MaxPasswordInputBytes {
			return invalidLogin()
		}
		release, e := s.passwordWork()
		if e != nil {
			return e
		}
		defer release()
		var user, state string
		var expected, salt []byte
		var iterations int
		var revision int64
		lookup, cancelLookup := context.WithTimeout(r.Context(), loginDatabaseTimeout)
		e = s.DB.QueryRowContext(lookup, "SELECT a.user_id,a.password_hash,a.password_salt,a.password_iterations,a.revision,u.state FROM local_accounts a JOIN users u ON u.id=a.user_id WHERE a.login_email=?", email).Scan(&user, &expected, &salt, &iterations, &revision, &state)
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
		ctx, cancel := context.WithTimeout(r.Context(), loginDatabaseTimeout)
		defer cancel()
		tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
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
		session, csrf, e := newConsoleSession(ctx, tx, user, "password")
		if e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		s.sessionCookies(w, session, csrf)
		httpapi.Send(w, 200, registry.Row{"ok": true})
		return nil
	}()
	if e != nil {
		httpapi.WriteError(w, r, e)
	}
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, p access.Principal) error {
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
	release, e := s.passwordWork()
	if e != nil {
		return e
	}
	defer release()
	var expected, salt []byte
	var iterations int
	var revision int64
	if e := s.DB.QueryRowContext(r.Context(), "SELECT password_hash,password_salt,password_iterations,revision FROM local_accounts WHERE user_id=?", p.ID).Scan(&expected, &salt, &iterations, &revision); e != nil {
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
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
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
	session, csrf, e := newConsoleSession(ctx, tx, p.ID, "password")
	if e != nil {
		return e
	}
	if e = op.Audit(ctx, tx, "", p.ID, "user.password_changed", "user", p.ID, r.Header.Get("X-Request-ID"), registry.Row{}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.sessionCookies(w, session, csrf)
	httpapi.Send(w, 200, registry.Row{"ok": true})
	return nil
}
