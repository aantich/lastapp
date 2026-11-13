package entity

import (
	"fmt"
	"github.com/google/uuid"
	"slices"
	"strings"
)

func PermissionID(entity, key string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("lastapp:data:"+entity+":"+key)).String()
}

type Gate struct {
	Permission string   `json:"permission"`
	Fields     []string `json:"fields,omitempty"`
	When       *Expr    `json:"when,omitempty"`
	Roles      []string `json:"roles,omitempty"`
}
type AppAccess struct {
	Origin       string `json:"origin"`
	Inherit      bool   `json:"inherit"`
	Ceiling      []Gate `json:"ceiling"`
	Restricted   bool   `json:"restricted"`
	Restrictions []Gate `json:"restrictions"`
}

func (d Definition) Activated(p Permission, c Context) bool {
	if len(p.Roles) > 0 {
		for _, r := range p.Roles {
			if c.Roles[r] {
				return true
			}
		}
		return false
	}
	switch p.Relationship {
	case "member":
		return true
	case "user":
		return c.Actor == p.Subject
	case "team":
		return c.Teams[p.Subject]
	case "owner":
		return c.Record[d.Owner] == c.Actor
	case "team_member":
		return c.Teams[fmt.Sprint(c.Record[d.Team])]
	case "team_reviewer":
		return c.ReviewTeams[fmt.Sprint(c.Record[d.Team])]
	case "team_admin":
		return c.AdminTeams[fmt.Sprint(c.Record[d.Team])]
	}
	return false
}
func includes(have, need []string) bool {
	for _, k := range need {
		if k == "id" || k == "revision" {
			continue
		}
		if !slices.Contains(have, k) {
			return false
		}
	}
	return true
}
func gateAllows(gates []Gate, id string, fields []string, c Context) bool {
	for _, g := range gates {
		if g.Permission != id || !includes(g.Fields, fields) || !Allows(g.When, c) {
			continue
		}
		if len(g.Roles) > 0 {
			found := false
			for _, r := range g.Roles {
				found = found || c.Roles[r]
			}
			if !found {
				continue
			}
		}
		return true
	}
	return false
}
func (a AppAccess) Allows(id string, fields []string, c Context) bool {
	return (a.Inherit || gateAllows(a.Ceiling, id, fields, c)) && (!a.Restricted || gateAllows(a.Restrictions, id, fields, c))
}
func (d Definition) Allowed(entity, op string, fields []string, c Context, a AppAccess, shared map[string]bool) bool {
	c = d.Bind(c)
	for _, p := range d.Permissions {
		if p.Operation != op || !includes(p.Fields, fields) {
			continue
		}
		pid := PermissionID(entity, p.Key)
		if !(d.Activated(p, c) || shared[pid] && len(p.Roles) > 0) || !Allows(p.When, c) || !Allows(p.ChangeWhen, c) || !a.Allows(pid, fields, c) {
			continue
		}
		if op == "update" {
			old := c
			old.Record = c.Old
			if !(d.Activated(p, old) || shared[pid] && len(p.Roles) > 0) || !Allows(p.When, old) || !a.Allows(pid, fields, old) {
				continue
			}
		}
		return true
	}
	return false
}

// SQL predicates retain each permission's row and field restrictions together.
// Share supplies an EXISTS predicate for the exact base permission/revision.
type ShareSQL func(Permission) (string, []any)

func (d Definition) ReadSQL(entity string, fields []string, c Context, a AppAccess, share ShareSQL) (string, []any, error) {
	parts := []string{}
	args := []any{}
	pathsUsed := 0
	if len(fields) == 0 {
		fields = []string{"id"}
	}
	for _, k := range fields {
		paths := []string{}
		for _, p := range d.Permissions {
			if p.Operation != "read" || !includes(p.Fields, []string{k}) {
				continue
			}
			pathsUsed++
			if pathsUsed > Limits.ReadPaths {
				return "", nil, fmt.Errorf("Query permission complexity exceeds the limit. Select fewer fields.")
			}
			activation, aa := d.activationSQL(p, c)
			if share != nil && len(p.Roles) > 0 {
				ss, sa := share(p)
				activation = "(" + activation + " OR " + ss + ")"
				aa = append(aa, sa...)
			}
			cond, ca, e := d.ExprSQL(p.When, c)
			if e != nil {
				return "", nil, e
			}
			pid := PermissionID(entity, p.Key)
			ceiling, ga, e := d.gateSQL(a.Ceiling, pid, []string{k}, c)
			if e != nil {
				return "", nil, e
			}
			if a.Inherit {
				ceiling = "1=1"
				ga = nil
			}
			restriction, ra, e := d.gateSQL(a.Restrictions, pid, []string{k}, c)
			if e != nil {
				return "", nil, e
			}
			if !a.Restricted {
				restriction = "1=1"
				ra = nil
			}
			paths = append(paths, "("+activation+" AND "+cond+" AND "+ceiling+" AND "+restriction+")")
			args = append(args, aa...)
			args = append(args, ca...)
			args = append(args, ga...)
			args = append(args, ra...)
			if len(args) > MaxSQLParameters {
				return "", nil, fmt.Errorf("Query permission expansion exceeds the parameter limit. Select fewer fields.")
			}
		}
		if len(paths) == 0 {
			paths = []string{"1=0"}
		}
		parts = append(parts, "("+strings.Join(paths, " OR ")+")")
	}
	return "(" + strings.Join(parts, " AND ") + ")", args, nil
}
func (d Definition) gateSQL(gs []Gate, id string, fields []string, c Context) (string, []any, error) {
	parts := []string{}
	args := []any{}
	for _, g := range gs {
		if g.Permission != id || !includes(g.Fields, fields) {
			continue
		}
		if len(g.Roles) > 0 {
			ok := false
			for _, r := range g.Roles {
				ok = ok || c.Roles[r]
			}
			if !ok {
				continue
			}
		}
		s, a, e := d.ExprSQL(g.When, c)
		if e != nil {
			return "", nil, e
		}
		parts = append(parts, s)
		args = append(args, a...)
	}
	if len(parts) == 0 {
		return "1=0", nil, nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args, nil
}
func (d Definition) activationSQL(p Permission, c Context) (string, []any) {
	if len(p.Roles) > 0 {
		if d.Activated(p, c) {
			return "1=1", nil
		}
		return "1=0", nil
	}
	switch p.Relationship {
	case "member":
		return "1=1", nil
	case "user":
		if p.Subject == c.Actor {
			return "1=1", nil
		}
	case "team":
		if c.Teams[p.Subject] {
			return "1=1", nil
		}
	case "owner":
		return Column(d.Owner) + "=?", []any{c.Actor}
	case "team_member":
		return teamSQL(Column(d.Team), c.Teams)
	case "team_reviewer":
		return teamSQL(Column(d.Team), c.ReviewTeams)
	case "team_admin":
		return teamSQL(Column(d.Team), c.AdminTeams)
	}
	return "1=0", nil
}
func teamSQL(column string, teams map[string]bool) (string, []any) {
	ks := []string{}
	for k, v := range teams {
		if v {
			ks = append(ks, k)
		}
	}
	slices.Sort(ks)
	if len(ks) == 0 {
		return "1=0", nil
	}
	args := []any{}
	for _, k := range ks {
		args = append(args, k)
	}
	return column + " IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ks)), ",") + ")", args
}
func Column(k string) string {
	if slices.Contains(reserved, k) {
		return "r.`" + k + "`"
	}
	return "r.`f_" + k + "`"
}
func (d Definition) ExprSQL(x *Expr, c Context) (string, []any, error) {
	if x == nil {
		return "1=1", nil, nil
	}
	if x.Field != "" {
		if _, ok := d.Field(x.Field); !ok {
			return "", nil, fmt.Errorf("Unknown query field.")
		}
		return Column(x.Field), nil, nil
	}
	if x.Op == "" {
		v, e := Eval(*x, c)
		if e != nil {
			return "", nil, e
		}
		if x.Decimal != "" {
			return fmt.Sprintf("CAST(? AS DECIMAL(%d,%d))", MaxDecimalPrecision+MaxDecimalScale, MaxDecimalScale), []any{x.Decimal}, nil
		}
		return "?", []any{v}, nil
	}
	ss := []string{}
	args := []any{}
	for i := range x.Args {
		s, a, e := d.ExprSQL(&x.Args[i], c)
		if e != nil {
			return "", nil, e
		}
		ss = append(ss, s)
		args = append(args, a...)
	}
	switch x.Op {
	case "all":
		return "(" + strings.Join(ss, " AND ") + ")", args, nil
	case "any":
		return "(" + strings.Join(ss, " OR ") + ")", args, nil
	case "not":
		return "(NOT COALESCE(" + ss[0] + ",FALSE))", args, nil
	case "is_null":
		return "(" + ss[0] + " IS NULL)", args, nil
	case "is_not_null":
		return "(" + ss[0] + " IS NOT NULL)", args, nil
	case "in":
		return "COALESCE((" + ss[0] + " IN (" + strings.Join(ss[1:], ",") + ")),FALSE)", args, nil
	case "coalesce":
		return "COALESCE(" + strings.Join(ss, ",") + ")", args, nil
	}
	ops := map[string]string{"eq": "=", "ne": "<>", "lt": "<", "lte": "<=", "gt": ">", "gte": ">=", "add": "+", "sub": "-", "mul": "*", "div": "/"}
	op, ok := ops[x.Op]
	if !ok {
		return "", nil, fmt.Errorf("Unsupported SQL operator.")
	}
	s := "(" + ss[0] + op + ss[1] + ")"
	if slices.Contains([]string{"eq", "ne", "lt", "lte", "gt", "gte"}, x.Op) {
		s = "COALESCE(" + s + ",FALSE)"
	}
	return s, args, nil
}
