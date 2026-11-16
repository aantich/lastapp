package roles

import (
	"context"
	"fmt"

	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// teamSnapshot is a team's audited security state.
type teamSnapshot struct {
	Name     string `json:"display_name"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	State    string `json:"state"`
}

func (v *teamSnapshot) ScanFields() []any { return []any{&v.Name, &v.ID, &v.Revision, &v.State} }

// Audited security states use explicit safe columns: never snapshot
// encrypted credential columns or sessions.

func snapshotWhere(tenant, record string) (string, []any) {
	where, args := " WHERE id=?", []any{record}
	if tenant != "" {
		where += " AND tenant_id=?"
		args = append(args, tenant)
	}
	return where + " FOR UPDATE", args
}

// roleSnapshot is a role's audited state with its permission codes.
func roleSnapshot(ctx context.Context, q registry.Queryer, tenant, record string) (Role, error) {
	where, args := snapshotWhere(tenant, record)
	v, e := registry.ScanOne[Role](ctx, q, "SELECT "+roleColumns+" FROM roles"+where, args...)
	if e != nil {
		return v, e
	}
	codes, e := registry.ScanStrings(ctx, q, "SELECT p.code FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=? ORDER BY p.code", record)
	v.Permissions = &codes
	return v, e
}

func TeamSnapshotOf(ctx context.Context, q registry.Queryer, tenant, record string) (teamSnapshot, error) {
	where, args := snapshotWhere(tenant, record)
	return registry.ScanOne[teamSnapshot](ctx, q, "SELECT display_name,id,revision,state FROM groups"+where, args...)
}

func assignmentSnapshot(ctx context.Context, q registry.Queryer, tenant, record string) (Assignment, error) {
	where, args := snapshotWhere(tenant, record)
	return registry.ScanOne[Assignment](ctx, q, "SELECT "+assignmentColumns+" FROM access_grants"+where, args...)
}

// securitySnapshot is the audited state of a record of table.
func securitySnapshot(ctx context.Context, q registry.Queryer, table, tenant, record string) (any, error) {
	switch table {
	case "roles":
		return roleSnapshot(ctx, q, tenant, record)
	case "groups":
		return TeamSnapshotOf(ctx, q, tenant, record)
	case "access_grants":
		return assignmentSnapshot(ctx, q, tenant, record)
	}
	return nil, fmt.Errorf("no security snapshot for %s", table)
}

// AuditChangeAs records a security-relevant change with before/after snapshots.
func AuditChangeAs(ctx context.Context, q registry.Queryer, a op.Actor, tenant, action, table, kind, record string, before any) error {
	after, e := securitySnapshot(ctx, q, table, tenant, record)
	if e != nil {
		return e
	}
	return op.AuditAs(ctx, q, a, tenant, action, kind, record, registry.Row{"before": before, "after": after})
}
