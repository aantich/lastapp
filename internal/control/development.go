package control

import (
	"context"
	"fmt"
	mariadb "github.com/aantich/go-mariadb"
	"net"
	"strconv"
)

// SeedDevelopment is an explicit CLI-only fixture operation. It is never an HTTP route.
func (s *Server) SeedDevelopment(ctx context.Context, provisionerPassword string) error {
	if !s.Config.Development {
		return fmt.Errorf("development configuration required")
	}
	if _, e := s.Bootstrap(ctx, s.Config.OIDCIssuer, "operator", "Platform operator", "operator@lastapp.local"); e != nil {
		return e
	}
	for _, u := range []struct{ subject, name string }{{"tenant-admin", "Tenant administrator"}, {"outsider", "Outside user"}} {
		if _, e := s.DB.ExecContext(ctx, "INSERT INTO users (id,oidc_issuer,oidc_subject,display_name,email) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name)", newID(), s.Config.OIDCIssuer, u.subject, u.name, u.subject+"@lastapp.local"); e != nil {
			return e
		}
	}
	_, e := s.DB.ExecContext(ctx, "INSERT IGNORE INTO plans (id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases) VALUES ('10000000-0000-4000-8000-000000000001','development','Development',1,1,1,1,1,'shared',100,500,100),('10000000-0000-4000-8000-000000000002','starter','Starter',1,0,1,0,0,'shared',3,10,1)")
	if e != nil {
		return e
	}
	if provisionerPassword == "" {
		return nil
	}
	const target = "20000000-0000-4000-8000-000000000001"
	enc, e := s.seal("target", target, "", credentials{Username: "lastapp_provisioner", Password: provisionerPassword})
	if e != nil {
		return e
	}
	host, port := "127.0.0.1", 33317
	if cfg, err := mariadb.ParseDSN(s.Config.RegistryDSN); err == nil && cfg.Net == "tcp" {
		if h, p, err := net.SplitHostPort(cfg.Addr); err == nil {
			host = h
			port, _ = strconv.Atoi(p)
		}
	}
	_, e = s.DB.ExecContext(ctx, "INSERT INTO database_targets (id,display_name,canonical_host,port,tls_mode,managed_enabled,provisioning_credentials_enc) VALUES (?,?,?,?,'disabled',1,?) ON DUPLICATE KEY UPDATE provisioning_credentials_enc=VALUES(provisioning_credentials_enc)", target, "Local MariaDB", host, port, enc)
	return e
}
