// Package delivery owns development workspaces, uploaded static builds,
// releases, production deployments, starters and development launch codes.
// Its typed operations are called by both the console handlers and the MCP
// tools in internal/control; HTTP decoding, rate limits and the App host
// server stay there. The worker leader loop (internal/control) calls
// WorkspaceJob and DeploymentJob as steps.
package delivery

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/publisher"
)

// Service runs the delivery operations against the registry.
type Service struct {
	DB           *sql.DB
	Run          *op.Runner
	Access       *access.Checker
	Config       *config.Config
	Storage      *databases.Storage
	Publisher    *publisher.Publisher
	RegistryName string
	// Warn logs a failure without its message (see httpapi.Diagnostic).
	Warn func(ctx context.Context, msg string, err error, attrs ...any)
}

// ArtifactDirectory is the filesystem root of immutable uploaded builds, or
// "" when no persistent artifact storage is configured.
func (s *Service) ArtifactDirectory() string {
	if s.Config.ArtifactDirectory != "" {
		return s.Config.ArtifactDirectory
	}
	if s.Config.Development {
		return filepath.Join(".local", "artifacts")
	}
	return ""
}

// HostingBase is the origin whose host namespace serves static Apps.
func HostingBase(c *config.Config) string {
	base := c.AppOrigin
	if base == "" && c.Development {
		u, _ := url.Parse(c.PublicURL)
		port := u.Port()
		u.Host = "localhost"
		if port != "" {
			u.Host += ":" + port
		}
		base = u.String()
	}
	return base
}

// HostNamespace is shared by startup validation and routing: every hostname
// below this suffix belongs to static Apps, including malformed App IDs and
// alternate ports.
func HostNamespace(origin string) string {
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" {
		return ""
	}
	return "." + strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
}

// HostedOrigin is the dedicated static origin of an App ("" when App hosting
// is not configured).
func HostedOrigin(c *config.Config, app string) string {
	base := HostingBase(c)
	u, e := url.Parse(base)
	if e != nil || u.Host == "" {
		return ""
	}
	port := u.Port()
	u.Host = app + HostNamespace(base)
	if port != "" && !(u.Scheme == "https" && port == "443") && !(u.Scheme == "http" && port == "80") {
		u.Host = net.JoinHostPort(u.Host, port)
	}
	return strings.TrimSuffix(u.String(), "/")
}

// HostedOrigin is the dedicated static origin of an App.
func (s *Service) HostedOrigin(app string) string { return HostedOrigin(s.Config, app) }

// LockTenant takes the Tenant row lock that serializes registry mutations.
func LockTenant(ctx context.Context, tx *sql.Tx, tenant string) error {
	var v string
	return tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE id=? FOR UPDATE", tenant).Scan(&v)
}
