package authz

import "testing"

func TestScopes(t *testing.T) {
	for _, s := range []Scope{{Kind: Platform}, {Kind: Tenant, Tenant: "t"}, {Kind: Team, Tenant: "t", Team: "m"}, {Kind: Space, Tenant: "t", Space: "s"}, {Kind: App, Tenant: "t", App: "a"}} {
		if e := s.Validate(); e != nil {
			t.Fatal(s, e)
		}
	}
	for _, s := range []Scope{{}, {Kind: Tenant}, {Kind: Platform, Tenant: "t"}, {Kind: Tenant, Tenant: "t", Space: "s"}, {Kind: Team, Tenant: "t"}, {Kind: Team, Tenant: "t", Team: "m", App: "a"}, {Kind: App, Tenant: "t", Space: "s", App: "a"}} {
		if s.Validate() == nil {
			t.Fatal("invalid scope accepted", s)
		}
	}
}

func TestCatalogIsClosedAndImmutable(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Roles() {
		if seen[r.ID] {
			t.Fatal("duplicate role")
		}
		seen[r.ID] = true
		if !ValidKind(r.Scope) {
			t.Fatal("invalid role scope")
		}
		for _, p := range r.Permissions {
			if !IsControlAction(p) {
				t.Fatal("unknown action", p)
			}
		}
	}
	r := Roles()
	r[0].Permissions[0] = "deals.read"
	if Roles()[0].Permissions[0] != "platform.admin" || IsControlAction("deals.read") {
		t.Fatal("catalog mutation escaped")
	}
	if _, ok := Builtin("tenant-admin"); ok {
		t.Fatal("name treated as identity")
	}
}
