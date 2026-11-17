package control

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

func TestEntityRuntime(t *testing.T) {
	s, c, operator, owner, outsider := foundationFixture(t)
	ctx := context.Background()
	defer s.dataPools.Close()
	oc := c.as(owner)
	tenant, space, app := registry.NewID(), registry.NewID(), registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "entities-" + tenant, "display_name": "Entity tests", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := s.DB.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	resource, _ := businessDatabaseFixture(t, s, tenant, false)
	base := "/v1/tenants/" + tenant
	oc.want("POST", base+"/memberships", registry.Row{"user_id": outsider}, 0, 201)
	oc.want("POST", base+"/spaces", registry.Row{"id": space, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": resource}}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": app, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner}, 0, 201)
	d := entity.Definition{Format: 1, Key: "deal", Label: "Deal", Fields: []entity.Field{{Key: "name", Label: "Name", Type: "text", MaxLength: 100, Required: true, Filterable: true, Sortable: true}, {Key: "amount", Label: "Amount", Type: "decimal", Precision: 18, Scale: 2, Required: true, Filterable: true, Sortable: true}, {Key: "owner", Label: "Owner", Type: "user", Default: &entity.Expr{Context: "actor.id"}}}, Owner: "owner", Projection: []string{"name", "amount"}, Rules: []entity.Rule{}, Permissions: []entity.Permission{{Key: "read", Label: "Read", Operation: "read", Relationship: "owner", Fields: []string{"name", "amount", "owner"}}, {Key: "create", Label: "Create", Operation: "create", Relationship: "member", Fields: []string{"name", "amount", "owner"}}, {Key: "edit", Label: "Edit", Operation: "update", Relationship: "member", Fields: []string{"name", "amount"}, ChangeWhen: &entity.Expr{Op: "eq", Args: []entity.Expr{{Field: "old.owner"}, {Context: "actor.id"}}}}, {Key: "delete", Label: "Delete", Operation: "delete", Relationship: "owner"}}}
	// Wide projections must reduce the implicit page and reject an oversized
	// explicit page, even when the current records happen to contain nulls.
	wideFields := []string{"details_a", "details_b", "details_c", "details_d"}
	for _, key := range wideFields {
		d.Fields = append(d.Fields, entity.Field{Key: key, Label: key, Type: "json"})
		d.Permissions[0].Fields = append(d.Permissions[0].Fields, key)
	}
	created := oc.want("POST", "/v1/spaces/"+space+"/entities", registry.Row{"definition": d}, 0, 200)
	eid := registry.Str(created["id"])
	c.want("GET", "/v1/entities/"+eid, nil, 0, 403)
	oc.want("POST", "/v1/entities/"+eid+"/publications", registry.Row{}, 1, 202)
	// A failed workspace read is not "no workspace": the job fails closed and
	// stays queued for a retry instead of skipping the owner's authority check.
	if _, e := s.publicationJob(ctx, failingWorkspaceRead{s.DB}); e == nil || isAPIStatus(e, 404) {
		t.Fatal("publication proceeded after the workspace read failed", e)
	}
	if v, e := registry.QueryRow(ctx, s.DB, "SELECT state FROM entity_versions WHERE entity_id=?", eid); e != nil || v["state"] != "queued" {
		t.Fatal("publication left the queue after the workspace read failed", v, e)
	}
	if _, e := s.publicationJob(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	published := oc.want("GET", "/v1/entities/"+eid, nil, 0, 200)
	if published["active_version_id"] == nil {
		t.Fatal(published["publications"])
	}
	appInfo := oc.want("GET", base+"/apps/"+app, nil, 0, 200)
	oc.want("PATCH", "/v1/apps/"+app+"/data-access", entity.AppAccess{Origin: "http://localhost:8789", Inherit: true}, registry.Integer(appInfo["revision"]), 200)
	origins := map[string]string{}
	dataReq := func(raw, method, path string, body any, revision int64, override ...string) (int, registry.Row) {
		t.Helper()
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/data/v1"+path, bytes.NewReader(b))
		origin := "http://localhost:8789"
		if value := origins[raw]; value != "" {
			origin = value
		}
		if len(override) > 0 {
			origin = override[0]
		}
		r.Header.Set("Origin", origin)
		if raw != "" {
			r.Header.Set("Authorization", "Bearer "+raw)
		}
		if revision > 0 {
			r.Header.Set("If-Match", registry.Str(revision))
		}
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		var v registry.Row
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Code, v
	}
	launch := func(client testClient, options ...string) string {
		appID, origin := app, "http://localhost:8789"
		if len(options) > 0 {
			appID, origin = options[0], options[1]
		}
		verifier := registry.Token(32)
		nonce := registry.Token(32)
		v := client.want("POST", "/v1/apps/"+appID+"/launch", registry.Row{"challenge": base64.RawURLEncoding.EncodeToString(hash(verifier)), "nonce": nonce}, 0, 200)
		code, result := dataReq("", "POST", "/session/exchange", registry.Row{"code": v["code"], "verifier": verifier, "nonce": nonce}, 0, origin)
		if code != 200 {
			t.Fatal(result)
		}
		if status, _ := dataReq("", "POST", "/session/exchange", registry.Row{"code": v["code"], "verifier": verifier, "nonce": nonce}, 0, origin); status != 401 {
			t.Fatal("exchange replay", status)
		}
		raw := registry.Str(result["access_token"])
		origins[raw] = origin
		return raw
	}
	bearer := launch(oc)
	other := launch(c.as(outsider))
	request := func(raw, method, path string, body any, rev int64, want int) registry.Row {
		t.Helper()
		status, v := dataReq(raw, method, path, body, rev)
		if status != want {
			t.Fatalf("%s %s: %d want %d: %v", method, path, status, want, v)
		}
		return v
	}
	record := registry.NewID()
	request(bearer, "POST", "/entities/deal/records", registry.Row{"id": record, "values": registry.Row{"name": "First", "amount": "10.00"}}, 0, 200)
	request(bearer, "POST", "/entities/deal/query", registry.Row{"select": wideFields, "limit": entity.Limits.MaxPage}, 0, 400)
	request(bearer, "POST", "/entities/deal/query", registry.Row{"select": wideFields}, 0, 200)
	q := request(bearer, "POST", "/entities/deal/query", registry.Row{"include_count": true, "sort": []registry.Row{{"field": "amount", "direction": "asc"}}}, 0, 200)
	if registry.Integer(q["count"]) != 1 {
		t.Fatal(q)
	}
	q = request(other, "POST", "/entities/deal/query", registry.Row{"include_count": true}, 0, 200)
	if registry.Integer(q["count"]) != 0 {
		t.Fatal("owner ACL leaked", q)
	}
	request(other, "GET", "/entities/deal/records/"+record, nil, 0, 404)
	request(other, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"name": "Forbidden transition"}}, 999999, 404)
	request(bearer, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"amount": "20.00"}}, 1, 200)
	request(bearer, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"amount": "30.00"}}, 1, 412)
	request(bearer, "POST", "/entities/deal/bulk", registry.Row{"items": []registry.Row{{"id": record, "operation": "update", "revision": 2, "values": registry.Row{"name": "Must rollback"}}, {"id": registry.NewID(), "operation": "create", "values": registry.Row{"name": "Invalid"}}}}, 0, 400)
	q = request(bearer, "GET", "/entities/deal/records/"+record, nil, 0, 200)
	if q["name"] != "First" {
		t.Fatal("partial bulk persisted", q)
	}
	request("", "GET", "/schema", nil, 0, 401)
	db, e := s.businessDB(ctx, resource, false)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var n int
	if e = db.QueryRow("SELECT COUNT(*) FROM " + data.Table(eid)).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	// Synthetic preview produces no record mutation and gives no impersonation token.
	oc.want("POST", "/v1/entities/"+eid+"/test", registry.Row{"definition": d, "user_id": outsider, "operation": "create", "values": registry.Row{"name": "Fake", "amount": "5.00"}}, 0, 200)
	if e = db.QueryRow("SELECT COUNT(*) FROM " + data.Table(eid)).Scan(&n); e != nil || n != 1 {
		t.Fatal("preview wrote records", n, e)
	}
	// Pagination applies the same authorized predicate to count and cursor pages.
	extras := []string{registry.NewID(), registry.NewID(), registry.NewID()}
	for i, id := range extras {
		request(bearer, "POST", "/entities/deal/records", registry.Row{"id": id, "values": registry.Row{"name": "Page " + registry.Str(i), "amount": "1.00"}}, 0, 200)
	}
	page := request(bearer, "POST", "/entities/deal/query", registry.Row{"select": []string{"name"}, "limit": 2, "include_count": true, "sort": []registry.Row{{"field": "name", "direction": "asc"}}}, 0, 200)
	if registry.Integer(page["count"]) != 4 || page["next_cursor"] == nil {
		t.Fatal(page)
	}
	next := request(bearer, "POST", "/entities/deal/query", registry.Row{"select": []string{"name"}, "limit": 2, "include_count": true, "sort": []registry.Row{{"field": "name", "direction": "asc"}}, "cursor": page["next_cursor"]}, 0, 200)
	if len(next["items"].([]any)) != 2 {
		t.Fatal(next)
	}
	request(bearer, "POST", "/entities/deal/query", registry.Row{"select": []string{"amount"}, "limit": 2, "cursor": page["next_cursor"]}, 0, 400)
	for _, id := range extras {
		request(bearer, "DELETE", "/entities/deal/records/"+id, nil, 1, 200)
	}
	// SQL and Go conditions agree on null, text collation and decimal comparisons.
	for _, expr := range []entity.Expr{{Op: "not", Args: []entity.Expr{{Op: "eq", Args: []entity.Expr{{Field: "owner"}, {Literal: nil}}}}}, {Op: "eq", Args: []entity.Expr{{Field: "name"}, {Literal: "First"}}}, {Op: "eq", Args: []entity.Expr{{Field: "name"}, {Literal: "first"}}}, {Op: "eq", Args: []entity.Expr{{Field: "name"}, {Literal: "First "}}}, {Op: "gt", Args: []entity.Expr{{Field: "amount"}, {Decimal: "2.00"}}}} {
		if e = d.CheckCondition(expr, false); e != nil {
			t.Fatal(e)
		}
		sqlExpr, args, e := d.ExprSQL(&expr, entity.Context{})
		if e != nil {
			t.Fatal(e)
		}
		var got bool
		if e = db.QueryRow("SELECT "+sqlExpr+" FROM "+data.Table(eid)+" r WHERE r.id=?", append(args, record)...).Scan(&got); e != nil {
			t.Fatal(e)
		}
		value, e := entity.Eval(expr, d.Bind(entity.Context{Record: map[string]any{"name": "First", "owner": owner, "amount": "20.00"}}))
		if e != nil || got != entity.Truth(value) {
			t.Fatal("SQL evaluator disagreement", expr, got, value, e)
		}
	}
	// Another App in the same Space sees the same entity and records, without copies.
	sharedApp := registry.NewID()
	oc.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": sharedApp, "slug": "shared", "display_name": "Shared backend App", "initial_manager_user_id": owner}, 0, 201)
	sharedInfo := oc.want("GET", base+"/apps/"+sharedApp, nil, 0, 200)
	oc.want("PATCH", "/v1/apps/"+sharedApp+"/data-access", entity.AppAccess{Origin: "http://127.0.0.3:8789", Inherit: true}, registry.Integer(sharedInfo["revision"]), 200)
	sharedBearer := launch(oc, sharedApp, "http://127.0.0.3:8789")
	sharedSchema := request(sharedBearer, "GET", "/schema", nil, 0, 200)
	if registry.Str(sharedSchema["entities"].([]any)[0].(map[string]any)["id"]) != eid {
		t.Fatal("App copied the entity contract", sharedSchema)
	}
	request(sharedBearer, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"name": "Shared backend"}}, 2, 200)
	sharedRecord := request(bearer, "GET", "/entities/deal/records/"+record, nil, 0, 200)
	if sharedRecord["name"] != "Shared backend" {
		t.Fatal("Apps do not share records", sharedRecord)
	}
	request(bearer, "PATCH", "/entities/deal/records/"+record, registry.Row{"values": registry.Row{"name": "First"}}, 3, 200)
	// Recover after a process stopped between registry activation and opening the local gate.
	exec("UPDATE entity_versions SET state='publishing' WHERE id=?", published["active_version_id"])
	schemaDB, e := s.businessDB(ctx, resource, true)
	if e != nil {
		t.Fatal(e)
	}
	defer schemaDB.Close()
	// A comparison cast must preserve a full 30-digit integer exactly.
	if _, e = schemaDB.Exec("CREATE TABLE wide_decimal_probe(f_amount DECIMAL(30,0) NOT NULL)"); e != nil {
		t.Fatal(e)
	}
	wide := "123456789012345678901234567890"
	if _, e = schemaDB.Exec("INSERT INTO wide_decimal_probe VALUES(?)", wide); e != nil {
		t.Fatal(e)
	}
	wideDef := d
	wideDef.Fields = append([]entity.Field{}, d.Fields...)
	wideDef.Fields[1].Precision = 30
	wideDef.Fields[1].Scale = 0
	for _, op := range []string{"eq", "ne", "lt", "gte"} {
		x := entity.Expr{Op: op, Args: []entity.Expr{{Field: "amount"}, {Decimal: wide}}}
		if e = wideDef.CheckCondition(x, false); e != nil {
			t.Fatal(e)
		}
		sqlExpr, args, e := wideDef.ExprSQL(&x, entity.Context{})
		if e != nil {
			t.Fatal(e)
		}
		var got bool
		if e = schemaDB.QueryRow("SELECT "+sqlExpr+" FROM wide_decimal_probe r", args...).Scan(&got); e != nil {
			t.Fatal(e)
		}
		value, e := entity.Eval(x, wideDef.Bind(entity.Context{Record: map[string]any{"amount": wide}}))
		if e != nil || got != entity.Truth(value) {
			t.Fatal("decimal cast changed authorization", op, got, value, e)
		}
	}

	if _, e = schemaDB.Exec("UPDATE _lastapp_entity_state SET gate_open=0 WHERE entity_id=?", eid); e != nil {
		t.Fatal(e)
	}
	if _, e = s.publicationJob(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	request(bearer, "GET", "/entities/deal/records/"+record, nil, 0, 200)
	// Two Spaces on one database reuse keys without sharing business records.
	space2, app2 := registry.NewID(), registry.NewID()
	oc.want("POST", base+"/spaces", registry.Row{"id": space2, "slug": "other", "display_name": "Other Space", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": resource}}, 0, 201)
	oc.want("POST", base+"/spaces/"+space2+"/apps", registry.Row{"id": app2, "slug": "other", "display_name": "Other App", "initial_manager_user_id": owner}, 0, 201)
	entity2 := oc.want("POST", "/v1/spaces/"+space2+"/entities", registry.Row{"definition": d}, 0, 200)
	oc.want("POST", "/v1/entities/"+registry.Str(entity2["id"])+"/publications", registry.Row{}, registry.Integer(entity2["revision"]), 202)
	if _, e = s.publicationJob(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	app2Info := oc.want("GET", base+"/apps/"+app2, nil, 0, 200)
	oc.want("PATCH", "/v1/apps/"+app2+"/data-access", entity.AppAccess{Origin: "http://127.0.0.2:8789", Inherit: true}, registry.Integer(app2Info["revision"]), 200)
	bearer2 := launch(oc, app2, "http://127.0.0.2:8789")
	request(bearer2, "GET", "/entities/deal/records/"+record, nil, 0, 404)
	request(bearer2, "POST", "/entities/deal/records", registry.Row{"id": record, "values": registry.Row{"name": "Other Space record", "amount": "999.00"}}, 0, 200)
	first := request(bearer, "GET", "/entities/deal/records/"+record, nil, 0, 200)
	if first["name"] != "First" {
		t.Fatal("Space isolation failed", first)
	}
	invalidRef := d
	invalidRef.Fields = append(append([]entity.Field{}, d.Fields...), entity.Field{Key: "foreign", Label: "Foreign", Type: "reference", Target: registry.Str(entity2["id"])})
	validated := oc.want("POST", "/v1/entities/"+eid+"/validate", registry.Row{"definition": invalidRef}, 0, 200)
	if len(validated["diagnostics"].([]any)) == 0 {
		t.Fatal("cross-Space reference accepted")
	}
	// Record sharing reuses the same permission identity, without duplicating roles.
	role := registry.NewID()
	oc.want("POST", base+"/spaces/"+space+"/roles", registry.Row{"id": role, "code": "reader", "display_name": "Reader"}, 0, 201)
	assignment := registry.NewID()
	oc.want("POST", base+"/spaces/"+space+"/role-assignments", registry.Row{"id": assignment, "role_id": role, "user_id": owner}, 0, 201)
	d.Sharing = true
	d.Permissions = append(d.Permissions, entity.Permission{Key: "shared_read", Label: "Shared read", Operation: "read", Roles: []string{role}, Fields: []string{"name", "amount"}}, entity.Permission{Key: "manage", Label: "Share access", Operation: "acl.manage", Relationship: "user", Subject: owner, Delegable: []string{"shared_read"}})
	publish := func() {
		t.Helper()
		latest := oc.want("GET", "/v1/entities/"+eid, nil, 0, 200)
		saved := oc.want("PATCH", "/v1/entities/"+eid, registry.Row{"definition": d}, registry.Integer(latest["revision"]), 200)
		oc.want("POST", "/v1/entities/"+eid+"/publications", registry.Row{}, registry.Integer(saved["revision"]), 202)
		if _, e := s.publicationJob(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
		latest = oc.want("GET", "/v1/entities/"+eid, nil, 0, 200)
		if latest["pending_version_id"] != nil || latest["publications"].([]any)[0].(map[string]any)["state"] != "published" {
			t.Fatal(latest["publications"])
		}
	}
	publish()
	pid := entity.PermissionID(eid, "shared_read")
	grant := registry.Row{"permission_id": pid, "recipient_kind": "user", "recipient_id": outsider}
	request(bearer, "POST", "/entities/deal/records/"+record+"/shares", grant, 0, 200)
	q = request(other, "POST", "/entities/deal/query", registry.Row{"include_count": true}, 0, 200)
	if registry.Integer(q["count"]) != 1 {
		t.Fatal("share not effective", q)
	}
	options := request(bearer, "GET", "/entities/deal/records/"+record+"/shares?view=options&recipient_kind=user", nil, 0, 200)
	if len(options["permissions"].([]any)) != 1 {
		t.Fatal(options)
	}
	oc.want("DELETE", base+"/spaces/"+space+"/role-assignments/"+assignment, nil, 1, 200)
	request(bearer, "POST", "/entities/deal/records/"+record+"/shares", grant, 0, 403)
	q = request(other, "POST", "/entities/deal/query", registry.Row{"include_count": true}, 0, 200)
	if registry.Integer(q["count"]) != 1 {
		t.Fatal("grantor revocation incorrectly cascaded", q)
	}
	oc.want("POST", base+"/spaces/"+space+"/role-assignments", registry.Row{"id": registry.NewID(), "role_id": role, "user_id": owner}, 0, 201)
	// Label edits preserve shares, security changes invalidate them.
	d.Permissions[4].Label = "Read label edited"
	publish()
	q = request(other, "POST", "/entities/deal/query", registry.Row{"include_count": true}, 0, 200)
	if registry.Integer(q["count"]) != 1 {
		t.Fatal("label revoked share", q)
	}
	d.Permissions[4].Fields = []string{"name"}
	publish()
	q = request(other, "POST", "/entities/deal/query", registry.Row{"include_count": true}, 0, 200)
	if registry.Integer(q["count"]) != 0 {
		t.Fatal("stale share effective", q)
	}
	// An approved App ceiling narrows both fields and records.
	access := oc.want("GET", "/v1/apps/"+app+"/data-access", nil, 0, 200)
	oc.want("PATCH", "/v1/apps/"+app+"/data-access", entity.AppAccess{Origin: "http://localhost:8789", Ceiling: []entity.Gate{{Permission: entity.PermissionID(eid, "read"), Fields: []string{"name"}}}}, registry.Integer(access["revision"]), 200)
	q = request(bearer, "POST", "/entities/deal/query", registry.Row{"select": []string{"name"}}, 0, 200)
	if len(q["items"].([]any)) != 1 {
		t.Fatal(q)
	}
	request(bearer, "POST", "/entities/deal/query", registry.Row{"select": []string{"name", "amount"}}, 0, 400)
	request(sharedBearer, "GET", "/entities/deal/records/"+record, nil, 0, 200)
	// Unsupported storage change fails safely and reopens the previous version.
	latest := oc.want("GET", "/v1/entities/"+eid, nil, 0, 200)
	d.Fields[0].MaxLength = 50
	saved := oc.want("PATCH", "/v1/entities/"+eid, registry.Row{"definition": d}, registry.Integer(latest["revision"]), 200)
	oc.want("POST", "/v1/entities/"+eid+"/publications", registry.Row{}, registry.Integer(saved["revision"]), 202)
	if _, e := s.publicationJob(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	latest = oc.want("GET", "/v1/entities/"+eid, nil, 0, 200)
	if latest["active_version_id"] == nil || latest["publications"].([]any)[0].(map[string]any)["state"] != "failed" {
		t.Fatal(latest["publications"])
	}
	request(bearer, "POST", "/entities/deal/query", registry.Row{"select": []string{"name"}}, 0, 200)
	exec("DELETE FROM sessions WHERE token_hash=?", hash(oc.session))
	request(bearer, "GET", "/schema", nil, 0, 401)
	request(bearer2, "GET", "/schema", nil, 0, 401)
	request(sharedBearer, "GET", "/schema", nil, 0, 401)
	_ = operator
}

// businessDatabaseFixture registers a real, empty business database on the
// local server with separate runtime and schema accounts, owned and dropped by
// the test. A managed-enabled target lets workspaces allocate beside it.
func businessDatabaseFixture(t *testing.T, s *Server, tenant string, managed bool) (resource, target string) {
	t.Helper()
	resource, target = registry.NewID(), registry.NewID()
	name := "entity_test_" + strings.ReplaceAll(registry.NewID(), "-", "")
	runtime, ddl := databases.Credentials{Username: "er_" + strings.ReplaceAll(registry.NewID(), "-", "")[:20], Password: registry.Token(32)}, databases.Credentials{Username: "es_" + strings.ReplaceAll(registry.NewID(), "-", "")[:20], Password: registry.Token(32)}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := s.DB.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec("CREATE DATABASE `" + name + "`")
	for _, cred := range []databases.Credentials{runtime, ddl} {
		exec("CREATE USER '" + cred.Username + "'@'" + s.Config.RuntimeAccountHost + "' IDENTIFIED BY '" + cred.Password + "'")
	}
	exec("GRANT SELECT,INSERT,UPDATE,DELETE ON `" + name + "`.* TO '" + runtime.Username + "'@'" + s.Config.RuntimeAccountHost + "'")
	exec("GRANT CREATE,ALTER,INDEX,REFERENCES,SELECT,INSERT,UPDATE,DELETE ON `" + name + "`.* TO '" + ddl.Username + "'@'" + s.Config.RuntimeAccountHost + "'")
	t.Cleanup(func() {
		s.dataPools.Close()
		if _, err := s.DB.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Error("fixture database cleanup failed", err)
		}
		for _, cr := range []databases.Credentials{runtime, ddl} {
			if _, err := s.DB.Exec("DROP USER '" + cr.Username + "'@'" + s.Config.RuntimeAccountHost + "'"); err != nil {
				t.Error("fixture account cleanup failed", err)
			}
		}
	})
	enc, _ := s.sealer.Seal("database", resource, tenant, runtime)
	schema, _ := s.sealer.Seal("schema", resource, tenant, ddl)
	if managed {
		exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode,managed_enabled,provisioning_credentials_enc) VALUES(?,'Test','127.0.0.1',33317,'disabled',1,X'00')", target)
	} else {
		exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode) VALUES(?,'Test','127.0.0.1',33317,'disabled')", target)
	}
	exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc,schema_credentials_enc) VALUES(?,?,?,?,'registered','shared','ready',?,?)", resource, tenant, target, name, enc, schema)
	return resource, target
}

func TestTenantDataFairness(t *testing.T) {
	var limiter inFlightTenants
	releases := []func(){}
	for i := 0; i < entity.MaxTenantDataRequests; i++ {
		release, ok := limiter.reserve("one")
		if !ok {
			t.Fatal("premature limit")
		}
		releases = append(releases, release)
	}
	if _, ok := limiter.reserve("one"); ok {
		t.Fatal("one Tenant exceeded its concurrency budget")
	}
	other, ok := limiter.reserve("two")
	if !ok {
		t.Fatal("another Tenant was blocked")
	}
	other()
	for _, release := range releases {
		release()
	}
	if len(limiter.counts) != 0 {
		t.Fatal("idle Tenant counters retained")
	}
}

// failingWorkspaceRead fails the publisher's workspace lookup with a database
// error (not a missing row) and passes every other query through.
type failingWorkspaceRead struct{ registry.Queryer }

func (q failingWorkspaceRead) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if strings.Contains(query, "FROM development_workspaces") {
		return q.Queryer.QueryRowContext(ctx, "SELECT no_such_column FROM development_workspaces")
	}
	return q.Queryer.QueryRowContext(ctx, query, args...)
}

func isAPIStatus(e error, status int) bool {
	var ae *registry.APIError
	return errors.As(e, &ae) && ae.Status == status
}
