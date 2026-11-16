package access

import (
	"context"

	"github.com/aantich/lastapp/internal/registry"
)

func EnsurePlatformAdmin(ctx context.Context, q registry.Queryer) error {
	var n int
	e := q.QueryRowContext(ctx, "SELECT COUNT(DISTINCT g.user_id) FROM access_grants g JOIN users u ON u.id=g.user_id AND u.state='active' WHERE g.scope_kind='platform' AND g.role_id=?", SuperRole).Scan(&n)
	if e != nil {
		return e
	}
	if n < 1 {
		return registry.Conflict("Keep at least one active platform administrator.")
	}
	return nil
}

func EnsureTenantAdmin(ctx context.Context, q registry.Queryer, tenant string) error {
	var n int
	e := q.QueryRowContext(ctx, "SELECT COUNT(DISTINCT g.user_id) FROM access_grants g JOIN users u ON u.id=g.user_id AND u.state='active' JOIN tenant_memberships m ON m.tenant_id=g.tenant_id AND m.user_id=g.user_id AND m.state='active' WHERE g.tenant_id=? AND g.role_id=? AND g.space_id IS NULL AND g.app_id IS NULL AND NOT EXISTS (SELECT 1 FROM access_grants pg WHERE pg.user_id=g.user_id AND pg.role_id=?)", tenant, TenantAdminRole, SuperRole).Scan(&n)
	if e != nil {
		return e
	}
	if n < 1 {
		return registry.Conflict("Keep at least one active, directly assigned Tenant administrator.")
	}
	return nil
}
