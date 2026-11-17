package control

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	mariadb "github.com/aantich/go-mariadb"
	"github.com/pressly/goose/v3"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/dataapi"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/inventory"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/publisher"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
	"github.com/aantich/lastapp/internal/sealer"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Server struct {
	*access.Checker
	uploadSlots            chan struct{}
	agentSlots             chan struct{}
	agentBootstrapSlots    chan struct{}
	agentTokenSlots        chan struct{}
	agentValidationSlots   chan struct{}
	agentBootstrapLimiter  httpapi.Limiter
	agentValidationLimiter httpapi.Limiter
	limiter                httpapi.Limiter
	loginLimiter           httpapi.Limiter
	passwordSlots          chan struct{}
	trustedProxies         httpapi.TrustedProxies
	DB                     *sql.DB
	Config                 config.Config
	sealer                 *sealer.Sealer
	registryName           string
	mux                    *http.ServeMux
	auth                   *oidcAuth
	Log                    *slog.Logger
	run                    *op.Runner
	inv                    *inventory.Inventory
	members                *membership.Members
	authority              *roles.Authority
	storage                *databases.Storage
	authoring              *entities.Authoring
	publisher              *publisher.Publisher
	dataAPI                *dataapi.Runtime
}

func LoadConfig(path string) (config.Config, error) {
	var c config.Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}

func Open(c config.Config) (*Server, error) {
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
	proxies, e := httpapi.ParseTrustedProxies(c.TrustedProxies)
	if e != nil {
		return nil, e
	}
	if !config.ValidOrigin(c.PublicURL, c.Development) {
		return nil, fmt.Errorf("public_url must be an origin (loopback in development)")
	}
	if c.DevelopmentUIOrigin != "" && (!c.Development || !config.ValidOrigin(c.DevelopmentUIOrigin, true)) {
		return nil, fmt.Errorf("development_ui_origin requires development mode and a loopback origin")
	}
	adminURL, _ := url.Parse(c.PublicURL)
	appURL, _ := url.Parse(c.AppOrigin)
	appNamespace := appHostNamespace(c.AppOrigin)
	adminHost := strings.TrimSuffix(strings.ToLower(adminURL.Hostname()), ".")
	if c.AppOrigin != "" && (appURL == nil || !config.ValidOrigin(c.AppOrigin, false) || adminHost == strings.TrimPrefix(appNamespace, ".") || appNamespace != "" && strings.HasSuffix(adminHost, appNamespace) || (!c.Development && !strings.HasPrefix(c.AppOrigin, "https://"))) {
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
	seal, e := sealer.New(c.EncryptionKey, c.EncryptionKeyID)
	if e != nil {
		return nil, e
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
	s := &Server{uploadSlots: make(chan struct{}, 2), DB: db, Config: c, sealer: seal, registryName: cfg.DBName, Log: slog.Default(), mux: http.NewServeMux(), passwordSlots: make(chan struct{}, 2), trustedProxies: proxies, agentSlots: make(chan struct{}, agentMaxConcurrent), agentBootstrapSlots: make(chan struct{}, agentBootstrapConcurrent), agentTokenSlots: make(chan struct{}, agentTokenConcurrent), agentValidationSlots: make(chan struct{}, agentValidationConcurrent)}
	s.Checker = &access.Checker{Audience: s.mcpURL()}
	s.run = &op.Runner{DB: db, Access: s.Checker}
	s.publisher = &publisher.Publisher{DB: db, Access: s.Checker, Sealer: seal, RegistryName: cfg.DBName, Connect: s.targetDB, DeploymentAuthority: s.publishingDeploymentAuthority, DeploymentEntities: deploymentEntities, Warn: func(ctx context.Context, msg string, err error, attrs ...any) {
		httpapi.Logger(ctx, s.Log).Warn(msg, append(attrs, httpapi.Diagnostic(err)...)...)
	}}
	s.authoring = &entities.Authoring{DB: db, Run: s.run, Config: &s.Config, Queue: s.queueDefinition}
	s.storage = &databases.Storage{DB: db, Run: s.run, Seal: s.sealer.Seal, Config: &s.Config, RegistryName: cfg.DBName}
	s.inv = &inventory.Inventory{DB: db, Run: s.run, AllocateDatabase: s.storage.Allocate, PrepareAccount: s.prepareAccount}
	s.members = &membership.Members{DB: db, Run: s.run, PrepareAccount: s.prepareAccount}
	s.authority = &roles.Authority{DB: db, Run: s.run}
	s.dataAPI = &dataapi.Runtime{DB: db, Access: s.Checker, Authoring: s.authoring, Publisher: s.publisher, Sealer: seal, Config: &s.Config, Limiter: &s.limiter, Proxies: &s.trustedProxies, Slots: make(chan struct{}, entity.Limits.DataRequests)}
	return s, nil
}

func (s *Server) Close() error { s.publisher.Close(); return s.DB.Close() }

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

// mutate is the context shim of runner.mutate for handlers not yet migrated
// to operations with an explicit Actor.
func (s *Server) mutate(ctx context.Context, p access.Principal, action string, res access.Resource, request string, fn func(*sql.Tx) error) error {
	return s.run.Mutate(ctx, op.ActorFrom(ctx, p, request), action, res, fn)
}
