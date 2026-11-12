package control

import "context"

// Explicit safe columns: never snapshot encrypted credential columns or sessions.
func securitySnapshot(ctx context.Context, q Queryer, table, tenant, record string) (Row, error) {
	columns := map[string]string{
		"roles":         "id,display_name,state,revision",
		"groups":        "id,display_name,state,revision",
		"access_grants": "id,tenant_id,kind,user_id,group_id,space_id,app_id,role_id,permission_code,revision",
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
		ps, e := rows(ctx, q, "SELECT permission_code FROM role_permissions WHERE role_id=? ORDER BY permission_code", record)
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
