package entity

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func sample() Definition {
	return Definition{Format: 1, Key: "deal", Label: "Deal", Fields: []Field{{Key: "name", Label: "Name", Type: "text", MaxLength: 200, Required: true}, {Key: "amount", Label: "Amount", Type: "decimal", Precision: 18, Scale: 2, Required: true}, {Key: "probability", Label: "Probability", Type: "integer", Required: true, Default: &Expr{Literal: json.Number("0")}}, {Key: "expected", Label: "Expected", Type: "decimal", Precision: 18, Scale: 2, Calculation: &Expr{Op: "div", Args: []Expr{{Op: "mul", Args: []Expr{{Field: "amount"}, {Field: "probability"}}}, {Literal: json.Number("100")}}}}, {Key: "owner", Label: "Owner", Type: "user", Default: &Expr{Context: "actor.id"}}}, Owner: "owner", Rules: []Rule{}, Permissions: []Permission{}}
}

func TestCandidate(t *testing.T) {
	d := sample()
	c := Context{Actor: "11111111-1111-4111-8111-111111111111", Now: time.Now()}
	v, e := d.Candidate(c, map[string]any{"name": "A", "amount": "10.05", "probability": json.Number("50")}, true)
	if e != nil || v["expected"] != "5.02" || v["owner"] != c.Actor {
		t.Fatal(v, e)
	}
	c.Record = v
	c.Old = v
	v, e = d.Candidate(c, map[string]any{"name": "B"}, false)
	if e != nil || v["amount"] != "10.05" {
		t.Fatal(v, e)
	}
	if _, e = d.Candidate(c, map[string]any{"amount": "0.001"}, false); e == nil {
		t.Fatal("silent scale coercion")
	}
	if _, e = d.Candidate(c, map[string]any{"expected": "3"}, false); e == nil {
		t.Fatal("client calculation accepted")
	}
	if _, e = d.Candidate(c, map[string]any{"name": nil}, false); e == nil {
		t.Fatal("required null accepted")
	}
}

func TestValidationAndCycles(t *testing.T) {
	d := sample()
	if ds := d.Validate(); len(ds) > 0 {
		t.Fatal(ds)
	}
	d.Fields[3].Calculation = &Expr{Field: "expected"}
	if len(d.Validate()) == 0 {
		t.Fatal("cycle accepted")
	}
	d = sample()
	p := Permission{Key: "bad", Label: "Bad", Operation: "update", Relationship: "member", Fields: []string{"expected"}}
	d.Permissions = []Permission{p}
	if len(d.Validate()) == 0 {
		t.Fatal("writable calculation")
	}
	_, e := Decode([]byte(`{"format_version":1,"unknown":true}`))
	if e == nil {
		t.Fatal("unknown contract property")
	}
}

func TestCompletePaths(t *testing.T) {
	d := sample()
	d.Permissions = []Permission{{Key: "name", Label: "Name", Operation: "update", Relationship: "member", Fields: []string{"name"}}, {Key: "amount", Label: "Amount", Operation: "update", Relationship: "member", Fields: []string{"amount"}}}
	c := Context{Actor: "joe", Record: map[string]any{"owner": "joe"}, Old: map[string]any{"owner": "joe"}}
	app := AppAccess{Inherit: true}
	if d.Allowed("entity", "update", []string{"name", "amount"}, c, app, nil) {
		t.Fatal("assembled unrelated permissions")
	}
	if !d.Allowed("entity", "update", []string{"name"}, c, app, nil) {
		t.Fatal("single valid permission rejected")
	}
	d.Permissions = []Permission{{Key: "owner", Operation: "update", Relationship: "owner", Fields: []string{"owner"}}}
	c.Record = map[string]any{"owner": "other"}
	if d.Allowed("entity", "update", []string{"owner"}, c, app, nil) {
		t.Fatal("owner escape")
	}
}

func TestReadFieldComposition(t *testing.T) {
	d := sample()
	d.Permissions = []Permission{{Key: "names", Operation: "read", Relationship: "member", Fields: []string{"name"}}, {Key: "salaries", Operation: "read", Relationship: "owner", Fields: []string{"amount"}}}
	sql, args, e := d.ReadSQL("entity", []string{"name", "amount"}, Context{Actor: "joe"}, AppAccess{Inherit: true}, nil)
	if e != nil || !strings.Contains(sql, " AND ") || len(args) != 1 || args[0] != "joe" {
		t.Fatal(sql, args, e)
	}
	app := AppAccess{Ceiling: []Gate{{Permission: PermissionID("entity", "names"), Fields: []string{"amount"}}}}
	sql, _, e = d.ReadSQL("entity", []string{"amount"}, Context{Actor: "joe"}, app, nil)
	if e != nil || !strings.Contains(sql, "1=0") {
		t.Fatal("mismatched permission ceiling", sql, e)
	}
}

func TestNullAndTypedArithmetic(t *testing.T) {
	d := sample()
	x := Expr{Op: "not", Args: []Expr{{Op: "eq", Args: []Expr{{Field: "name"}, {Literal: "A"}}}}}
	v, e := Eval(x, Context{Record: map[string]any{"name": nil}})
	if e != nil || v != true {
		t.Fatal(v, e)
	}
	sql, _, e := d.ExprSQL(&x, Context{})
	if e != nil || !strings.Contains(sql, "COALESCE") {
		t.Fatal(sql, e)
	}
	x = Expr{Op: "div", Args: []Expr{{Decimal: "1"}, {Decimal: "0"}}}
	if _, e = Eval(x, Context{}); e == nil {
		t.Fatal("zero division")
	}
}

func TestTransitionScope(t *testing.T) {
	d := sample()
	d.Permissions = []Permission{{Key: "approve", Operation: "update", Relationship: "member", Fields: []string{"name"}, ChangeWhen: &Expr{Op: "eq", Args: []Expr{{Field: "old.name"}, {Literal: "Pending"}}}}}
	c := Context{Old: map[string]any{"name": "Pending"}, Record: map[string]any{"name": "Approved"}}
	if !d.Allowed("e", "update", []string{"name"}, c, AppAccess{Inherit: true}, nil) {
		t.Fatal("transition incorrectly treated as record scope")
	}
}

func TestPredicateArithmeticIsExplicitlyBounded(t *testing.T) {
	d := sample()
	x := Expr{Op: "not", Args: []Expr{{Op: "eq", Args: []Expr{{Op: "div", Args: []Expr{{Field: "amount"}, {Field: "probability"}}}, {Decimal: "0"}}}}}
	if d.CheckCondition(x, false) == nil {
		t.Fatal("potentially failing arithmetic accepted in a SQL predicate")
	}
	if _, ok := rat(json.Number("1e999999999")); ok {
		t.Fatal("unbounded exponent accepted")
	}
	v, e := Eval(Expr{Op: "round", Args: []Expr{{Decimal: "2.345"}, {Literal: json.Number("2")}}}, Context{})
	r, ok := rat(v)
	if e != nil || !ok || round(r, 2) != "2.34" {
		t.Fatal(v, e)
	}
}

func TestSQLPredicateLiteralPrecision(t *testing.T) {
	d := sample()
	for _, literal := range []Expr{{Decimal: "0.1000000000001"}, {Decimal: "1000000000000000000000000000000"}, {Literal: json.Number("9007199254740992")}, {Literal: json.Number("1e300")}} {
		if d.CheckCondition(Expr{Op: "eq", Args: []Expr{{Field: "amount"}, literal}}, false) == nil {
			t.Fatal("unrepresentable SQL literal accepted", literal)
		}
	}
	x := Expr{Op: "eq", Args: []Expr{{Field: "amount"}, {Decimal: "123456789012345678901234567890"}}}
	if e := d.CheckCondition(x, false); e != nil {
		t.Fatal(e)
	}
	s, _, e := d.ExprSQL(&x, Context{})
	if e != nil || !strings.Contains(s, "DECIMAL(42,12)") {
		t.Fatal(s, e)
	}
}

func TestMembershipSQLExpansionIsBounded(t *testing.T) {
	d := sample()
	d.Team = "team"
	d.Fields = append(d.Fields, Field{Key: "team", Label: "Team", Type: "team"})
	for i := 0; i < 64; i++ {
		d.Permissions = append(d.Permissions, Permission{Key: fmt.Sprintf("read_%d", i), Label: "Team read", Operation: "read", Relationship: "team_member", Fields: []string{"name"}})
	}
	c := Context{Teams: map[string]bool{}}
	for i := 0; i < 200; i++ {
		c.Teams[fmt.Sprintf("team-%d", i)] = true
	}
	if _, _, e := d.ReadSQL("e", []string{"name"}, c, AppAccess{Inherit: true}, nil); e == nil {
		t.Fatal("unbounded membership expansion")
	}
}

func TestDefaultsAndGeneratedInputContract(t *testing.T) {
	d := sample()
	d.Fields[1].Default = &Expr{Decimal: "0.00"}
	d.Fields = append(d.Fields, Field{Key: "details", Label: "Details", Type: "json", Default: &Expr{Literal: map[string]any{"tags": []any{"new"}}}})
	if diagnostics := d.Validate(); len(diagnostics) > 0 {
		t.Fatal(diagnostics)
	}
	value, e := d.Prepare(Context{Actor: "11111111-1111-4111-8111-111111111111", Now: time.Now()}, map[string]any{"name": "Only required input"}, true, "22222222-2222-4222-8222-222222222222", 1)
	if e != nil || value["amount"] != "0.00" || value["details"] == nil {
		t.Fatal(value, e)
	}
	schema := d.Schema("create").(map[string]any)
	required := schema["required"].([]string)
	if len(required) != 1 || required[0] != "name" {
		t.Fatal("trusted defaults became required inputs", required)
	}
	if _, present := schema["properties"].(map[string]any)["expected"]; present {
		t.Fatal("calculated field became writable")
	}
	if MaxDecimalPrecision+MaxDecimalScale > 65 {
		t.Fatal("literal bounds exceed MariaDB's exact decimal cast")
	}
}

func TestPolicyPublicationDoesNotRequireRecordScan(t *testing.T) {
	before := sample()
	raw, _ := json.Marshal(before)
	after, _ := Decode(raw)
	after.Label = "Renamed"
	after.Projection = []string{"amount"}
	after.Permissions = []Permission{{Key: "reader", Label: "Reader", Operation: "read", Relationship: "member", Fields: []string{"name"}}}
	after.Fields[2].Default = &Expr{Literal: json.Number("10")}
	if NeedsRecordValidation(before, after) {
		t.Fatal("policy/default/label change would scan all records")
	}
	after.Fields = append(after.Fields, Field{Key: "notes", Label: "Notes", Type: "text", MaxLength: 200})
	if NeedsRecordValidation(before, after) {
		t.Fatal("nullable addition requires an unnecessary data scan")
	}
	after.Fields[1].Minimum = "1"
	if !NeedsRecordValidation(before, after) {
		t.Fatal("constraint change skipped existing-record validation")
	}
}
