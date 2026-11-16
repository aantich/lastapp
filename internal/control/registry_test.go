package control

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// rawFields is the undecoded top-level members of a console GET response.
func rawFields(t *testing.T, c testClient, path string) map[string]json.RawMessage {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("Origin", c.s.Config.PublicURL)
	r.AddCookie(&http.Cookie{Name: c.s.cookieName("lastapp_session"), Value: c.session})
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	out := map[string]json.RawMessage{}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatalf("GET %s: %d %s", path, w.Code, w.Body.String())
	}
	return out
}

// sameRaw is sameJSON for a member of a raw response.
func sameRaw(t *testing.T, q Queryer, got json.RawMessage, untyped string, args ...any) {
	t.Helper()
	var v any
	if e := json.Unmarshal(got, &v); e != nil {
		t.Fatal(e)
	}
	sameJSON(t, q, rawJSON(got), untyped, args...)
}

// rawJSON marshals as its bytes, unchanged.
type rawJSON json.RawMessage

func (r rawJSON) MarshalJSON() ([]byte, error) { return r, nil }

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

	// Tenants, Spaces and Apps, with a Tenant default layout set and unset.
	if _, e = s.DB.Exec("UPDATE tenants SET default_layout='shared' WHERE id=?", tenant); e != nil {
		t.Fatal(e)
	}
	c.as(operator).want("POST", "/admin/v1/tenants", Row{"id": newID(), "slug": "typed-" + newID()[:8], "display_name": "Typed", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": p.ID}, 0, 201)
	tenants, e := s.inv.Tenants(ctx, root, PageRequest{Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	tenantColumns := "t.id,t.slug,t.display_name,t.state,t.plan_id,t.default_layout,t.default_shared_database_id,t.revision,t.created_at,p.display_name AS plan_name"
	sameJSON(t, s.DB, tenants.Items, "SELECT "+tenantColumns+",(SELECT COUNT(*) FROM spaces sp WHERE sp.tenant_id=t.id AND sp.state<>'archived') AS space_count,(SELECT COUNT(*) FROM apps a WHERE a.tenant_id=t.id AND a.state<>'archived') AS app_count FROM tenants t JOIN plans p ON p.id=t.plan_id ORDER BY t.display_name,t.id LIMIT 200")
	one, e := s.inv.Tenant(ctx, root, tenant)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, one, "SELECT "+tenantColumns+" FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", tenant)
	spaces, e := s.inv.Spaces(ctx, admin, tenant, PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, spaces.Items, "SELECT sp.id,sp.tenant_id,sp.slug,sp.display_name,sp.database_resource_id,sp.database_layout,sp.state,sp.revision,sp.created_at,d.database_name,d.state AS database_state,(SELECT COUNT(*) FROM apps a WHERE a.space_id=sp.id AND a.state<>'archived') AS app_count FROM spaces sp JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.workspace_id IS NULL ORDER BY sp.created_at DESC,sp.id", tenant)
	space, e := s.inv.Space(ctx, admin, tenant, spaces.Items[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, space, "SELECT sp.id,sp.workspace_id,sp.tenant_id,t.display_name AS tenant_name,sp.slug,sp.display_name,sp.state,sp.revision,sp.database_resource_id,sp.database_layout,d.database_name,d.state AS database_state FROM spaces sp JOIN tenants t ON t.id=sp.tenant_id JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.id=?", tenant, space.ID)
	apps, e := s.inv.Apps(ctx, admin, tenant, "", PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, apps.Items, "SELECT a.id,a.tenant_id,a.space_id,sp.workspace_id,a.slug,a.display_name,a.state,a.compute_mode,a.revision,a.created_at,sp.display_name AS space_name,(SELECT COUNT(*) FROM microapp_installations m WHERE m.app_id=a.id AND m.state<>'archived') AS microapp_count FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND sp.workspace_id IS NULL ORDER BY a.created_at DESC,a.id", tenant)
	one2, e := s.inv.App(ctx, admin, tenant, app)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, one2, "SELECT a.id,a.tenant_id,t.display_name AS tenant_name,a.space_id,sp.workspace_id,a.active_release_id,a.slug,a.display_name,a.state,a.compute_mode,a.revision,sp.display_name AS space_name FROM apps a JOIN tenants t ON t.id=a.tenant_id JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND a.id=?", tenant, app)

	// The Tenant overview, with databases that need attention.
	var target string
	if e = s.DB.QueryRow("SELECT target_id FROM database_resources WHERE tenant_id=? LIMIT 1", tenant).Scan(&target); e != nil {
		t.Fatal(e)
	}
	for i, state := range []string{"failed", "pending"} {
		if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc,last_error_message) VALUES(?,?,?,?,'registered','shared',?,X'00',?)", newID(), tenant, target, "typed_attention_"+state, state, map[int]any{0: "Failed <check>", 1: nil}[i]); e != nil {
			t.Fatal(e)
		}
	}
	overview := rawFields(t, c.as(operator), "/v1/tenants/"+tenant+"/overview")
	sameRaw(t, s.DB, overview["tenant"], "SELECT t.id,t.slug,t.display_name,t.state,t.revision,t.plan_id,CASE WHEN t.default_layout='shared' AND p.allow_shared_database=1 THEN 'shared' WHEN t.default_layout='dedicated' AND p.allow_dedicated_database=1 THEN 'dedicated' ELSE p.default_layout END AS default_layout,p.display_name AS plan_name,p.max_spaces,p.max_apps,p.max_databases,p.allow_shared_database,p.allow_dedicated_database,p.allow_managed_database,p.allow_registered_database,p.allow_dedicated_compute FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", tenant)
	sameRaw(t, s.DB, overview["recent_apps"], "SELECT a.id,a.space_id,a.display_name,a.state,sp.display_name AS space_name FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND a.state<>'archived' AND NOT EXISTS(SELECT 1 FROM spaces dsp WHERE dsp.id=a.space_id AND dsp.workspace_id IS NOT NULL) ORDER BY a.created_at DESC,a.id LIMIT 6", tenant)
	sameRaw(t, s.DB, overview["attention"], "SELECT id,database_name,state,last_error_message FROM database_resources WHERE tenant_id=? AND state IN ('pending','provisioning','failed') ORDER BY created_at DESC LIMIT 6", tenant)
}
