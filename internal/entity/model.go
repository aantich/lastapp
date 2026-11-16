// Package entity owns the portable, versioned business contract. All consumers
// (editor, evaluator, SQL adapter and client generators) use this model.
package entity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"slices"
	"strings"
)

const FormatVersion = 1

var (
	Limits     = struct{ Fields, Rules, Permissions, Depth, Nodes, Page, MaxPage, Bulk, ReadPaths, DataRequests int }{64, 64, 64, 8, 64, 50, 200, 100, 512, 64}
	Types      = []string{"text", "integer", "decimal", "boolean", "date", "timestamp", "choice", "reference", "user", "team", "json"}
	Operations = []string{"read", "create", "update", "delete", "acl.manage"}
	keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)
	reserved   = []string{"id", "revision", "created_at", "updated_at", "created_by", "updated_by", "tenant_id", "space_id"}
)

type Definition struct {
	Format      int          `json:"format_version"`
	Key         string       `json:"key"`
	Label       string       `json:"label"`
	Description string       `json:"description,omitempty"`
	Fields      []Field      `json:"fields"`
	Owner       string       `json:"owner_field,omitempty"`
	Team        string       `json:"team_field,omitempty"`
	Projection  []string     `json:"default_projection,omitempty"`
	Rules       []Rule       `json:"rules"`
	Permissions []Permission `json:"permissions"`
	Sharing     bool         `json:"sharing,omitempty"`
}
type Field struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Type        string            `json:"type"`
	Required    bool              `json:"required,omitempty"`
	MaxLength   int               `json:"max_length,omitempty"`
	Precision   int               `json:"precision,omitempty"`
	Scale       int               `json:"scale,omitempty"`
	Minimum     any               `json:"minimum,omitempty"`
	Maximum     any               `json:"maximum,omitempty"`
	Values      []string          `json:"values,omitempty"`
	ValueLabels map[string]string `json:"value_labels,omitempty"`
	Target      string            `json:"target,omitempty"`
	Unique      string            `json:"unique,omitempty"`
	Filterable  bool              `json:"filterable,omitempty"`
	Sortable    bool              `json:"sortable,omitempty"`
	Default     *Expr             `json:"default,omitempty"`
	Calculation *Expr             `json:"calculation,omitempty"`
}
type Expr struct {
	Op      string `json:"op,omitempty"`
	Args    []Expr `json:"args,omitempty"`
	Field   string `json:"field,omitempty"`
	Context string `json:"context,omitempty"`
	Literal any    `json:"literal,omitempty"`
	Decimal string `json:"decimal,omitempty"`
}
type Rule struct {
	Key     string `json:"key"`
	Name    string `json:"name,omitempty"`
	Kind    string `json:"kind"`
	When    *Expr  `json:"when,omitempty"`
	Assert  Expr   `json:"assert"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}
type Permission struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Operation    string   `json:"operation"`
	Roles        []string `json:"roles,omitempty"`
	Relationship string   `json:"relationship,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	Fields       []string `json:"fields,omitempty"`
	When         *Expr    `json:"when,omitempty"`
	ChangeWhen   *Expr    `json:"change_when,omitempty"`
	Delegable    []string `json:"delegable,omitempty"`
}
type Diagnostic struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (d Diagnostic) Error() string { return d.Message }
func Decode(b []byte) (Definition, error) {
	var d Definition
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if e := dec.Decode(&d); e != nil {
		return d, e
	}
	var more any
	if !errors.Is(dec.Decode(&more), io.EOF) {
		return d, fmt.Errorf("one definition required")
	}
	return d, nil
}

func Hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (d Definition) Field(key string) (Field, bool) {
	for _, f := range d.Fields {
		if f.Key == key {
			return f, true
		}
	}
	if slices.Contains(reserved[:6], key) {
		t := "text"
		if key == "revision" {
			t = "integer"
		}
		if strings.HasSuffix(key, "_at") {
			t = "timestamp"
		}
		if strings.HasSuffix(key, "_by") {
			t = "user"
		}
		return Field{Key: key, Type: t, Filterable: true, Sortable: true}, true
	}
	return Field{}, false
}

func (d Definition) Validate() []Diagnostic {
	out := []Diagnostic{}
	add := func(path, msg string) { out = append(out, Diagnostic{"invalid_definition", path, msg}) }
	if len(d.Description) > MaxDescriptionBytes {
		add("description", "Entity description exceeds its size limit.")
	}
	if d.Format != FormatVersion {
		add("format_version", "Unsupported definition format.")
	}
	if !keyPattern.MatchString(d.Key) || strings.TrimSpace(d.Label) == "" || len(d.Label) > 160 {
		add("key", "Use a stable lowercase key and a name of 1–160 bytes.")
	}
	if len(d.Fields) == 0 || len(d.Fields) > Limits.Fields || len(d.Rules) > Limits.Rules || len(d.Permissions) > Limits.Permissions {
		add("fields", "Definition exceeds field, rule or permission limits, or has no fields.")
	}
	estimatedRow, indexes := 1000, 1
	seen := map[string]bool{}
	for _, f := range d.Fields {
		p := "fields." + f.Key
		if f.Type == "text" {
			estimatedRow += f.MaxLength * 4
			if (f.Filterable || f.Sortable || f.Unique != "") && f.MaxLength > MaxIndexedTextLength {
				add(p, "Indexed text must be at most 512 characters.")
			}
		} else {
			estimatedRow += 640
		}
		if f.Filterable || f.Sortable || f.Unique != "" || f.Key == d.Owner || f.Key == d.Team || f.Type == "reference" {
			indexes++
		}
		if f.Type == "reference" && (f.Unique == "owner" || f.Unique == "team") {
			indexes++
		}
		if !keyPattern.MatchString(f.Key) || slices.Contains(reserved, f.Key) || seen[f.Key] {
			add(p, "Field keys must be unique, stable and non-reserved.")
		}
		seen[f.Key] = true
		if !slices.Contains(Types, f.Type) || f.Label == "" || len(f.Label) > 160 {
			add(p, "Choose a supported type and field name.")
		}
		if f.Type == "text" && (f.MaxLength < MinTextLength || f.MaxLength > MaxTextLength) {
			add(p, "Text length must be 1–1000 characters.")
		}
		if f.Type == "decimal" && (f.Precision < 1 || f.Precision > MaxDecimalPrecision || f.Scale < 0 || f.Scale > f.Precision || f.Scale > MaxDecimalScale) {
			add(p, "Invalid decimal precision or scale.")
		}
		if f.Minimum != nil || f.Maximum != nil {
			if f.Type != "integer" && f.Type != "decimal" {
				add(p, "Only numeric fields have numeric bounds.")
			}
			var min, maxValid bool
			var minValue, maxValue *big.Rat
			if f.Minimum != nil {
				minValue, min = rat(f.Minimum)
				if !min {
					add(p, "Invalid minimum.")
				}
			}
			if f.Maximum != nil {
				maxValue, maxValid = rat(f.Maximum)
				if !maxValid {
					add(p, "Invalid maximum.")
				}
			}
			if min && maxValid && minValue.Cmp(maxValue) > 0 {
				add(p, "Minimum exceeds maximum.")
			}
		}
		if f.Type == "choice" && (len(f.Values) == 0 || len(f.Values) > MaxChoiceValues) {
			add(p, "Choose 1–100 values.")
		}
		for key, label := range f.ValueLabels {
			if !slices.Contains(f.Values, key) || strings.TrimSpace(label) == "" || len(label) > 160 {
				add(p, "Choice labels must refer to a stable choice key and contain 1–160 bytes.")
			}
		}
		for _, v := range f.Values {
			if len(v) > 160 || v == "" {
				add(p, "Choice values must be nonempty and bounded.")
			}
		}
		if f.Type == "reference" && f.Target == "" {
			add(p, "Choose a target entity.")
		}
		if f.Unique != "" && !slices.Contains([]string{"entity", "owner", "team"}, f.Unique) {
			add(p, "Unknown uniqueness scope.")
		}
		if f.Unique == "owner" {
			v, ok := d.Field(d.Owner)
			if !ok || !v.Required {
				add(p, "Owner-scoped uniqueness requires a required owner field.")
			}
		}
		if f.Unique == "team" {
			v, ok := d.Field(d.Team)
			if !ok || !v.Required {
				add(p, "Team-scoped uniqueness requires a required team field.")
			}
		}
		if f.Type == "json" && (f.Sortable || f.Filterable || f.Unique != "") {
			add(p, "JSON fields do not support query indexes or uniqueness.")
		}
		if f.Default != nil && f.Calculation != nil {
			add(p, "A calculated field cannot also have a default.")
		}
		for _, x := range []*Expr{f.Default, f.Calculation} {
			if x != nil {
				if e := d.CheckExpr(*x, false); e != nil {
					add(p, e.Error())
				}
			}
		}
		if f.Default != nil && f.Default.Context == "" {
			if _, e := Value(f, defaultValue(*f.Default), false); e != nil {
				add(p, "Default does not match the field type.")
			}
		}
		if f.Default != nil && f.Default.Context != "" {
			if f.Default.Context == "actor.id" && f.Type != "user" && f.Type != "text" || f.Default.Context == "request.time" && f.Type != "timestamp" && f.Type != "text" {
				add(p, "Context default does not match the field type.")
			}
		}
		if f.Calculation != nil {
			if result, e := d.ExpressionType(*f.Calculation, false); e == nil && result != f.Type && result != "null" && !(result == "decimal" && f.Type == "integer" || result == "integer" && f.Type == "decimal") {
				add(p, "Calculation result type does not match this field.")
			}
		}
		if f.Default != nil && (f.Default.Op != "" || f.Default.Field != "") {
			add(p, "Defaults are literals or trusted context values.")
		}
		if f.Calculation != nil {
			for _, c := range contexts(*f.Calculation) {
				if c != "" {
					add(p, "Calculations cannot depend on actor or time context.")
				}
			}
		}
	}
	if estimatedRow > 60000 || indexes > 64 {
		add("fields", "Definition exceeds the adapter row-size or index limit.")
	}
	for _, v := range []struct{ k, t string }{{d.Owner, "user"}, {d.Team, "team"}} {
		if v.k != "" {
			f, ok := d.Field(v.k)
			if !ok || f.Type != v.t {
				add(v.k, "Invalid owner/team field.")
			}
		}
	}
	if _, e := d.CalculationOrder(); e != nil {
		add("fields", e.Error())
	}
	seen = map[string]bool{}
	for _, r := range d.Rules {
		if r.Field != "" {
			if _, ok := d.Field(r.Field); !ok {
				add("rules."+r.Key, "Unknown error-placement field.")
			}
		}
		for _, x := range []*Expr{r.When, &r.Assert} {
			if x != nil && len(contexts(*x)) > 0 {
				add("rules."+r.Key, "Entity invariants cannot depend on actor or time; use a permission change condition.")
			}
		}
		if len(r.Name) > 160 {
			add("rules."+r.Key, "Rule name exceeds its size limit.")
		}
		if seen[r.Key] || !keyPattern.MatchString(r.Key) || !slices.Contains([]string{"validation", "change"}, r.Kind) || r.Message == "" || len(r.Message) > 500 {
			add("rules."+r.Key, "Invalid rule key, kind or message.")
		}
		seen[r.Key] = true
		for _, x := range []*Expr{r.When, &r.Assert} {
			if x != nil {
				if e := d.CheckCondition(*x, r.Kind == "change"); e != nil {
					add("rules."+r.Key, e.Error())
				}
			}
		}
	}
	seen = map[string]bool{}
	for _, p := range d.Permissions {
		path := "permissions." + p.Key
		if len(p.Fields) > len(d.Fields)+len(reserved) || len(p.Roles) > 100 || len(p.Delegable) > Limits.Permissions {
			add(path, "Permission field/role/delegation list exceeds its limit.")
			continue
		}
		for _, list := range [][]string{p.Fields, p.Roles, p.Delegable} {
			if !uniqueKeys(list) {
				add(path, "Permission lists must not contain duplicates.")
			}
		}
		if seen[p.Key] || !keyPattern.MatchString(p.Key) || !slices.Contains(Operations, p.Operation) || strings.TrimSpace(p.Label) == "" || len(p.Label) > 160 {
			add(path, "Invalid permission key or operation.")
		}
		seen[p.Key] = true
		if (len(p.Roles) > 0) == (p.Relationship != "") {
			add(path, "Choose roles or one relationship.")
		}
		if len(p.Roles) > 100 {
			add(path, "Too many roles.")
		}
		if p.Relationship != "" && !slices.Contains([]string{"member", "owner", "team_member", "team_reviewer", "team_admin", "user", "team"}, p.Relationship) {
			add(path, "Unsupported relationship.")
		}
		if p.Relationship == "owner" && d.Owner == "" {
			add(path, "Select an owner field first.")
		}
		if strings.HasPrefix(p.Relationship, "team_") && d.Team == "" {
			add(path, "Select a team field first.")
		}
		if (p.Relationship == "user" || p.Relationship == "team") && p.Subject == "" {
			add(path, "Choose a recipient.")
		}
		if p.Operation == "create" {
			for _, field := range d.Fields {
				if field.Required && field.Default == nil && field.Calculation == nil && !slices.Contains(p.Fields, field.Key) {
					add(path, "A create permission must include every required input field without a default.")
				}
			}
		}
		if slices.Contains([]string{"read", "create", "update"}, p.Operation) && len(p.Fields) == 0 {
			add(path, "Select fields explicitly.")
		}
		for _, k := range p.Fields {
			f, ok := d.Field(k)
			if !ok {
				add(path, "Unknown field: "+k)
			}
			if p.Operation != "read" && (f.Calculation != nil || slices.Contains(reserved, k)) {
				add(path, "System and calculated fields are read-only.")
			}
		}
		for _, x := range []*Expr{p.When, p.ChangeWhen} {
			if x != nil {
				if e := d.CheckCondition(*x, x == p.ChangeWhen); e != nil {
					add(path, e.Error())
				}
			}
		}
		if p.ChangeWhen != nil && p.Operation != "update" {
			add(path, "Change conditions apply only to updates.")
		}
	}
	for _, p := range d.Permissions {
		for _, k := range p.Delegable {
			found := false
			for _, v := range d.Permissions {
				if v.Key == k && len(v.Roles) > 0 && v.Operation != "acl.manage" && v.Operation != "create" {
					found = true
				}
			}
			if !found || p.Operation != "acl.manage" {
				add("permissions."+p.Key, "Delegation must reference a role-activated read/update/delete permission.")
			}
		}
	}
	if len(d.Projection) > len(d.Fields)+len(reserved) || !uniqueKeys(d.Projection) {
		add("default_projection", "Projection must contain unique bounded field keys.")
	}
	for _, k := range d.Projection {
		if _, ok := d.Field(k); !ok {
			add("default_projection", "Unknown projection field.")
		}
	}
	return out
}

func contexts(x Expr) []string {
	out := []string{}
	if x.Context != "" {
		out = append(out, x.Context)
	}
	for _, a := range x.Args {
		out = append(out, contexts(a)...)
	}
	return out
}

func (d Definition) CalculationOrder() ([]Field, error) {
	out := []Field{}
	state := map[string]int{}
	var visit func(Field) error
	visit = func(f Field) error {
		if state[f.Key] == 1 {
			return fmt.Errorf("Calculated field dependency cycle.")
		}
		if state[f.Key] == 2 {
			return nil
		}
		state[f.Key] = 1
		if f.Calculation != nil {
			for _, k := range Fields(*f.Calculation) {
				g, ok := d.Field(strings.TrimPrefix(k, "new."))
				if !ok {
					return fmt.Errorf("Unknown calculation field.")
				}
				if g.Calculation != nil {
					if e := visit(g); e != nil {
						return e
					}
				}
			}
			out = append(out, f)
		}
		state[f.Key] = 2
		return nil
	}
	for _, f := range d.Fields {
		if e := visit(f); e != nil {
			return nil, e
		}
	}
	return out, nil
}

func Capabilities() any {
	return map[string]any{"format_version": FormatVersion, "types": Types, "operations": Operations, "operators": Operators, "relationships": []string{"member", "owner", "team_member", "team_reviewer", "team_admin", "user", "team"}, "limits": Limits, "templates": Templates(), "field_defaults": FieldDefaults(), "field_constraints": FieldConstraints(), "context_providers": ContextProviders}
}

func defaultValue(x Expr) any {
	if x.Decimal != "" {
		return x.Decimal
	}
	return x.Literal
}

func Templates() []Definition {
	base := func(key, label string, fields []Field) Definition {
		return Definition{Format: FormatVersion, Key: key, Label: label, Fields: fields, Projection: []string{fields[0].Key}, Rules: []Rule{}, Permissions: []Permission{}}
	}
	name := Field{Key: "name", Label: "Name", Type: "text", MaxLength: 200, Required: true, Filterable: true, Sortable: true}
	return []Definition{base("company", "Company", []Field{name, {Key: "website", Label: "Website", Type: "text", MaxLength: 500}}), base("contact", "Contact", []Field{name, {Key: "email", Label: "Email", Type: "text", MaxLength: 320, Filterable: true}}), base("deal", "Deal", []Field{name, {Key: "amount", Label: "Amount", Type: "decimal", Precision: 18, Scale: 2, Required: true, Minimum: "0", Filterable: true, Sortable: true}, {Key: "stage", Label: "Stage", Type: "choice", Required: true, Values: []string{"new", "qualified", "won", "lost"}, Default: &Expr{Literal: "new"}, Filterable: true}})}
}

func FieldDefaults() map[string]Field {
	out := map[string]Field{}
	for _, t := range Types {
		f := Field{Type: t}
		if t == "text" {
			f.MaxLength = 200
		}
		if t == "decimal" {
			f.Precision = 18
			f.Scale = 2
		}
		out[t] = f
	}
	return out
}

func ValidKey(key string) bool { return keyPattern.MatchString(key) }

func uniqueKeys(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

// Policy, label, projection and default changes never require scanning records.
// Only resulting-record constraints and validation assertions do. Transition
// rules apply to future changes; stored formula changes have a separate backfill
// check in the publisher.
func NeedsRecordValidation(before, after Definition) bool {
	constraints := func(d Definition) any {
		out := map[string]any{}
		for _, f := range d.Fields {
			out[f.Key] = struct {
				Required         bool
				Minimum, Maximum any
				Values           []string
			}{f.Required, f.Minimum, f.Maximum, f.Values}
		}
		return out
	}
	// A new nullable field has null values on existing rows and is valid by itself.
	previous := map[string]bool{}
	for _, f := range before.Fields {
		previous[f.Key] = true
	}
	trimmed := after
	trimmed.Fields = nil
	for _, f := range after.Fields {
		if previous[f.Key] || f.Required {
			trimmed.Fields = append(trimmed.Fields, f)
		}
	}
	if Hash(constraints(before)) != Hash(constraints(trimmed)) {
		return true
	}
	rules := func(d Definition) []string {
		out := []string{}
		for _, r := range d.Rules {
			if r.Kind == "validation" {
				out = append(out, Hash([]any{r.When, r.Assert}))
			}
		}
		slices.Sort(out)
		return out
	}
	return Hash(rules(before)) != Hash(rules(after))
}
