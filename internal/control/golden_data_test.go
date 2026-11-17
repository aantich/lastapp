package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// App data API contracts (/data/v1): session exchange, CORS preflight, schema,
// query, mutation, bulk, capabilities, reference options and record shares,
// for both an authorized caller and a denied one.
func TestDataShapeGolden(t *testing.T) {
	s, c, _, owner, joe := foundationFixture(t)
	ctx := context.Background()
	rec := newShapeRecorder(t, c.h)
	oc, jc := c.as(owner), c.as(joe)
	tenant, space, app, role := registry.NewID(), registry.NewID(), registry.NewID(), registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "data-" + tenant[:8], "display_name": "Data golden", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	resource, _ := businessDatabaseFixture(t, s, tenant, false)
	base := "/v1/tenants/" + tenant
	oc.want("POST", base+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	oc.want("POST", base+"/spaces", registry.Row{"id": space, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": resource}}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": app, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/roles", registry.Row{"id": role, "code": "reader", "display_name": "Reader"}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/role-assignments", registry.Row{"id": registry.NewID(), "role_id": role, "user_id": owner}, 0, 201)
	d := entity.Definition{
		Format: 1, Key: "deal", Label: "Deal", Sharing: true, Owner: "owner", Projection: []string{"name", "amount"}, Rules: []entity.Rule{},
		Fields: []entity.Field{{Key: "name", Label: "Name", Type: "text", MaxLength: 100, Required: true, Filterable: true, Sortable: true}, {Key: "amount", Label: "Amount", Type: "decimal", Precision: 18, Scale: 2, Required: true, Filterable: true, Sortable: true}, {Key: "owner", Label: "Owner", Type: "user", Default: &entity.Expr{Context: "actor.id"}}},
		Permissions: []entity.Permission{
			{Key: "read", Label: "Read", Operation: "read", Relationship: "owner", Fields: []string{"name", "amount", "owner"}},
			{Key: "create", Label: "Create", Operation: "create", Relationship: "member", Fields: []string{"name", "amount", "owner"}},
			{Key: "edit", Label: "Edit", Operation: "update", Relationship: "owner", Fields: []string{"name", "amount"}},
			{Key: "delete", Label: "Delete", Operation: "delete", Relationship: "owner"},
			{Key: "shared_read", Label: "Shared read", Operation: "read", Roles: []string{role}, Fields: []string{"name", "amount"}},
			{Key: "manage", Label: "Share access", Operation: "acl.manage", Relationship: "user", Subject: owner, Delegable: []string{"shared_read"}},
		},
	}
	created := oc.want("POST", "/v1/spaces/"+space+"/entities", registry.Row{"definition": d}, 0, 200)
	eid := registry.Str(created["id"])
	oc.want("POST", "/v1/entities/"+eid+"/publications", registry.Row{}, 1, 202)
	if _, e := s.publisher.Job(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	origin := "http://localhost:5590"
	oc.want("PATCH", "/v1/apps/"+app+"/data-access", entity.AppAccess{Origin: origin, Inherit: true}, 1, 200)

	request := func(label, from, token, method, path string, body any, revision int64, status int) registry.Row {
		t.Helper()
		var reader *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			reader = bytes.NewReader(b)
		} else {
			reader = bytes.NewReader(nil)
		}
		r := httptest.NewRequest(method, "/data/v1"+path, reader)
		if from != "" {
			r.Header.Set("Origin", from)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if revision > 0 {
			r.Header.Set("If-Match", registry.Str(revision))
		}
		w := rec.raw(label, r, status)
		var v registry.Row
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return v
	}

	// Console launch and PKCE session exchange.
	rec.http(oc, "GET /v1/apps/{app}/launch-info", "GET", "/v1/apps/"+app+"/launch-info", nil, 0, 200)
	launch := func(client testClient, label string) string {
		t.Helper()
		verifier, nonce := registry.Token(32), registry.Token(32)
		code := rec.http(client, "POST /v1/apps/{app}/launch"+label, "POST", "/v1/apps/"+app+"/launch", registry.Row{"challenge": base64.RawURLEncoding.EncodeToString(registry.TokenHash(verifier)), "nonce": nonce}, 0, 200)
		exchange := registry.Row{"code": code["code"], "verifier": verifier, "nonce": nonce}
		v := request("POST /data/v1/session/exchange"+label, origin, "", "POST", "/session/exchange", exchange, 0, 200)
		if label == "" {
			request("POST /data/v1/session/exchange replay", origin, "", "POST", "/session/exchange", exchange, 0, 401)
		}
		return registry.Str(v["access_token"])
	}
	verifier, nonce := registry.Token(32), registry.Token(32)
	code := oc.want("POST", "/v1/apps/"+app+"/launch", registry.Row{"challenge": base64.RawURLEncoding.EncodeToString(registry.TokenHash(verifier)), "nonce": nonce}, 0, 200)
	request("POST /data/v1/session/exchange wrong verifier", origin, "", "POST", "/session/exchange", registry.Row{"code": code["code"], "verifier": registry.Token(32), "nonce": nonce}, 0, 401)
	request("POST /data/v1/session/exchange unapproved origin", "http://localhost:5599", "", "POST", "/session/exchange", registry.Row{"code": code["code"], "verifier": verifier, "nonce": nonce}, 0, 401)
	request("POST /data/v1/session/exchange malformed", origin, "", "POST", "/session/exchange", registry.Row{"code": 1}, 0, 400)
	rec.http(jc, "POST /v1/apps/{app}/launch outsider", "POST", "/v1/apps/"+app+"/launch", registry.Row{"challenge": base64.RawURLEncoding.EncodeToString(registry.TokenHash(verifier)), "nonce": nonce}, 0, 200)
	bearer := launch(oc, "")
	other := launch(jc, " member")

	// CORS and authentication.
	request("OPTIONS /data/v1/schema approved origin", origin, "", "OPTIONS", "/schema", nil, 0, 204)
	request("OPTIONS /data/v1/schema unapproved origin", "http://localhost:5599", "", "OPTIONS", "/schema", nil, 0, 403)
	request("GET /data/v1/schema without bearer", origin, "", "GET", "/schema", nil, 0, 401)
	request("GET /data/v1/schema invalid bearer", origin, registry.Token(32), "GET", "/schema", nil, 0, 401)
	request("GET /data/v1/schema wrong origin", "http://localhost:5599", bearer, "GET", "/schema", nil, 0, 403)
	request("GET /data/v1/schema", origin, bearer, "GET", "/schema", nil, 0, 200)
	request("GET /data/v1/schema member", origin, other, "GET", "/schema", nil, 0, 200)
	request("GET /data/v1/unknown", origin, bearer, "GET", "/unknown", nil, 0, 404)
	request("POST /data/v1/entities/{missing}/query", origin, bearer, "POST", "/entities/missing/query", registry.Row{}, 0, 404)

	// Mutations.
	record, second := registry.NewID(), registry.NewID()
	request("POST /data/v1/entities/{entity}/records", origin, bearer, "POST", "/entities/deal/records", registry.Row{"id": record, "values": registry.Row{"name": "First", "amount": "10.00"}}, 0, 200)
	request("POST /data/v1/entities/{entity}/records invalid", origin, bearer, "POST", "/entities/deal/records", registry.Row{"id": registry.NewID(), "values": registry.Row{"name": "Missing amount"}}, 0, 400)
	request("POST /data/v1/entities/{entity}/records unknown field", origin, bearer, "POST", "/entities/deal/records", registry.Row{"id": registry.NewID(), "values": registry.Row{"name": "x", "amount": "1.00", "secret": "x"}}, 0, 400)
	request("PATCH /data/v1/entities/{entity}/records/{id}", origin, bearer, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"amount": "20.00"}}, 1, 200)
	request("PATCH /data/v1/entities/{entity}/records/{id} stale", origin, bearer, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"amount": "30.00"}}, 1, 412)
	request("PATCH /data/v1/entities/{entity}/records/{id} without If-Match", origin, bearer, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"amount": "30.00"}}, 0, 428)
	request("PATCH /data/v1/entities/{entity}/records/{id} denied", origin, other, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"name": "Forbidden"}}, 2, 404)
	request("POST /data/v1/entities/{entity}/bulk", origin, bearer, "POST", "/entities/deal/bulk", registry.Row{"items": []registry.Row{{"id": second, "operation": "create", "values": registry.Row{"name": "Second", "amount": "5.00"}}}}, 0, 200)
	request("POST /data/v1/entities/{entity}/bulk invalid", origin, bearer, "POST", "/entities/deal/bulk", registry.Row{"items": []registry.Row{{"id": record, "operation": "update", "revision": 2, "values": registry.Row{"name": "Rollback"}}, {"id": registry.NewID(), "operation": "create", "values": registry.Row{"name": "Invalid"}}}}, 0, 400)
	request("DELETE /data/v1/entities/{entity}/records/{id} denied", origin, other, "DELETE", "/entities/deal/records/"+second, nil, 1, 404)
	request("DELETE /data/v1/entities/{entity}/records/{id}", origin, bearer, "DELETE", "/entities/deal/records/"+second, nil, 1, 200)

	// Queries.
	request("GET /data/v1/entities/{entity}/records/{id}", origin, bearer, "GET", "/entities/deal/records/"+record, nil, 0, 200)
	request("GET /data/v1/entities/{entity}/records/{id} denied", origin, other, "GET", "/entities/deal/records/"+record, nil, 0, 404)
	request("POST /data/v1/entities/{entity}/query", origin, bearer, "POST", "/entities/deal/query", registry.Row{"include_count": true, "limit": 1, "sort": []registry.Row{{"field": "name", "direction": "asc"}}}, 0, 200)
	request("POST /data/v1/entities/{entity}/query member", origin, other, "POST", "/entities/deal/query", registry.Row{"include_count": true}, 0, 200)
	request("POST /data/v1/entities/{entity}/query unknown field", origin, bearer, "POST", "/entities/deal/query", registry.Row{"select": []string{"secret"}}, 0, 400)
	request("POST /data/v1/entities/{entity}/query oversized page", origin, bearer, "POST", "/entities/deal/query", registry.Row{"limit": entity.Limits.MaxPage + 1}, 0, 400)
	request("POST /data/v1/entities/{entity}/capabilities", origin, bearer, "POST", "/entities/deal/capabilities", registry.Row{"ids": []string{record}}, 0, 200)
	request("POST /data/v1/entities/{entity}/capabilities member", origin, other, "POST", "/entities/deal/capabilities", registry.Row{"ids": []string{record}}, 0, 200)
	request("GET /data/v1/entities/{entity}/reference-options", origin, bearer, "GET", "/entities/deal/reference-options?field=owner", nil, 0, 200)
	request("GET /data/v1/entities/{entity}/reference-options member", origin, other, "GET", "/entities/deal/reference-options?field=owner", nil, 0, 200)
	request("GET /data/v1/entities/{entity}/reference-options unknown field", origin, bearer, "GET", "/entities/deal/reference-options?field=name", nil, 0, 404)

	// Record shares.
	shares := "/entities/deal/records/" + record + "/shares"
	grant := registry.Row{"permission_id": entity.PermissionID(eid, "shared_read"), "recipient_kind": "user", "recipient_id": joe}
	request("POST /data/v1/entities/{entity}/records/{id}/shares", origin, bearer, "POST", shares, grant, 0, 200)
	request("POST /data/v1/entities/{entity}/records/{id}/shares not shareable", origin, bearer, "POST", shares, registry.Row{"permission_id": entity.PermissionID(eid, "read"), "recipient_kind": "user", "recipient_id": joe}, 0, 400)
	request("POST /data/v1/entities/{entity}/records/{id}/shares denied", origin, other, "POST", shares, grant, 0, 404)
	request("GET /data/v1/entities/{entity}/records/{id}/shares", origin, bearer, "GET", shares, nil, 0, 200)
	request("GET /data/v1/entities/{entity}/records/{id}/shares options", origin, bearer, "GET", shares+"?view=options&recipient_kind=user", nil, 0, 200)
	request("POST /data/v1/entities/{entity}/query shared", origin, other, "POST", "/entities/deal/query", registry.Row{"include_count": true}, 0, 200)
	request("DELETE /data/v1/entities/{entity}/records/{id}/shares", origin, bearer, "DELETE", shares, grant, 0, 200)

	// Ending the console session ends the App session.
	if _, e := s.DB.Exec("DELETE FROM sessions WHERE token_hash=?", registry.TokenHash(oc.session)); e != nil {
		t.Fatal(e)
	}
	request("GET /data/v1/schema after console sign-out", origin, bearer, "GET", "/schema", nil, 0, 401)
	rec.golden("data_shapes.golden.txt")
}
