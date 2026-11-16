package entity

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Operator struct {
	Key  string `json:"key"`
	Min  int    `json:"min_args"`
	Max  int    `json:"max_args"`
	Kind string `json:"kind"`
}

var Operators = []Operator{{"all", 1, 32, "boolean"}, {"any", 1, 32, "boolean"}, {"not", 1, 1, "boolean"}, {"eq", 2, 2, "compare"}, {"ne", 2, 2, "compare"}, {"lt", 2, 2, "compare"}, {"lte", 2, 2, "compare"}, {"gt", 2, 2, "compare"}, {"gte", 2, 2, "compare"}, {"in", 2, 32, "compare"}, {"is_null", 1, 1, "boolean"}, {"is_not_null", 1, 1, "boolean"}, {"add", 2, 2, "numeric"}, {"sub", 2, 2, "numeric"}, {"mul", 2, 2, "numeric"}, {"div", 2, 2, "numeric"}, {"coalesce", 2, 8, "value"}, {"round", 2, 2, "numeric"}}

type Context struct {
	DecimalFields map[string]bool

	Actor       string
	Now         time.Time
	Record, Old map[string]any
	Roles       map[string]bool
	Teams       map[string]bool
	ReviewTeams map[string]bool
	AdminTeams  map[string]bool
}

func Fields(x Expr) []string {
	out := []string{}
	if x.Field != "" {
		out = append(out, x.Field)
	}
	for _, a := range x.Args {
		out = append(out, Fields(a)...)
	}
	return out
}

func (d Definition) ExpressionType(x Expr, transition bool) (string, error) {
	nodes := 0
	var check func(Expr, int) (string, error)
	check = func(x Expr, depth int) (string, error) {
		nodes++
		if nodes > Limits.Nodes || depth > Limits.Depth {
			return "", fmt.Errorf("Expression exceeds complexity limits.")
		}
		forms := 0
		for _, v := range []bool{x.Op != "", x.Field != "", x.Context != "", x.Decimal != "", x.Literal != nil} {
			if v {
				forms++
			}
		}
		if forms > 1 {
			return "", fmt.Errorf("Expression node must have one form.")
		}
		if x.Field != "" {
			k := x.Field
			if strings.HasPrefix(k, "old.") || strings.HasPrefix(k, "new.") {
				if !transition {
					return "", fmt.Errorf("Old/new values are only allowed in change conditions.")
				}
				k = k[4:]
			}
			f, ok := d.Field(k)
			if !ok {
				return "", fmt.Errorf("Unknown field: %s", k)
			}
			return f.Type, nil
		}
		if x.Context != "" {
			for _, provider := range ContextProviders {
				if provider.Key == x.Context {
					return provider.Type, nil
				}
			}
			return "", fmt.Errorf("Unknown context provider.")
		}
		if x.Decimal != "" {
			if _, ok := rat(x.Decimal); !ok {
				return "", fmt.Errorf("Invalid decimal literal.")
			}
			return "decimal", nil
		}
		if x.Op == "" {
			switch x.Literal.(type) {
			case nil:
				return "null", nil
			case bool:
				return "boolean", nil
			case json.Number, float64, int, int64:
				n, ok := rat(x.Literal)
				if !ok || !n.IsInt() || !n.Num().IsInt64() || n.Num().Int64() > MaxInteger || n.Num().Int64() < -MaxInteger {
					return "", fmt.Errorf("Integer literals must be exact safe integers; use a decimal literal otherwise.")
				}
				return "integer", nil
			case map[string]any, []any:
				return "json", nil
			case string:
				if len(x.Literal.(string)) > MaxTextLength {
					return "", fmt.Errorf("Expression text literal exceeds its size limit.")
				}
				return "text", nil
			default:
				return "", fmt.Errorf("Unsupported literal.")
			}
		}
		var op *Operator
		for i := range Operators {
			if Operators[i].Key == x.Op {
				op = &Operators[i]
			}
		}
		if op == nil || len(x.Args) < op.Min || len(x.Args) > op.Max {
			return "", fmt.Errorf("Unsupported operator or argument count: %s", x.Op)
		}
		ts := []string{}
		for _, a := range x.Args {
			t, e := check(a, depth+1)
			if e != nil {
				return "", e
			}
			ts = append(ts, t)
		}
		numeric := func(t string) bool { return t == "integer" || t == "decimal" || t == "null" }
		if op.Kind == "numeric" {
			if x.Op == "round" {
				n, ok := rat(x.Args[1].Literal)
				if !ok || !n.IsInt() || !n.Num().IsInt64() || n.Num().Int64() < 0 || n.Num().Int64() > MaxDecimalScale {
					return "", fmt.Errorf("Round scale must be a literal integer from 0 to 12.")
				}
			}
			for _, t := range ts {
				if !numeric(t) {
					return "", fmt.Errorf("Arithmetic requires numeric values.")
				}
			}
			return "decimal", nil
		}
		if x.Op == "all" || x.Op == "any" || x.Op == "not" {
			for _, t := range ts {
				if t != "boolean" {
					return "", fmt.Errorf("Logical operators require boolean conditions.")
				}
			}
		}
		if op.Kind == "compare" {
			if slices.Contains(ts, "json") {
				return "", fmt.Errorf("JSON comparisons are not supported.")
			}
			for _, t := range ts[1:] {
				if t != ts[0] && t != "null" && ts[0] != "null" && !(numeric(t) && numeric(ts[0])) && !(t == "text" && slices.Contains([]string{"date", "timestamp", "choice", "reference", "user", "team"}, ts[0]) || ts[0] == "text" && slices.Contains([]string{"date", "timestamp", "choice", "reference", "user", "team"}, t)) {
					return "", fmt.Errorf("Comparison value types do not match.")
				}
			}
		}
		if x.Op == "coalesce" {
			result := "null"
			for _, t := range ts {
				if t == "null" {
					continue
				}
				if result == "null" {
					result = t
				} else if result != t {
					if numeric(result) && numeric(t) {
						result = "decimal"
					} else {
						return "", fmt.Errorf("Coalesce values must have compatible types.")
					}
				}
			}
			return result, nil
		}
		return "boolean", nil
	}
	return check(x, 0)
}

func rat(v any) (*big.Rat, bool) {
	if v == nil {
		return nil, false
	}
	switch n := v.(type) {
	case *big.Rat:
		return new(big.Rat).Set(n), true
	case json.Number:
		if !decimalPattern.MatchString(string(n)) {
			return nil, false
		}
		r, ok := new(big.Rat).SetString(string(n))
		return r, ok
	case float64:
		return new(big.Rat).SetFloat64(n), true
	case int:
		return new(big.Rat).SetInt64(int64(n)), true
	case int64:
		return new(big.Rat).SetInt64(n), true
	case string:
		if !decimalPattern.MatchString(n) {
			return nil, false
		}
		r, ok := new(big.Rat).SetString(n)
		return r, ok
	}
	return nil, false
}
func Truth(v any) bool { b, _ := v.(bool); return b }
func Eval(x Expr, c Context) (any, error) {
	if x.Field != "" {
		k := x.Field
		m := c.Record
		if strings.HasPrefix(k, "old.") {
			m = c.Old
			k = k[4:]
		} else {
			k = strings.TrimPrefix(k, "new.")
		}
		if c.DecimalFields[k] && m[k] != nil {
			n, ok := rat(m[k])
			if !ok {
				return nil, fmt.Errorf("Invalid stored decimal.")
			}
			return n, nil
		}
		return m[k], nil
	}
	if x.Context != "" {
		if x.Context == "actor.id" {
			return c.Actor, nil
		}
		if x.Context == "request.time" {
			return c.Now.UTC().Truncate(time.Microsecond).Format("2006-01-02T15:04:05.000000Z"), nil
		}
		return nil, fmt.Errorf("Unknown context.")
	}
	if x.Decimal != "" {
		r, ok := rat(x.Decimal)
		if !ok {
			return nil, fmt.Errorf("Invalid decimal.")
		}
		return r, nil
	}
	if x.Op == "" {
		return x.Literal, nil
	}
	a := []any{}
	for _, v := range x.Args {
		n, e := Eval(v, c)
		if e != nil {
			return nil, e
		}
		a = append(a, n)
	}
	switch x.Op {
	case "all":
		for _, v := range a {
			if !Truth(v) {
				return false, nil
			}
		}
		return true, nil
	case "any":
		for _, v := range a {
			if Truth(v) {
				return true, nil
			}
		}
		return false, nil
	case "not":
		return !Truth(a[0]), nil
	case "is_null":
		return a[0] == nil, nil
	case "is_not_null":
		return a[0] != nil, nil
	case "coalesce":
		for _, v := range a {
			if v != nil {
				return v, nil
			}
		}
		return nil, nil
	}
	if x.Op == "round" {
		if a[0] == nil {
			return nil, nil
		}
		value, ok := rat(a[0])
		scale, ok2 := rat(a[1])
		if !ok || !ok2 || !scale.IsInt() || !scale.Num().IsInt64() || scale.Num().Int64() < 0 || scale.Num().Int64() > MaxDecimalScale {
			return nil, fmt.Errorf("Invalid round scale.")
		}
		n, _ := rat(round(value, int(scale.Num().Int64())))
		return n, nil
	}
	if slices.Contains([]string{"add", "sub", "mul", "div"}, x.Op) {
		if a[0] == nil || a[1] == nil {
			return nil, nil
		}
		l, ok := rat(a[0])
		r, ok2 := rat(a[1])
		if !ok || !ok2 {
			return nil, fmt.Errorf("Invalid arithmetic.")
		}
		switch x.Op {
		case "add":
			l.Add(l, r)
		case "sub":
			l.Sub(l, r)
		case "mul":
			l.Mul(l, r)
		case "div":
			if r.Sign() == 0 {
				return nil, fmt.Errorf("Division by zero.")
			}
			l.Quo(l, r)
		}
		if l.Num().BitLen() > 512 || l.Denom().BitLen() > 512 {
			return nil, fmt.Errorf("Arithmetic overflow.")
		}
		return l, nil
	}
	cmp := func(l, r any) (int, bool) {
		if l == nil || r == nil {
			return 0, false
		} // numeric strings are only numeric if the other operand is numeric/decimal
		_, ls := l.(string)
		_, rs := r.(string)
		if !ls || !rs {
			ln, ok := rat(l)
			rn, ok2 := rat(r)
			if ok && ok2 {
				return ln.Cmp(rn), true
			}
		}
		return strings.Compare(fmt.Sprint(l), fmt.Sprint(r)), true
	}
	if x.Op == "in" {
		for _, v := range a[1:] {
			n, ok := cmp(a[0], v)
			if ok && n == 0 {
				return true, nil
			}
		}
		return false, nil
	}
	n, ok := cmp(a[0], a[1])
	if !ok {
		return false, nil
	}
	switch x.Op {
	case "eq":
		return n == 0, nil
	case "ne":
		return n != 0, nil
	case "lt":
		return n < 0, nil
	case "lte":
		return n <= 0, nil
	case "gt":
		return n > 0, nil
	case "gte":
		return n >= 0, nil
	}
	return nil, fmt.Errorf("Unsupported expression.")
}

func Allows(x *Expr, c Context) bool {
	if x == nil {
		return true
	}
	v, e := Eval(*x, c)
	return e == nil && Truth(v)
}

func round(r *big.Rat, scale int) string {
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	n := new(big.Int).Mul(r.Num(), factor)
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(n, r.Denom(), rem)
	twice := new(big.Int).Lsh(new(big.Int).Abs(rem), 1)
	cmp := twice.Cmp(r.Denom())
	if cmp > 0 || (cmp == 0 && q.Bit(0) == 1) {
		if n.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	neg := q.Sign() < 0
	s := new(big.Int).Abs(q).String()
	if scale > 0 {
		for len(s) <= scale {
			s = "0" + s
		}
		s = s[:len(s)-scale] + "." + s[len(s)-scale:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

func Value(f Field, v any, calculated bool) (any, error) {
	err := func() (any, error) {
		return nil, Diagnostic{"invalid_value", f.Key, "Invalid value for " + f.Label + "."}
	}
	if v == nil {
		if f.Required {
			return err()
		}
		return nil, nil
	}
	switch f.Type {
	case "text", "choice":
		s, ok := v.(string)
		if !ok || !utf8.ValidString(s) {
			return err()
		}
		if f.Type == "text" && utf8.RuneCountInString(s) > f.MaxLength {
			return err()
		}
		if f.Type == "choice" && !slices.Contains(f.Values, s) {
			return err()
		}
		return s, nil
	case "integer":
		r, ok := rat(v)
		if !ok || !r.IsInt() || !r.Num().IsInt64() || r.Num().Int64() > MaxInteger || r.Num().Int64() < -MaxInteger {
			return err()
		}
		if _, str := v.(string); str && !calculated {
			return err()
		}
		if !bounds(f, r) {
			return err()
		}
		return r.Num().Int64(), nil
	case "decimal":
		if _, ok := v.(string); !ok && !calculated {
			return err()
		}
		if text, ok := v.(string); ok && !calculated {
			parts := strings.Split(text, ".")
			if len(parts) == 2 && len(parts[1]) > f.Scale {
				return err()
			}
		}
		r, ok := rat(v)
		if !ok {
			return err()
		}
		s := round(r, f.Scale)
		if !calculated {
			rr, _ := rat(s)
			if rr.Cmp(r) != 0 {
				return err()
			}
		}
		digits := strings.TrimLeft(strings.ReplaceAll(strings.TrimPrefix(s, "-"), ".", ""), "0")
		if len(digits) > f.Precision || !bounds(f, r) {
			return err()
		}
		return s, nil
	case "boolean":
		if b, ok := v.(bool); ok {
			return b, nil
		}
		return err()
	case "date":
		s, ok := v.(string)
		if !ok {
			return err()
		}
		t, e := time.Parse("2006-01-02", s)
		if e != nil || t.Format("2006-01-02") != s {
			return err()
		}
		return s, nil
	case "timestamp":
		s, ok := v.(string)
		if !ok {
			return err()
		}
		t, e := time.Parse(time.RFC3339Nano, s)
		if e != nil || !calculated && t.Nanosecond()%1000 != 0 {
			return err()
		}
		return t.UTC().Truncate(time.Microsecond).Format("2006-01-02T15:04:05.000000Z"), nil
	case "reference", "user", "team":
		s, ok := v.(string)
		u, e := uuid.Parse(s)
		if !ok || e != nil || u.String() != s || u == uuid.Nil {
			return err()
		}
		return s, nil
	case "json":
		nodes := 0
		var bounded func(any, int) bool
		bounded = func(value any, depth int) bool {
			nodes++
			if nodes > MaxJSONNodes || depth > MaxJSONDepth {
				return false
			}
			switch value := value.(type) {
			case map[string]any:
				for _, v := range value {
					if !bounded(v, depth+1) {
						return false
					}
				}
			case []any:
				for _, v := range value {
					if !bounded(v, depth+1) {
						return false
					}
				}
			}
			return true
		}
		if !bounded(v, 0) {
			return err()
		}
		b, e := json.Marshal(v)
		if e != nil || len(b) > MaxJSONBytes {
			return err()
		}
		switch v.(type) {
		case map[string]any, []any:
			return v, nil
		}
		return err()
	}
	return err()
}

func bounds(f Field, r *big.Rat) bool {
	if f.Minimum != nil {
		m, ok := rat(f.Minimum)
		if !ok || r.Cmp(m) < 0 {
			return false
		}
	}
	if f.Maximum != nil {
		m, ok := rat(f.Maximum)
		if !ok || r.Cmp(m) > 0 {
			return false
		}
	}
	return true
}

func (d Definition) Candidate(c Context, input map[string]any, create bool) (map[string]any, error) {
	c = d.Bind(c)
	out := map[string]any{}
	for k, v := range c.Record {
		out[k] = v
	}
	for k, v := range input {
		f, ok := d.Field(k)
		if !ok || slices.Contains(reserved, k) || f.Calculation != nil {
			return nil, Diagnostic{"read_only_field", k, "Unknown or read-only field."}
		}
		n, e := Value(f, v, false)
		if e != nil {
			return nil, e
		}
		out[k] = n
	}
	c.Record = out
	for _, f := range d.Fields {
		if f.Calculation != nil {
			continue
		}
		if _, exists := out[f.Key]; !exists && create {
			var v any
			if f.Default != nil {
				var e error
				v = defaultValue(*f.Default)
				if f.Default.Context != "" {
					v, e = Eval(*f.Default, c)
					if e != nil {
						return nil, e
					}
				}
			}
			n, e := Value(f, v, false)
			if e != nil {
				return nil, e
			}
			out[f.Key] = n
		}
	}
	order, e := d.CalculationOrder()
	if e != nil {
		return nil, e
	}
	for _, f := range order {
		v, e := Eval(*f.Calculation, c)
		if e != nil {
			return nil, e
		}
		n, e := Value(f, v, true)
		if e != nil {
			return nil, e
		}
		out[f.Key] = n
	}
	for _, f := range d.Fields {
		value, e := Value(f, out[f.Key], true)
		if e != nil {
			return nil, e
		}
		out[f.Key] = value
	}
	return out, nil
}

func (d Definition) CheckRules(c Context, create bool) error {
	c = d.Bind(c)
	for _, r := range d.Rules {
		if r.Kind == "change" && create {
			continue
		}
		if Allows(r.When, c) {
			v, e := Eval(r.Assert, c)
			if e != nil || !Truth(v) {
				return RuleFailure{Diagnostic: Diagnostic{"rule_failed", r.Key, r.Message}, Field: r.Field, Rule: r.Key}
			}
		}
	}
	return nil
}

func (d Definition) Bind(c Context) Context {
	c.DecimalFields = map[string]bool{}
	for _, f := range d.Fields {
		if f.Type == "decimal" {
			c.DecimalFields[f.Key] = true
		}
	}
	return c
}

func (d Definition) CheckCondition(x Expr, transition bool) error {
	var checkMath func(Expr) bool
	checkMath = func(e Expr) bool {
		if e.Decimal != "" {
			n, ok := rat(e.Decimal)
			if !ok {
				return false
			}
			scaled := new(big.Rat).Mul(n, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(MaxDecimalScale), nil)))
			bound := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(MaxDecimalPrecision), nil))
			if !scaled.IsInt() || new(big.Rat).Abs(n).Cmp(bound) >= 0 {
				return false
			}
		}
		if slices.Contains([]string{"add", "sub", "mul", "div", "round"}, e.Op) {
			return false
		}
		for _, a := range e.Args {
			if !checkMath(a) {
				return false
			}
		}
		return true
	}
	if !checkMath(x) {
		return fmt.Errorf("Conditions require stored arithmetic results and decimal literals within 30 integer digits and 12 decimal places.")
	}
	result, e := d.ExpressionType(x, transition)
	if e != nil {
		return e
	}
	if result != "boolean" {
		return fmt.Errorf("A condition must produce a boolean value.")
	}
	return nil
}

func (d Definition) CheckExpr(x Expr, transition bool) error {
	_, e := d.ExpressionType(x, transition)
	return e
}

var decimalPattern = regexp.MustCompile(`^-?[0-9]{1,64}(\.[0-9]{1,64})?$`)

// Prepare supplies trusted metadata before defaults/calculations so preview and
// live mutations see exactly the same resulting record semantics.
func (d Definition) Prepare(c Context, input map[string]any, create bool, id string, revision int64) (map[string]any, error) {
	base := map[string]any{}
	for k, v := range c.Record {
		base[k] = v
	}
	stamp := c.Now.UTC().Truncate(time.Microsecond).Format("2006-01-02T15:04:05.000000Z")
	base["id"] = id
	base["revision"] = revision
	base["updated_at"] = stamp
	base["updated_by"] = c.Actor
	if create {
		base["created_at"] = stamp
		base["created_by"] = c.Actor
	}
	c.Record = base
	return d.Candidate(c, input, create)
}

// RuleFailure retains public rule identity and optional field placement without
// exposing an expression or any record value.
type RuleFailure struct {
	Diagnostic
	Field string
	Rule  string
}

func (e RuleFailure) Unwrap() error { return e.Diagnostic }

type ContextProvider struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

var ContextProviders = []ContextProvider{{"actor.id", "Current user's ID", "user"}, {"request.time", "Current UTC time", "timestamp"}}
