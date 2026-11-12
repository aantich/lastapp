package control

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	mariadb "github.com/aantich/go-mariadb"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
)

const SuperRole = authz.SuperRole
const TenantAdminRole = authz.TenantAdminRole

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Config struct {
	DevelopmentUIOrigin string   `json:"development_ui_origin"`
	AppOrigin           string   `json:"app_origin"`
	Address             string   `json:"address"`
	PublicURL           string   `json:"public_url"`
	RegistryDSN         string   `json:"registry_dsn"`
	EncryptionKey       string   `json:"encryption_key"`
	EncryptionKeyID     string   `json:"encryption_key_id"`
	OIDCIssuer          string   `json:"oidc_issuer"`
	OIDCClientID        string   `json:"oidc_client_id"`
	OIDCClientSecret    string   `json:"oidc_client_secret"`
	Development         bool     `json:"development"`
	AllowedNetworks     []string `json:"allowed_networks"`
	RuntimeAccountHost  string   `json:"runtime_account_host"`
}

type Server struct {
	limiter      requestLimiter
	DB           *sql.DB
	Config       Config
	key          cipher.AEAD
	registryName string
	mux          *http.ServeMux
	auth         *oidcAuth
	Log          *slog.Logger
}
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

func (e *APIError) Error() string                { return e.Code + ": " + e.Message }
func problem(status int, code, msg string) error { return &APIError{status, code, msg} }
func bad(msg string) error                       { return problem(400, "invalid_request", msg) }
func missing() error                             { return problem(404, "not_found", "Resource not found or not accessible.") }
func conflict(msg string) error                  { return problem(409, "conflict", msg) }

func LoadConfig(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}
func Open(c Config) (*Server, error) {
	if !validOrigin(c.PublicURL, c.Development) {
		return nil, fmt.Errorf("public_url must be an origin (loopback in development)")
	}
	if c.DevelopmentUIOrigin != "" && (!c.Development || !validOrigin(c.DevelopmentUIOrigin, true)) {
		return nil, fmt.Errorf("development_ui_origin requires development mode and a loopback origin")
	}
	adminURL, _ := url.Parse(c.PublicURL)
	appURL, _ := url.Parse(c.AppOrigin)
	if c.AppOrigin != "" && (appURL == nil || strings.EqualFold(strings.TrimSuffix(appURL.Hostname(), "."), strings.TrimSuffix(adminURL.Hostname(), ".")) || !validOrigin(c.AppOrigin, false) || c.AppOrigin == c.PublicURL || (!c.Development && !strings.HasPrefix(c.AppOrigin, "https://"))) {
		return nil, fmt.Errorf("app_origin must be a separate secure origin")
	}
	host, _, e := net.SplitHostPort(c.Address)
	if e != nil {
		return nil, fmt.Errorf("address must include a port")
	}
	if c.Development {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("development mode must bind to a loopback IP")
		}
	} else if !strings.HasPrefix(c.PublicURL, "https://") {
		return nil, fmt.Errorf("production public_url requires HTTPS")
	}
	raw, e := base64.StdEncoding.DecodeString(c.EncryptionKey)
	if e != nil || len(raw) != 32 {
		return nil, fmt.Errorf("encryption_key must encode 32 bytes")
	}
	block, _ := aes.NewCipher(raw)
	key, _ := cipher.NewGCM(block)
	cfg, e := mariadb.ParseDSN(c.RegistryDSN)
	if e != nil {
		return nil, fmt.Errorf("invalid registry DSN")
	}
	if cfg.DBName == "" {
		return nil, fmt.Errorf("registry database is required")
	}
	if e = registryTransport(cfg, c.Development); e != nil {
		return nil, e
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.MultiStatements = false
	cfg.AllowAllFiles = false
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 15 * time.Second
	cfg.WriteTimeout = 15 * time.Second
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["time_zone"] = "'+00:00'"
	cfg.Params["sql_mode"] = "'STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'"
	connector, e := mariadb.NewConnector(cfg)
	if e != nil {
		return nil, e
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, fmt.Errorf("registry connection failed")
	}
	s := &Server{DB: db, Config: c, key: key, registryName: cfg.DBName, Log: slog.Default(), mux: http.NewServeMux()}
	return s, nil
}
func Migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrationFiles)
	if e := goose.SetDialect("mysql"); e != nil {
		return e
	}
	if e := goose.UpContext(ctx, db, "migrations"); e != nil {
		return e
	}
	return seedControlCatalog(ctx, db)
}
func CheckSchema(ctx context.Context, db *sql.DB) error {
	v, e := goose.GetDBVersionContext(ctx, db)
	if e != nil || v != 1 {
		return fmt.Errorf("registry schema missing/incompatible; run lastapp migrate")
	}
	if _, e = db.ExecContext(ctx, "SELECT scope_kind,domain FROM roles LIMIT 0"); e != nil {
		return fmt.Errorf("registry uses an earlier development schema; initialize a fresh registry")
	}
	return nil
}
func id(v string) error {
	parsed, e := uuid.Parse(v)
	if e != nil || parsed.String() != v || parsed == uuid.Nil {
		return bad("IDs must be canonical UUIDs.")
	}
	return nil
}
func newID() string { return uuid.NewString() }

var slugRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

func named(slug, name string) error {
	if !slugRE.MatchString(slug) {
		return bad("Slug must start with a letter and use lowercase letters, numbers or hyphens (maximum 63 characters).")
	}
	if strings.TrimSpace(name) == "" || len(name) > 160 {
		return bad("Name must contain 1–160 bytes.")
	}
	return nil
}
func validState(state string) bool {
	return state == "active" || state == "suspended" || state == "archived"
}
func token(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *Server) encryptionKeyID() string {
	if s.Config.EncryptionKeyID != "" {
		return s.Config.EncryptionKeyID
	}
	return "primary-v1"
}
func (s *Server) seal(purpose, record, tenant string, value any) ([]byte, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, s.key.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	out := s.key.Seal(nil, nonce, b, []byte(purpose+"|"+record+"|"+tenant))
	return json.Marshal(struct {
		Version     int    `json:"v"`
		KeyID       string `json:"key_id"`
		Nonce, Data []byte
	}{1, s.encryptionKeyID(), nonce, out})
}
func (s *Server) unseal(purpose, record, tenant string, data []byte, out any) error {
	var env struct {
		Version     int    `json:"v"`
		KeyID       string `json:"key_id"`
		Nonce, Data []byte
	}
	if e := json.Unmarshal(data, &env); e != nil {
		return fmt.Errorf("invalid encrypted envelope")
	}
	if (env.KeyID != "" && env.KeyID != s.encryptionKeyID()) || env.Version != 1 || len(env.Nonce) != s.key.NonceSize() {
		return fmt.Errorf("invalid encrypted envelope")
	}
	b, e := s.key.Open(nil, env.Nonce, env.Data, []byte(purpose+"|"+record+"|"+tenant))
	if e != nil {
		return fmt.Errorf("credential decryption failed")
	}
	return json.Unmarshal(b, out)
}
func rows(ctx context.Context, q Queryer, query string, args ...any) ([]Row, error) {
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
				if strings.HasSuffix(k, "_json") {
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
func row(ctx context.Context, q Queryer, query string, args ...any) (Row, error) {
	rs, e := rows(ctx, q, query, args...)
	if e != nil {
		return nil, e
	}
	if len(rs) == 0 {
		return nil, missing()
	}
	return rs[0], nil
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func integer(v any) int64 {
	switch n := v.(type) {
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
		i, _ := strconv.ParseInt(str(v), 10, 64)
		return i
	}
}
func decode(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return bad("Invalid or unsupported JSON request fields.")
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return bad("Request must contain one JSON object.")
	}
	return nil
}
func send(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *APIError
	var me *mariadb.MySQLError
	if errors.As(err, &ae) {
	} else if errors.As(err, &me) {
		switch me.Number {
		case 1062:
			ae = &APIError{409, "already_exists", "This ID or name already exists. Read the existing resource before retrying."}
		case 1451, 1452:
			ae = &APIError{409, "invalid_reference", "A referenced resource is missing, belongs to another Tenant, or is still in use."}
		case 4025:
			ae = &APIError{400, "invalid_request", "The request violates a resource constraint."}
		case 1205, 1213:
			ae = &APIError{409, "retry_transaction", "A concurrent change occurred; refresh and retry."}
		}
	}
	if ae == nil {
		ae = &APIError{500, "internal_error", "The operation failed. See the server log with this request ID."}
		slog.Error("request failed", append([]any{"request_id", r.Header.Get("X-Request-ID"), "operation", r.Method + " " + r.Pattern}, diagnostic(err)...)...)
	}
	send(w, ae.Status, Row{"error": Row{"code": ae.Code, "message": ae.Message, "request_id": r.Header.Get("X-Request-ID")}})
}
func expectedRevision(r *http.Request) (int64, error) {
	v := strings.Trim(r.Header.Get("If-Match"), `"`)
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 1 {
		return 0, problem(428, "revision_required", "Send If-Match with the current resource revision.")
	}
	return n, nil
}
func checkRevision(got, want int64) error {
	if got != want {
		return problem(412, "stale_revision", "This resource changed. Refresh before saving.")
	}
	return nil
}
func audit(ctx context.Context, q Queryer, tenant, user, action, kind, record, request string, details any) error {
	b, _ := json.Marshal(details)
	_, e := q.ExecContext(ctx, "INSERT INTO audit_events (tenant_id,actor_user_id,action,resource_type,resource_id,request_id,details_json) VALUES (?,?,?,?,?,?,?)", nullable(tenant), nullable(user), action, kind, nullable(record), request, string(b))
	return e
}
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func (s *Server) mutate(ctx context.Context, p Principal, action string, res Resource, request string, fn func(*sql.Tx) error) error {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if res.Tenant != "" {
		var x string
		if e = tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE id=? FOR UPDATE", res.Tenant).Scan(&x); e != nil {
			return missing()
		}
	} else {
		var x string
		if e = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE id=? FOR UPDATE", SuperRole).Scan(&x); e != nil {
			return e
		}
	}
	if e = s.authorize(ctx, tx, p, action, res, true); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit()
}
