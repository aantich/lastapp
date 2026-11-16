package control

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

// sameJSON requires that got, an operation's typed registry result, marshals
// byte-for-byte like the untyped Row reader's result of the statement the
// operation ran before it was typed (keys, order, nulls, *_json documents,
// numbers and timestamps). A slice compares with every row; anything else
// with the first row. The statement must return at least one row.
func sameJSON(t *testing.T, q Queryer, got any, untyped string, args ...any) {
	t.Helper()
	v, e := rows(context.Background(), q, untyped, args...)
	if e != nil {
		t.Fatal(e)
	}
	if len(v) == 0 {
		t.Fatalf("no rows to compare for %s", untyped)
	}
	var want []byte
	if b, _ := json.Marshal(got); len(b) > 0 && b[0] == '[' {
		want, _ = json.Marshal(v)
	} else {
		want, _ = json.Marshal(v[0])
	}
	b, e := json.Marshal(got)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(want, b) {
		t.Fatalf("typed JSON differs for %s\nwant: %s\ngot:  %s", untyped, want, b)
	}
}

func TestRegistryRecordsMatchUntypedRows(t *testing.T) {
	s, c, p, tenant, _, app := agentSpaceFixture(t)
	ctx := context.Background()
	admin := Actor{Principal: Principal{ID: p.ID}, RequestID: newID()}
	limited := Row{"id": newID(), "code": "limited-" + newID()[:8], "display_name": "Limited", "allow_shared_database": true, "allow_managed_database": true, "default_layout": "shared", "max_spaces": 2, "max_apps": 0}
	var operator string
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject='operator'", s.Config.OIDCIssuer).Scan(&operator); e != nil {
		t.Fatal(e)
	}
	c.as(operator).want("POST", "/admin/v1/plans", limited, 0, 201)
	root := Actor{Principal: Principal{ID: operator}, RequestID: newID()}

	plans, e := s.inv.Plans(ctx, root, PageRequest{Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, plans.Items, "SELECT id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases,revision,created_at FROM plans ORDER BY display_name,id LIMIT 200")
	plan, e := s.inv.Plan(ctx, root, str(limited["id"]))
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, plan, "SELECT id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases,revision,created_at FROM plans WHERE id=?", limited["id"])

	for i, artifact := range []string{"", "https://example.test/a.js"} {
		in := microappCreateInput{namedInput: namedInput{ID: newID(), Slug: "typed-" + newID()[:8], Name: "Typed"}, Artifact: artifact, Order: i}
		if _, e = s.inv.CreateMicroapp(ctx, admin, tenant, app, in); e != nil {
			t.Fatal(e)
		}
		m, e := s.inv.Microapp(ctx, admin, tenant, in.ID)
		if e != nil {
			t.Fatal(e)
		}
		sameJSON(t, s.DB, m, "SELECT id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order,state,revision FROM microapp_installations WHERE tenant_id=? AND id=?", tenant, in.ID)
	}
	microapps, e := s.inv.Microapps(ctx, admin, tenant, app, PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, microapps.Items, "SELECT m.id,m.tenant_id,m.app_id,m.slug,m.display_name,m.artifact_ref,m.navigation_order,m.state,m.revision,m.created_at,a.display_name AS app_name,a.space_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.tenant_id=? AND m.app_id=? ORDER BY m.navigation_order,m.created_at DESC", tenant, app)
}
