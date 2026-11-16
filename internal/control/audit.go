package control

import "context"

// Explicit safe columns: never snapshot encrypted credential columns or sessions.
func securitySnapshot(ctx context.Context, q Queryer, table, tenant, record string) (Row, error) {
	columns := map[string]string{
		"roles":         "id,tenant_id,space_id,app_id,scope_kind,domain,code,display_name,state,revision",
		"groups":        "id,display_name,state,revision",
		"access_grants": "id,tenant_id,scope_kind,domain,user_id,group_id,scope_team_id,space_id,app_id,role_id,revision",
	}
	where, args := "id=?", []any{record}
	if tenant != "" {
		where += " AND tenant_id=?"
		args = append(args, tenant)
	}
	v, e := row(ctx, q, "SELECT "+columns[table]+" FROM "+table+" WHERE "+where+" FOR UPDATE", args...)
	if e != nil {
		return nil, e
	}
	if table == "roles" {
		ps, e := rows(ctx, q, "SELECT p.code AS permission_code FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=? ORDER BY p.code", record)
		if e != nil {
			return nil, e
		}
		codes := []string{}
		for _, p := range ps {
			codes = append(codes, str(p["permission_code"]))
		}
		v["permissions"] = codes
	}
	return v, nil
}

// auditChangeAs records a security-relevant change with before/after snapshots.
func auditChangeAs(ctx context.Context, q Queryer, a Actor, tenant, action, table, kind, record string, before Row) error {
	after, e := securitySnapshot(ctx, q, table, tenant, record)
	if e != nil {
		return e
	}
	return auditAs(ctx, q, a, tenant, action, kind, record, Row{"before": before, "after": after})
}

// auditChange is the context shim of auditChangeAs.
func auditChange(ctx context.Context, q Queryer, tenant, user, action, table, kind, record, request string, before Row) error {
	return auditChangeAs(ctx, q, Actor{Principal: Principal{ID: user}, RequestID: request, Agent: agentCallFrom(ctx)}, tenant, action, table, kind, record, before)
}
