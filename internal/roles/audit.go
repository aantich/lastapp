package roles

import (
	"context"
	"fmt"
	"time"

	"github.com/aantich/lastapp/internal/access"
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

// AuditEvent is an audit trail entry with its actor's name.
type AuditEvent struct {
	Action       string             `json:"action"`
	ActorName    string             `json:"actor_name"`
	CreatedAt    time.Time          `json:"created_at"`
	Details      registry.JSONValue `json:"details_json"`
	ID           int64              `json:"id"`
	RequestID    string             `json:"request_id"`
	ResourceID   *string            `json:"resource_id"`
	ResourceType string             `json:"resource_type"`
	TenantID     *string            `json:"tenant_id"`
}

func (v *AuditEvent) ScanFields() []any {
	return []any{&v.Action, &v.ActorName, &v.CreatedAt, &v.Details, &v.ID, &v.RequestID, &v.ResourceID, &v.ResourceType, &v.TenantID}
}

// Audits pages a Tenant's audit trail (tenant set) or the whole platform's.
func (v *Authority) Audits(ctx context.Context, actor op.Actor, tenant string, page op.PageRequest) (op.Page[AuditEvent], error) {
	p := actor.Principal
	action := "audit.read"
	if tenant == "" {
		action = "platform.admin"
	}
	if e := v.Run.Access.Authorize(ctx, v.DB, p, action, access.Resource{Tenant: tenant}, false); e != nil {
		return op.Page[AuditEvent]{}, e
	}
	args := []any{}
	where := "1=1"
	if tenant != "" {
		where = "a.tenant_id=?"
		args = append(args, tenant)
	}
	return op.PageScan[AuditEvent](ctx, v.DB, page, op.CollectionFor("audit-events"), "SELECT a.action,COALESCE(u.display_name,'System worker') AS actor_name,a.created_at,a.details_json,a.id,a.request_id,a.resource_id,a.resource_type,a.tenant_id FROM audit_events a LEFT JOIN users u ON u.id=a.actor_user_id WHERE "+where+" ORDER BY a.id DESC", args...)
}
