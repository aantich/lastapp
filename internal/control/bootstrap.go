package control

import (
	"context"
	"database/sql"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

// InitAuth discovers the configured OIDC provider for console sign-in.
func (s *Server) InitAuth(ctx context.Context) error { return s.sessions.InitAuth(ctx) }

// Bootstrap makes an OIDC identity a platform superadmin. Trusted CLI and
// development seeding only, never an anonymous registration endpoint.
func (s *Server) Bootstrap(ctx context.Context, issuer, subject, name, email string) (string, error) {
	// READ COMMITTED like BootstrapLocal and op.Runner.Mutate: under the platform
	// role lock, the user lookup and platform checks read the latest commits.
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if e = lockPlatformRole(ctx, tx); e != nil {
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
	if e = grantBootstrapPlatform(ctx, tx, uid); e != nil {
		return "", e
	}
	return uid, tx.Commit()
}

// BootstrapLocal makes a local email/password account a platform superadmin.
// Trusted CLI bootstrap, never an anonymous registration endpoint.
func (s *Server) BootstrapLocal(ctx context.Context, name, email, password string) (string, error) {
	account, e := s.members.Passwords.PrepareAccount(membership.AccountInput{Name: name, Email: email, Password: password})
	if e != nil {
		return "", e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if e = lockPlatformRole(ctx, tx); e != nil {
		return "", e
	}
	user, e := membership.LocalAccount(ctx, tx, account, true)
	if e != nil {
		return "", e
	}
	if e = grantBootstrapPlatform(ctx, tx, user); e != nil {
		return "", e
	}
	return user, tx.Commit()
}

// lockPlatformRole serializes bootstrap with platform role changes.
func lockPlatformRole(ctx context.Context, tx *sql.Tx) error {
	var locked string
	return tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE id=? FOR UPDATE", access.SuperRole).Scan(&locked)
}

// grantBootstrapPlatform assigns platform superadmin to user unless it already
// holds it, with the platform candidate check and a bootstrap audit event.
func grantBootstrapPlatform(ctx context.Context, tx *sql.Tx, user string) error {
	ok, e := access.IsPlatform(ctx, tx, user)
	if e != nil || ok {
		return e
	}
	if e = roles.PlatformCandidate(ctx, tx, user); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants (id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'platform','control',?,?,'builtin')", registry.NewID(), user, access.SuperRole); e != nil {
		return e
	}
	return op.Audit(ctx, tx, "", user, "platform.bootstrap", "user", user, registry.NewID(), registry.Row{})
}
