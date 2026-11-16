package control

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/authz"
)

// SeedDevelopment is an explicit CLI-only fixture operation. It is never an HTTP route.
func (s *Server) SeedDevelopment(ctx context.Context, provisionerPassword string) error {
	if !s.Config.Development {
		return fmt.Errorf("development configuration required")
	}
	if _, e := s.Bootstrap(ctx, s.Config.OIDCIssuer, "operator", "Platform operator", "operator@lastapp.local"); e != nil {
		return e
	}
	for _, u := range []struct{ subject, name string }{{"tenant-admin", "Tenant administrator"}, {"outsider", "Outside user"}, {"joe", "Joe (Marketing Reviewer)"}} {
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
	if e != nil {
		return e
	}
	return s.seedFoundationDemo(ctx)
}

// One development-only example makes scoped roles inspectable in the console.
// It is created once; restarting never restores permissions a reviewer revoked.
func (s *Server) seedFoundationDemo(ctx context.Context) error {
	const tenant = "30000000-0000-4000-8000-000000000001"
	const space = "40000000-0000-4000-8000-000000000001"
	const marketing = "60000000-0000-4000-8000-000000000001"
	const engineering = "60000000-0000-4000-8000-000000000002"
	const role = "70000000-0000-4000-8000-000000000001"
	const app = "50000000-0000-4000-8000-000000000001"
	const second = "50000000-0000-4000-8000-000000000002"
	var operator, owner, joe string
	for sub, dest := range map[string]*string{"operator": &operator, "tenant-admin": &owner, "joe": &joe} {
		if e := s.DB.QueryRowContext(ctx, "SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject=?", s.Config.OIDCIssuer, sub).Scan(dest); e != nil {
			return e
		}
	}
	return s.mutate(ctx, Principal{ID: operator, Platform: true}, "platform.admin", Resource{}, newID(), func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants WHERE id=?", tenant).Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			return nil
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO tenants(id,slug,display_name,plan_id,created_by) VALUES (?,'lastapp-demo','Northstar demo','10000000-0000-4000-8000-000000000001',?)", tenant, operator); e != nil {
			return e
		}
		for _, user := range []string{operator, owner, joe} {
			if _, e := tx.ExecContext(ctx, "INSERT INTO tenant_memberships(tenant_id,user_id) VALUES (?,?)", tenant, user); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO access_grants(id,tenant_id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,?,'tenant','control',?,?,'builtin')", newID(), tenant, owner, TenantAdminRole); e != nil {
			return e
		}
		for _, team := range []struct{ id, slug, name string }{{marketing, "marketing", "Marketing"}, {engineering, "engineering", "Engineering"}} {
			if _, e := tx.ExecContext(ctx, "INSERT INTO groups(id,tenant_id,slug,display_name) VALUES (?,?,?,?)", team.id, tenant, team.slug, team.name); e != nil {
				return e
			}
			if _, e := tx.ExecContext(ctx, "INSERT INTO group_memberships(tenant_id,group_id,user_id) VALUES (?,?,?)", tenant, team.id, owner); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO group_memberships(tenant_id,group_id,user_id) VALUES (?,?,?)", tenant, marketing, joe); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO access_grants(id,tenant_id,scope_kind,domain,user_id,scope_team_id,role_id,role_owner_key) VALUES (?,?,'team','control',?,?,?,'builtin')", newID(), tenant, joe, marketing, authz.TeamReviewerRole); e != nil {
			return e
		}
		resource := newID()
		policy, e := policyFor(ctx, tx, tenant)
		if e != nil {
			return e
		}
		if e = s.allocateDatabase(ctx, tx, Principal{ID: owner}, tenant, databaseInput{ID: resource, Mode: "managed", Layout: "shared", Target: "20000000-0000-4000-8000-000000000001"}, policy, newID()); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE tenants SET default_shared_database_id=? WHERE id=?", resource, tenant); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO spaces(id,tenant_id,slug,display_name,database_resource_id,database_layout) VALUES (?,?,'crm','CRM',?,'shared')", space, tenant, resource); e != nil {
			return e
		}
		if e = assignInitialManager(ctx, tx, Principal{ID: operator}, tenant, space, "", owner, newID()); e != nil {
			return e
		}
		for _, a := range []struct{ id, slug, name string }{{app, "deal-review", "Deal review"}, {second, "forecasting", "Forecasting"}} {
			if _, e = tx.ExecContext(ctx, "INSERT INTO apps(id,tenant_id,space_id,slug,display_name) VALUES (?,?,?,?,?)", a.id, tenant, space, a.slug, a.name); e != nil {
				return e
			}
			if e = assignInitialManager(ctx, tx, Principal{ID: operator}, tenant, space, a.id, owner, newID()); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO roles(id,tenant_id,space_id,owner_key,scope_kind,domain,code,display_name) VALUES (?,?,?,?,'space','data','sales-reviewer','Sales reviewer')", role, tenant, space, "s:"+space); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO access_grants(id,tenant_id,scope_kind,domain,group_id,space_id,role_id,role_owner_key) VALUES (?,?,'space','data',?,?,?,?)", newID(), tenant, marketing, space, role, "s:"+space); e != nil {
			return e
		}
		return audit(ctx, tx, tenant, operator, "development.seeded", "tenant", tenant, newID(), Row{"space_id": space, "app_ids": []string{app, second}, "team_ids": []string{marketing, engineering}})
	})
}
