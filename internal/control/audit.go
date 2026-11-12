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
func auditChange(ctx context.Context, q Queryer, tenant, user, action, table, kind, record, request string, before Row) error {
	after, e := securitySnapshot(ctx, q, table, tenant, record)
	if e != nil {
		return e
	}
	return audit(ctx, q, tenant, user, action, kind, record, request, Row{"before": before, "after": after})
}
