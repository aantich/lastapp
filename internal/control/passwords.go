package control

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	passwordIterations = 600000
	minPasswordLength  = 8
	maxPasswordLength  = 128
	// Bound authentication work independently of the policy for NEW passwords.
	// Existing credentials must remain usable after a policy change.
	maxPasswordInputBytes = 1024
)
const localIssuer = "urn:lastapp:local"

func passwordLengthHint() string {
	return fmt.Sprintf("%d–%d characters.", minPasswordLength, maxPasswordLength)
}

func publicPasswordPolicy() Row {
	return Row{"min_length": minPasswordLength, "max_length": maxPasswordLength, "length_hint": passwordLengthHint()}
}

type accountInput struct {
	Name     string `json:"display_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type preparedAccount struct {
	Name, Email string
	Hash, Salt  []byte
}

func loginEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	a, e := mail.ParseAddress(email)
	if e != nil || a.Address != email || len(email) > 320 {
		return "", bad("Enter a valid email address.")
	}
	return email, nil
}

func validatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if !utf8.ValidString(password) || n < minPasswordLength || n > maxPasswordLength {
		return bad("Use a password of " + passwordLengthHint())
	}
	return nil
}

func passwordKey(password string, salt []byte, iterations int) []byte {
	key, _ := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	return key
}

func (s *Server) prepareAccount(in accountInput) (preparedAccount, error) {
	email, e := loginEmail(in.Email)
	if e != nil {
		return preparedAccount{}, e
	}
	out := preparedAccount{Name: strings.TrimSpace(in.Name), Email: email}
	if in.Password == "" {
		return out, nil
	} // Existing local account; never replace its credentials.
	if out.Name == "" || len(out.Name) > 160 {
		return out, bad("Enter the account holder's name (maximum 160 bytes).")
	}
	if e = validatePassword(in.Password); e != nil {
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
	out.Hash = passwordKey(in.Password, out.Salt, passwordIterations)
	return out, nil
}

// No email-based OIDC linking. An existing local account may be added to another
// Tenant, but its credentials and global profile are never overwritten.
func localAccount(ctx context.Context, tx *sql.Tx, in preparedAccount, canCreate bool) (string, error) {
	if len(in.Hash) > 0 && !canCreate {
		return "", problem(403, "platform_account_required", "A platform administrator must create global email/password accounts. Add an existing account instead.")
	}
	var user, state string
	e := tx.QueryRowContext(ctx, "SELECT u.id,u.state FROM local_accounts a JOIN users u ON u.id=a.user_id WHERE a.login_email=? FOR UPDATE", in.Email).Scan(&user, &state)
	if e == nil {
		if len(in.Hash) > 0 {
			return "", conflict("An account already uses this email. Select the existing account; its password has not changed.")
		}
		if state != stateActive {
			return "", bad("An active account is required.")
		}
		return user, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", e
	}
	if len(in.Hash) == 0 {
		return "", bad("A new account requires a name and initial password.")
	}
	user = newID()
	if _, e = tx.ExecContext(ctx, "INSERT INTO users(id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?)", user, localIssuer, user, in.Name, in.Email); e != nil {
		return "", e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO local_accounts(user_id,login_email,password_hash,password_salt,password_iterations) VALUES (?,?,?,?,?)", user, in.Email, in.Hash, in.Salt, passwordIterations)
	return user, e
}

func newConsoleSession(ctx context.Context, tx *sql.Tx, user, method string) (string, string, error) {
	session, csrf := token(32), token(32)
	_, e := tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_hash,expires_at,auth_method) VALUES (?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 8 HOUR),?)", hash(session), user, hash(csrf), method)
	return session, csrf, e
}

func (s *Server) sessionCookies(w http.ResponseWriter, session, csrf string) {
	s.cookie(w, "lastapp_session", session, 8*3600, true)
	s.cookie(w, "lastapp_csrf", csrf, 8*3600, false)
}

func invalidLogin() error {
	return problem(401, "invalid_credentials", "Email or password is incorrect.")
}

func (s *Server) passwordWork() (func(), error) {
	select {
	case s.passwordSlots <- struct{}{}:
		return func() { <-s.passwordSlots }, nil
	default:
		return nil, problem(429, "authentication_busy", "Password processing is busy. Retry shortly.")
	}
}

func (s *Server) createLocalUser(w http.ResponseWriter, r *http.Request, p Principal) error {
	if e := s.authorize(r.Context(), s.DB, p, "platform.admin", Resource{}, false); e != nil {
		return e
	}
	var in accountInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if in.Password == "" {
		return bad("A new account requires a name and initial password.")
	}
	account, e := s.prepareAccount(in)
	if e != nil {
		return e
	}
	var user string
	e = s.mutate(r.Context(), p, "platform.admin", Resource{}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var e error
		user, e = localAccount(r.Context(), tx, account, true)
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, "", p.ID, "user.created", "user", user, r.Header.Get("X-Request-ID"), Row{"display_name": account.Name, "email": account.Email})
	})
	if e == nil {
		send(w, 201, Row{"id": user, "display_name": account.Name, "email": account.Email, "state": stateActive})
	}
	return e
}

// Trusted CLI bootstrap, never an anonymous registration endpoint.
func (s *Server) BootstrapLocal(ctx context.Context, name, email, password string) (string, error) {
	account, e := s.prepareAccount(accountInput{Name: name, Email: email, Password: password})
	if e != nil {
		return "", e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var locked string
	if e = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE id=? FOR UPDATE", SuperRole).Scan(&locked); e != nil {
		return "", e
	}
	user, e := localAccount(ctx, tx, account, true)
	if e != nil {
		return "", e
	}
	ok, e := isPlatform(ctx, tx, user)
	if e != nil {
		return "", e
	}
	if !ok {
		if e = platformCandidate(ctx, tx, user); e != nil {
			return "", e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants(id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'platform','control',?,?,'builtin')", newID(), user, SuperRole); e != nil {
			return "", e
		}
		if e = audit(ctx, tx, "", user, "platform.bootstrap", "user", user, newID(), Row{}); e != nil {
			return "", e
		}
	}
	return user, tx.Commit()
}

func (s *Server) passwordLogin(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	e := func() error {
		if !s.allowedOrigin(r.Header.Get("Origin")) {
			return problem(403, "invalid_origin", "Request origin is not allowed.")
		}
		var in struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if e := decode(w, r, &in); e != nil {
			return e
		}
		// Forwarded addresses are used only through explicitly trusted proxy peers.
		email, e := loginEmail(in.Email)
		peer := s.loginClientIP(r)
		now := time.Now()
		if !s.loginLimiter.allowRate("ip:"+peer, now, 12, 1.0/5) || !s.loginLimiter.allowRate("email:"+hex.EncodeToString(hash(email)), now, 8, 1.0/30) {
			return rateLimited(w)
		}
		if e != nil || len(in.Password) > maxPasswordInputBytes {
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
		e = s.DB.QueryRowContext(ctx, "SELECT a.user_id,a.password_hash,a.password_salt,a.password_iterations,a.revision,u.state FROM local_accounts a JOIN users u ON u.id=a.user_id WHERE a.login_email=?", email).Scan(&user, &expected, &salt, &iterations, &revision, &state)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if errors.Is(e, sql.ErrNoRows) {
			salt, expected, iterations = make([]byte, 16), make([]byte, 32), passwordIterations
		}
		got := passwordKey(in.Password, salt, iterations)
		if subtle.ConstantTimeCompare(got, expected) != 1 || user == "" || state != stateActive {
			return invalidLogin()
		}
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
		if state != stateActive || current != revision {
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
		send(w, 200, Row{"ok": true})
		return nil
	}()
	if e != nil {
		writeError(w, r, e)
	}
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.AuthMethod != "password" {
		return bad("This account uses single sign-on. Manage its password with your identity provider.")
	}
	var in struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if len(in.Current) > maxPasswordInputBytes {
		return invalidLogin()
	}
	if e := validatePassword(in.Next); e != nil {
		return e
	}
	if in.Current == in.Next {
		return bad("Choose a different password.")
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
	if subtle.ConstantTimeCompare(passwordKey(in.Current, salt, iterations), expected) != 1 {
		return invalidLogin()
	}
	if _, e := rand.Read(salt); e != nil {
		return e
	}
	next := passwordKey(in.Next, salt, passwordIterations)
	tx, e := s.DB.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var state string
	if e = tx.QueryRowContext(r.Context(), "SELECT state FROM users WHERE id=? FOR UPDATE", p.ID).Scan(&state); e != nil {
		return e
	}
	if state != stateActive {
		return invalidLogin()
	}
	var current int64
	if e = tx.QueryRowContext(r.Context(), "SELECT revision FROM local_accounts WHERE user_id=? FOR UPDATE", p.ID).Scan(&current); e != nil {
		return e
	}
	if e = checkRevision(current, revision); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE local_accounts SET password_hash=?,password_salt=?,password_iterations=?,must_change_password=0,revision=revision+1 WHERE user_id=?", next, salt, passwordIterations, p.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", p.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE user_id=? AND state='active'", p.ID); e != nil {
		return e
	}
	session, csrf, e := newConsoleSession(r.Context(), tx, p.ID, "password")
	if e != nil {
		return e
	}
	if e = audit(r.Context(), tx, "", p.ID, "user.password_changed", "user", p.ID, r.Header.Get("X-Request-ID"), Row{}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.sessionCookies(w, session, csrf)
	send(w, 200, Row{"ok": true})
	return nil
}
