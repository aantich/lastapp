package dataapi

import (
	"reflect"
	"testing"

	"github.com/aantich/lastapp/internal/entity"
)

func TestCursorSQL(t *testing.T) {
	d := entity.Definition{Fields: []entity.Field{{Key: "name", Type: "text"}, {Key: "amount", Type: "decimal", Precision: 12, Scale: 2}}}
	order := []dataSort{{"name", "asc"}, {"amount", "desc"}, {"id", "asc"}}
	got, args := cursorSQL(d, order, []any{"Acme", "10.50", "r1"})
	want := "(((r.`f_name` IS NULL OR r.`f_name`>?))" +
		" OR (r.`f_name` <=> ? AND (r.`f_amount` IS NULL OR r.`f_amount`<CAST(? AS DECIMAL(12,2))))" +
		" OR (r.`f_name` <=> ? AND r.`f_amount` <=> CAST(? AS DECIMAL(12,2)) AND (r.`id` IS NULL OR r.`id`>?)))"
	if got != want {
		t.Fatalf("cursor SQL:\n got %s\nwant %s", got, want)
	}
	if wantArgs := []any{"Acme", "Acme", "10.50", "Acme", "10.50", "r1"}; !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("cursor args %v, want %v", args, wantArgs)
	}
	// A NULL sort value is in the NULLS-last tail: only later keys advance it.
	got, args = cursorSQL(d, order[:1:1], []any{nil})
	if got != "1=0" || args != nil {
		t.Fatalf("all-NULL cursor: %q %v", got, args)
	}
	got, args = cursorSQL(d, []dataSort{{"name", "asc"}, {"id", "asc"}}, []any{nil, "r1"})
	if got != "((r.`f_name` <=> ? AND (r.`id` IS NULL OR r.`id`>?)))" || !reflect.DeepEqual(args, []any{nil, "r1"}) {
		t.Fatalf("NULL-prefix cursor: %q %v", got, args)
	}
}

func TestRecipientAndShareSQL(t *testing.T) {
	if got, args := teamRecipientSQL(entity.Context{Actor: "u"}); got != "1=0" || args != nil {
		t.Fatalf("no teams: %q %v", got, args)
	}
	c := entity.Context{Actor: "u", Teams: map[string]bool{"t2": true, "t1": true}}
	got, args := teamRecipientSQL(c)
	if got != "(acl.recipient_kind='team' AND acl.recipient_id IN (?,?))" || !reflect.DeepEqual(args, []any{"t1", "t2"}) {
		t.Fatalf("teams: %q %v", got, args)
	}
	p := entity.Permission{Key: "read_all"}
	rt := dataRuntime{entityID: "e", actor: c, permissionRevisions: map[string]int64{entity.PermissionID("e", "read_all"): 3}}
	if got, args := rt.shareSQL(p); got != "1=0" || args != nil {
		t.Fatalf("sharing disabled: %q %v", got, args)
	}
	rt.d.Sharing = true
	got, args = rt.shareSQL(p)
	want := "EXISTS(SELECT 1 FROM _lastapp_record_acl acl WHERE acl.tenant_id=r.tenant_id AND acl.space_id=r.space_id AND acl.entity_id=? AND acl.record_id=r.id AND acl.permission_id=? AND acl.security_revision=? AND ((acl.recipient_kind='user' AND acl.recipient_id=?) OR (acl.recipient_kind='team' AND acl.recipient_id IN (?,?))))"
	if got != want || !reflect.DeepEqual(args, []any{"e", entity.PermissionID("e", "read_all"), int64(3), "u", "t1", "t2"}) {
		t.Fatalf("share SQL: %q %v", got, args)
	}
}

func TestUniqueAndSortFields(t *testing.T) {
	if got := uniqueStrings([]string{"id", "revision", "name", "id"}); !reflect.DeepEqual(got, []string{"id", "revision", "name"}) {
		t.Fatal(got)
	}
	if got := sortFields([]dataSort{{"name", "asc"}, {"id", "desc"}}); !reflect.DeepEqual(got, []string{"name", "id"}) {
		t.Fatal(got)
	}
}

func TestGateHas(t *testing.T) {
	c := entity.Context{Roles: map[string]bool{"reviewer": true}}
	gs := []entity.Gate{{Permission: "p", Fields: []string{"name"}}, {Permission: "q", Fields: []string{"secret"}, Roles: []string{"auditor"}}}
	for _, tc := range []struct {
		id, field string
		want      bool
	}{{"p", "name", true}, {"p", "id", true}, {"p", "revision", true}, {"p", "other", false}, {"q", "secret", false}, {"x", "id", false}} {
		if got := gateHas(gs, tc.id, tc.field, c); got != tc.want {
			t.Errorf("gateHas(%s, %s) = %v", tc.id, tc.field, got)
		}
	}
	c.Roles["auditor"] = true
	if !gateHas(gs, "q", "secret", c) {
		t.Error("role-gated field not admitted for a role holder")
	}
}
