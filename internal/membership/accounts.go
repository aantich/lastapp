package membership

import (
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

const (
	PasswordIterations = 600000
	MinPasswordLength  = 8
	MaxPasswordLength  = 128
	// Bound authentication work independently of the policy for NEW passwords.
	// Existing credentials must remain usable after a policy change.
	MaxPasswordInputBytes = 1024
)

const localIssuer = "urn:lastapp:local"

func PasswordLengthHint() string {
	return fmt.Sprintf("%d–%d characters.", MinPasswordLength, MaxPasswordLength)
}

func PublicPasswordPolicy() registry.Row {
	return registry.Row{"min_length": MinPasswordLength, "max_length": MaxPasswordLength, "length_hint": PasswordLengthHint()}
}

type AccountInput struct {
	Name     string `json:"display_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type PreparedAccount struct {
	Name, Email string
	Hash, Salt  []byte
}

// Deadline renews the request deadline after prepareAccount derived a
// password, so the derivation does not consume the database deadline. Without
// a derivation the request keeps its original deadline.
func (a PreparedAccount) Deadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.Hash == nil {
		return ctx, func() {}
	}
	return op.RenewDeadline(ctx)
}

func LoginEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	a, e := mail.ParseAddress(email)
	if e != nil || a.Address != email || len(email) > 320 {
		return "", registry.Bad("Enter a valid email address.")
	}
	return email, nil
}

func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if !utf8.ValidString(password) || n < MinPasswordLength || n > MaxPasswordLength {
		return registry.Bad("Use a password of " + PasswordLengthHint())
	}
	return nil
}

func PasswordKey(password string, salt []byte, iterations int) []byte {
	key, _ := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	return key
}

// No email-based OIDC linking. An existing local account may be added to another
// Tenant, but its credentials and global profile are never overwritten.
func LocalAccount(ctx context.Context, tx *sql.Tx, in PreparedAccount, canCreate bool) (string, error) {
	if len(in.Hash) > 0 && !canCreate {
		return "", registry.Problem(403, "platform_account_required", "A platform administrator must create global email/password accounts. Add an existing account instead.")
	}
	var user, state string
	e := tx.QueryRowContext(ctx, "SELECT u.id,u.state FROM local_accounts a JOIN users u ON u.id=a.user_id WHERE a.login_email=? FOR UPDATE", in.Email).Scan(&user, &state)
	if e == nil {
		if len(in.Hash) > 0 {
			return "", registry.Conflict("An account already uses this email. Select the existing account; its password has not changed.")
		}
		if state != registry.StateActive {
			return "", registry.Bad("An active account is required.")
		}
		return user, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", e
	}
	if len(in.Hash) == 0 {
		return "", registry.Bad("A new account requires a name and initial password.")
	}
	user = registry.NewID()
	if _, e = tx.ExecContext(ctx, "INSERT INTO users(id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?)", user, localIssuer, user, in.Name, in.Email); e != nil {
		return "", e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO local_accounts(user_id,login_email,password_hash,password_salt,password_iterations) VALUES (?,?,?,?,?)", user, in.Email, in.Hash, in.Salt, PasswordIterations)
	return user, e
}
