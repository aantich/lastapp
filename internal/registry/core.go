package registry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Row map[string]any

type APIError struct {
	Status        int
	Code, Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func Problem(status int, code, msg string) error { return &APIError{status, code, msg} }

func Bad(msg string) error { return Problem(400, "invalid_request", msg) }

func Missing() error { return Problem(404, "not_found", "Resource not found or not accessible.") }

func Conflict(msg string) error { return Problem(409, "conflict", msg) }

func ID(v string) error {
	parsed, e := uuid.Parse(v)
	if e != nil || parsed.String() != v || parsed == uuid.Nil {
		return Bad("IDs must be canonical UUIDs.")
	}
	return nil
}

func NewID() string { return uuid.NewString() }

var slugRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

func Named(slug, name string) error {
	if !slugRE.MatchString(slug) {
		return Bad("Slug must start with a letter and use lowercase letters, numbers or hyphens (maximum 63 characters).")
	}
	if strings.TrimSpace(name) == "" || len(name) > 160 {
		return Bad("Name must contain 1–160 bytes.")
	}
	return nil
}

// Record lifecycle states shared by registry tables. The migrations' CHECK
// constraints define which states each table allows; Go comparisons and written
// values use these names, while static SQL predicates keep the readable literal.
const (
	StateActive    = "active"
	StateDisabled  = "disabled"
	stateSuspended = "suspended"
	StateArchived  = "archived"
	StateRetired   = "retired"
	StateRevoked   = "revoked"
)

// EnabledState reports whether state is valid for active/disabled records
// such as users, memberships, groups, roles and database targets.
func EnabledState(state string) bool {
	return state == StateActive || state == StateDisabled
}

func ValidState(state string) bool {
	return state == StateActive || state == stateSuspended || state == StateArchived
}

func QueryRows(ctx context.Context, q Queryer, query string, args ...any) ([]Row, error) {
	r, e := q.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	cols, e := r.Columns()
	if e != nil {
		return nil, e
	}
	out := []Row{}
	for r.Next() {
		vals := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		if e = r.Scan(ptr...); e != nil {
			return nil, e
		}
		v := Row{}
		for i, k := range cols {
			switch t := vals[i].(type) {
			case []byte:
				if strings.HasSuffix(k, "_json") && !strings.HasPrefix(k, "f_") {
					var obj any
					if json.Unmarshal(t, &obj) == nil {
						v[k] = obj
					} else {
						v[k] = string(t)
					}
				} else {
					v[k] = string(t)
				}
			default:
				v[k] = t
			}
		}
		out = append(out, v)
	}
	return out, r.Err()
}

func QueryRow(ctx context.Context, q Queryer, query string, args ...any) (Row, error) {
	rs, e := QueryRows(ctx, q, query, args...)
	if e != nil {
		return nil, e
	}
	if len(rs) == 0 {
		return nil, Missing()
	}
	return rs[0], nil
}

func Str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func Integer(v any) int64 {
	switch n := v.(type) {
	case bool:
		if n {
			return 1
		}
		return 0
	case int64:
		return n
	case uint64:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		i, _ := strconv.ParseInt(Str(v), 10, 64)
		return i
	}
}

func Nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func Token(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// TokenHash is the stored SHA-256 digest of a secret token: console and App
// session tokens, launch codes, upload tickets and agent OAuth codes/tokens.
func TokenHash(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }

// PKCE formats shared by App launch codes and agent OAuth: PKCEVerifier is an
// RFC 7636 code verifier, PKCEChallenge an S256 challenge (also used for the
// launch nonce).
var (
	PKCEVerifier  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	PKCEChallenge = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

// PKCEMatches reports in constant time whether verifier hashes to the S256
// challenge (RFC 7636: BASE64URL(SHA256(verifier)) without padding). App
// launch exchange and agent OAuth code exchange both check it.
func PKCEMatches(verifier, challenge string) bool {
	return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(TokenHash(verifier))), []byte(challenge)) == 1
}

// LockedRecord is the revision and lifecycle state of a record locked for
// update.
type LockedRecord struct {
	Revision int64
	State    string
}

func LockRecord(ctx context.Context, tx *sql.Tx, table, tenant, record string) (LockedRecord, error) {
	query := "SELECT revision,state FROM " + table + " WHERE id=?"
	args := []any{record}
	if tenant != "" {
		query += " AND tenant_id=?"
		args = append(args, tenant)
	}
	var v LockedRecord
	return v, Lookup(ctx, tx, query+" FOR UPDATE", args, &v.Revision, &v.State)
}
