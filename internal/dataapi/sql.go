package dataapi

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aantich/lastapp/internal/entity"
)

// SQL fragments for business-record reads and keyset pages. They return
// parameterized SQL and its arguments and never touch a database.

func (rt dataRuntime) shareSQL(p entity.Permission) (string, []any) {
	if !rt.d.Sharing {
		return "1=0", nil
	}
	pid := entity.PermissionID(rt.entityID, p.Key)
	team, args := teamRecipientSQL(rt.actor)
	s := "EXISTS(SELECT 1 FROM _lastapp_record_acl acl WHERE acl.tenant_id=r.tenant_id AND acl.space_id=r.space_id AND acl.entity_id=? AND acl.record_id=r.id AND acl.permission_id=? AND acl.security_revision=? AND ((acl.recipient_kind='user' AND acl.recipient_id=?) OR " + team + "))"
	return s, append([]any{rt.entityID, pid, rt.permissionRevisions[pid], rt.actor.Actor}, args...)
}

func teamRecipientSQL(c entity.Context) (string, []any) {
	keys := []string{}
	for k := range c.Teams {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		return "1=0", nil
	}
	args := []any{}
	for _, k := range keys {
		args = append(args, k)
	}
	return "(acl.recipient_kind='team' AND acl.recipient_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",") + "))", args
}

func uniqueStrings(v []string) []string {
	out := []string{}
	for _, k := range v {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

func sortFields(v []dataSort) []string {
	out := []string{}
	for _, s := range v {
		out = append(out, s.Field)
	}
	return out
}

func cursorSQL(d entity.Definition, order []dataSort, values []any) (string, []any) {
	parts := []string{}
	args := []any{}
	placeholder := func(key string) string {
		f, _ := d.Field(key)
		if f.Type == "decimal" {
			return fmt.Sprintf("CAST(? AS DECIMAL(%d,%d))", f.Precision, f.Scale)
		}
		return "?"
	}
	for i, so := range order {
		eq := []string{}
		ea := []any{}
		for j := 0; j < i; j++ {
			eq = append(eq, entity.Column(order[j].Field)+" <=> "+placeholder(order[j].Field))
			ea = append(ea, values[j])
		}
		if values[i] == nil {
			continue
		}
		col := entity.Column(so.Field)
		op := ">"
		if so.Direction == "desc" {
			op = "<"
		}
		eq = append(eq, "("+col+" IS NULL OR "+col+op+placeholder(so.Field)+")")
		ea = append(ea, values[i])
		parts = append(parts, "("+strings.Join(eq, " AND ")+")")
		args = append(args, ea...)
	}
	if len(parts) == 0 {
		return "1=0", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}
