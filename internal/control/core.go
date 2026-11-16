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
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	mariadb "github.com/aantich/go-mariadb"
	"github.com/pressly/goose/v3"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Config struct {
	WorkspaceDatabaseTarget string        `json:"workspace_database_target_id"`
	ArtifactDirectory       string        `json:"artifact_directory"`
	AgentClients            []AgentClient `json:"agent_clients"`
	DevelopmentUIOrigin     string        `json:"development_ui_origin"`
	AppOrigin               string        `json:"app_origin"`
	Address                 string        `json:"address"`
	PublicURL               string        `json:"public_url"`
	RegistryDSN             string        `json:"registry_dsn"`
	EncryptionKey           string        `json:"encryption_key"`
	EncryptionKeyID         string        `json:"encryption_key_id"`
	OIDCIssuer              string        `json:"oidc_issuer"`
	OIDCClientID            string        `json:"oidc_client_id"`
	OIDCClientSecret        string        `json:"oidc_client_secret"`
	Development             bool          `json:"development"`
	AllowedNetworks         []string      `json:"allowed_networks"`
	TrustedProxies          []string      `json:"trusted_proxies"`
	RuntimeAccountHost      string        `json:"runtime_account_host"`
}

type Server struct {
	*access.Checker
	uploadSlots            chan struct{}
	agentSlots             chan struct{}
	agentBootstrapSlots    chan struct{}
	agentTokenSlots        chan struct{}
	agentValidationSlots   chan struct{}
	agentBootstrapLimiter  requestLimiter
	agentValidationLimiter requestLimiter
	tenantDataSlots        inFlightTenants
	dataSlots              chan struct{}
	dataPools              data.Pools
	definitions            definitionCache
	limiter                requestLimiter
	loginLimiter           requestLimiter
	passwordSlots          chan struct{}
	trustedProxies         []netip.Prefix
	DB                     *sql.DB
	Config                 Config
	key                    cipher.AEAD
	registryName           string
	mux                    *http.ServeMux
	auth                   *oidcAuth
	Log                    *slog.Logger
	run                    *runner
	inv                    *inventory
	members                *membership
	authority              *authority
	storage                *storage
	authoring              *authoring
}

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
	if c.WorkspaceDatabaseTarget != "" && registry.ID(c.WorkspaceDatabaseTarget) != nil {
		return nil, fmt.Errorf("invalid workspace_database_target_id")
	}
	seenClients := map[string]bool{}
	for _, client := range c.AgentClients {
		if client.ID == "" || len(client.ID) > 500 || client.Name == "" || len(client.Name) > 160 || seenClients[client.ID] || len(client.Redirects) == 0 || len(client.Redirects) > agentMaxClientRedirects {
			return nil, fmt.Errorf("invalid or duplicate agent client registration")
		}
		seenClients[client.ID] = true
		for _, redirect := range client.Redirects {
			if !validAgentRedirect(redirect) {
				return nil, fmt.Errorf("invalid agent client redirect")
			}
		}
	}
	proxies, e := parseTrustedProxies(c.TrustedProxies)
	if e != nil {
		return nil, e
	}
	if !validOrigin(c.PublicURL, c.Development) {
		return nil, fmt.Errorf("public_url must be an origin (loopback in development)")
	}
	if c.DevelopmentUIOrigin != "" && (!c.Development || !validOrigin(c.DevelopmentUIOrigin, true)) {
		return nil, fmt.Errorf("development_ui_origin requires development mode and a loopback origin")
	}
	adminURL, _ := url.Parse(c.PublicURL)
	appURL, _ := url.Parse(c.AppOrigin)
	appNamespace := appHostNamespace(c.AppOrigin)
	adminHost := strings.TrimSuffix(strings.ToLower(adminURL.Hostname()), ".")
	if c.AppOrigin != "" && (appURL == nil || !validOrigin(c.AppOrigin, false) || adminHost == strings.TrimPrefix(appNamespace, ".") || appNamespace != "" && strings.HasSuffix(adminHost, appNamespace) || (!c.Development && !strings.HasPrefix(c.AppOrigin, "https://"))) {
		return nil, fmt.Errorf("app_origin must use a separate secure host namespace that does not contain public_url")
	}
	host, _, e := net.SplitHostPort(c.Address)
	if e != nil {
		return nil, fmt.Errorf("address must include a port: %w", e)
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
	if e != nil {
		// base64 errors report only an input offset, never key material.
		return nil, fmt.Errorf("encryption_key must encode 32 bytes: %w", e)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("encryption_key must encode 32 bytes")
	}
	block, e := aes.NewCipher(raw)
	if e != nil {
		return nil, fmt.Errorf("encryption_key cipher setup failed: %w", e)
	}
	key, e := cipher.NewGCM(block)
	if e != nil {
		return nil, fmt.Errorf("encryption_key cipher setup failed: %w", e)
	}
	cfg, e := mariadb.ParseDSN(c.RegistryDSN)
	if e != nil {
		// Deliberately not wrapped: DSN parser messages can echo DSN fragments.
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
		return nil, fmt.Errorf("registry connection failed: %w", e)
	}
	s := &Server{uploadSlots: make(chan struct{}, 2), DB: db, Config: c, key: key, registryName: cfg.DBName, Log: slog.Default(), mux: http.NewServeMux(), passwordSlots: make(chan struct{}, 2), dataSlots: make(chan struct{}, entity.Limits.DataRequests), trustedProxies: proxies, agentSlots: make(chan struct{}, agentMaxConcurrent), agentBootstrapSlots: make(chan struct{}, agentBootstrapConcurrent), agentTokenSlots: make(chan struct{}, agentTokenConcurrent), agentValidationSlots: make(chan struct{}, agentValidationConcurrent)}
	s.Checker = &access.Checker{Audience: s.mcpURL()}
	s.run = &runner{db: db, access: s.Checker}
	s.authoring = &authoring{db: db, run: s.run, config: &s.Config, queue: s.queueDefinition, actorContext: s.actorContext}
	s.storage = &storage{db: db, run: s.run, seal: s.seal, config: &s.Config, registryName: cfg.DBName}
	s.inv = &inventory{db: db, run: s.run, allocateDatabase: s.storage.allocate, prepareAccount: s.prepareAccount}
	s.members = &membership{db: db, run: s.run, prepareAccount: s.prepareAccount}
	s.authority = &authority{db: db, run: s.run}
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

// latestMigrationVersion derives the required registry version from the
// embedded migrations, so adding a migration needs no second edit here.
func latestMigrationVersion() (int64, error) {
	names, e := fs.Glob(migrationFiles, "migrations/*.sql")
	if e != nil {
		return 0, fmt.Errorf("list registry migrations: %w", e)
	}
	var latest int64
	for _, name := range names {
		v, e := goose.NumericComponent(name)
		if e != nil {
			return 0, fmt.Errorf("registry migration %s: %w", name, e)
		}
		latest = max(latest, v)
	}
	if latest == 0 {
		return 0, errors.New("no embedded registry migrations")
	}
	return latest, nil
}

func CheckSchema(ctx context.Context, db *sql.DB) error {
	v, e := goose.GetDBVersionContext(ctx, db)
	if e != nil {
		return fmt.Errorf("registry schema missing/incompatible; run lastapp migrate: %w", e)
	}
	want, e := latestMigrationVersion()
	if e != nil {
		return e
	}
	if v != want {
		return fmt.Errorf("registry schema missing/incompatible; run lastapp migrate")
	}
	if _, e = db.ExecContext(ctx, "SELECT scope_kind,domain FROM roles LIMIT 0"); e != nil {
		return fmt.Errorf("registry uses an earlier development schema; initialize a fresh registry: %w", e)
	}
	return nil
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
		return fmt.Errorf("invalid encrypted envelope: %w", e)
	}
	if (env.KeyID != "" && env.KeyID != s.encryptionKeyID()) || env.Version != 1 || len(env.Nonce) != s.key.NonceSize() {
		return fmt.Errorf("invalid encrypted envelope")
	}
	b, e := s.key.Open(nil, env.Nonce, env.Data, []byte(purpose+"|"+record+"|"+tenant))
	if e != nil {
		return fmt.Errorf("credential decryption failed: %w", e)
	}
	return json.Unmarshal(b, out)
}

func decode(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return registry.Bad("Invalid or unsupported JSON request fields.")
	}
	var extra any
	if e := d.Decode(&extra); !errors.Is(e, io.EOF) {
		return registry.Bad("Request must contain one JSON object.")
	}
	return nil
}

func send(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Headers are already sent; a failed write means the client went away.
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	o := classify(err)
	if o.Unmapped {
		fields := []any{"operation", r.Method + " " + r.Pattern}
		if rid := r.Header.Get("X-Request-ID"); rid != requestIDFrom(r.Context()) {
			// MCP tool calls run operations under their own request IDs.
			fields = append(fields, "operation_request_id", rid)
		}
		requestLogger(r.Context()).Error("request failed", append(fields, diagnostic(err)...)...)
	}
	if o.Status == 429 {
		w.Header().Set("Retry-After", "1")
	}
	send(w, o.Status, registry.Row{"error": o.payload(r.Header.Get("X-Request-ID"))})
}

func expectedRevision(r *http.Request) (int64, error) {
	v := strings.Trim(r.Header.Get("If-Match"), `"`)
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 1 {
		return 0, revisionRequired()
	}
	return n, nil
}

// ifMatch returns the If-Match revision, or 0 for the operation to reject.
func ifMatch(r *http.Request) int64 {
	n, _ := expectedRevision(r)
	return n
}

func checkRevision(got, want int64) error {
	if got != want {
		return registry.Problem(412, "stale_revision", "This resource changed. Refresh before saving.")
	}
	return nil
}

// audit is the context shim of auditAs for code not yet migrated to actors.
func audit(ctx context.Context, q registry.Queryer, tenant, user, action, kind, record, request string, details any) error {
	return auditAs(ctx, q, Actor{Principal: access.Principal{ID: user}, RequestID: request, Agent: access.AgentCallFrom(ctx)}, tenant, action, kind, record, details)
}

// mutate is the context shim of runner.mutate for handlers not yet migrated
// to operations with an explicit Actor.
func (s *Server) mutate(ctx context.Context, p access.Principal, action string, res access.Resource, request string, fn func(*sql.Tx) error) error {
	return s.run.mutate(ctx, actorFrom(ctx, p, request), action, res, fn)
}
