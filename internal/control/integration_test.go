package control

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

type testClient struct {
	t             *testing.T
	s             *Server
	h             http.Handler
	session, csrf string
}

func (c testClient) req(method, path string, body any, revision int64) (int, registry.Row) {
	c.t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Origin", c.s.Config.PublicURL)
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.Header.Set("Content-Type", "application/json")
	if revision > 0 {
		r.Header.Set("If-Match", registry.Str(revision))
	}
	r.AddCookie(&http.Cookie{Name: httpapi.CookieName(c.s.Config.PublicURL, "lastapp_session"), Value: c.session})
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	var out registry.Row
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		if w.Code >= 400 {
			return w.Code, registry.Row{"message": w.Body.String()}
		}
		c.t.Fatalf("invalid response %d %s", w.Code, w.Body.String())
	}
	return w.Code, out
}

func (c testClient) want(method, path string, body any, revision int64, status int) registry.Row {
	c.t.Helper()
	got, v := c.req(method, path, body, revision)
	if got != status {
		c.t.Fatalf("%s %s: got %d want %d: %v", method, path, got, status, v)
	}
	return v
}

func (c testClient) as(uid string) testClient {
	c.t.Helper()
	session, csrf := registry.Token(32), registry.Token(32)
	_, e := c.s.DB.Exec("INSERT INTO sessions (token_hash,user_id,csrf_hash,expires_at) VALUES (?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))", registry.TokenHash(session), uid, registry.TokenHash(csrf))
	if e != nil {
		c.t.Fatal(e)
	}
	c.session, c.csrf = session, csrf
	return c
}

func (c testClient) waitDB(tenant, record, state string) registry.Row {
	c.t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		_, v := c.req("GET", "/v1/tenants/"+tenant+"/database-resources/"+record, nil, 0)
		if registry.Str(v["state"]) == state {
			return v
		}
		if registry.Str(v["state"]) == "failed" && state != "failed" {
			c.t.Fatalf("provision failed: %v", v)
		}
		time.Sleep(150 * time.Millisecond)
	}
	c.t.Fatal("database status timeout")
	return nil
}

func TestMariaDBControlPlane(t *testing.T) {
	configPath := os.Getenv("LASTAPP_TEST_CONFIG")
	if configPath == "" {
		t.Skip("set LASTAPP_TEST_CONFIG to a running local development config")
	}
	cfg, e := LoadConfig(configPath)
	if e != nil {
		t.Fatal(e)
	}
	if !cfg.Development {
		t.Fatal("integration tests require local development")
	}
	s, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	if _, e = s.DB.Exec("UPDATE audit_events SET action=action WHERE 1=0"); e == nil {
		t.Fatal("runtime account can modify audit history")
	}
	if _, e = s.DB.Exec("DELETE FROM audit_events WHERE 1=0"); e == nil {
		t.Fatal("runtime account can delete audit history")
	}
	h := s.Handler()
	ctx := context.Background()
	var operator, owner, outsider string
	for sub, dest := range map[string]*string{"operator": &operator, "tenant-admin": &owner, "outsider": &outsider} {
		if e = s.DB.QueryRow("SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject=?", cfg.OIDCIssuer, sub).Scan(dest); e != nil {
			t.Fatal(e)
		}
	}
	c := (testClient{t: t, s: s, h: h}).as(operator)
	tenant := registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "test-" + tenant, "display_name": "Integration " + tenant[:8], "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	base := "/v1/tenants/" + tenant
	ownerClient := c.as(owner)
	out := c.as(outsider)
	out.want("GET", base+"/spaces", nil, 0, 404)
	ownerClient.want("GET", "/admin/v1/tenants", nil, 0, 403)
	ownerClient.want("PATCH", base+"/memberships/"+owner, registry.Row{"state": "disabled"}, 1, 409)
	target := "20000000-0000-4000-8000-000000000001"
	spaceIDs := []string{registry.NewID(), registry.NewID()}
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i], _ = ownerClient.req("POST", base+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": spaceIDs[i], "slug": "shared-" + registry.Str(i), "display_name": "Shared " + registry.Str(i), "database": registry.Row{"mode": "managed", "layout": "shared", "target_id": target}}, 0)
		}(i)
	}
	wg.Wait()
	for _, status := range statuses {
		if status != 201 {
			t.Fatalf("concurrent shared create: %v", statuses)
		}
	}
	sp := ownerClient.want("GET", base+"/spaces", nil, 0, 200)["items"].([]any)
	first := sp[0].(map[string]any)
	second := sp[1].(map[string]any)
	if first["database_resource_id"] != second["database_resource_id"] {
		t.Fatal("shared Spaces allocated different databases")
	}
	shared := registry.Str(first["database_resource_id"])
	ready := c.waitDB(tenant, shared, "ready")
	t.Log("concurrent shared reuse and real provisioning passed")
	// Idempotency must not allocate an orphan database on a duplicate parent ID.
	ownerClient.want("POST", base+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": spaceIDs[0], "slug": "duplicate", "display_name": "Duplicate", "database": registry.Row{"mode": "managed", "layout": "dedicated", "target_id": target}}, 0, 409)
	var n int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM database_resources WHERE tenant_id=?", tenant).Scan(&n); e != nil {
		t.Fatal(e)
	}
	if n != 1 {
		t.Fatal("duplicate Space allocated a database")
	}
	// Verify a ready resource again using the same names, password and ownership token.
	ownerClient.want("POST", base+"/database-resources/"+shared+"/retry", registry.Row{}, registry.Integer(ready["revision"]), 202)
	c.waitDB(tenant, shared, "ready")
	// Dedicated binding is exclusive, even within a Tenant.
	dedicated := registry.NewID()
	ownerClient.want("POST", base+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": dedicated, "slug": "dedicated", "display_name": "Dedicated", "database": registry.Row{"mode": "managed", "layout": "dedicated", "target_id": target}}, 0, 201)
	sp = c.want("GET", base+"/spaces", nil, 0, 200)["items"].([]any)
	dedDB := ""
	for _, v := range sp {
		m := v.(map[string]any)
		if m["id"] == dedicated {
			dedDB = registry.Str(m["database_resource_id"])
		}
	}
	c.waitDB(tenant, dedDB, "ready")
	ownerClient.want("POST", base+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": registry.NewID(), "slug": "illegal-reuse", "display_name": "Illegal reuse", "database": registry.Row{"resource_id": dedDB}}, 0, 409)
	// App-scoped grants filter collection reads and deny sibling writes.
	apps := []string{registry.NewID(), registry.NewID()}
	for i, app := range apps {
		ownerClient.want("POST", base+"/spaces/"+spaceIDs[0]+"/apps", registry.Row{"initial_manager_user_id": owner, "id": app, "slug": "app-" + registry.Str(i), "display_name": "App " + registry.Str(i)}, 0, 201)
	}
	ownerClient.want("POST", base+"/memberships", registry.Row{"user_id": outsider}, 0, 201)
	ownerClient.want("POST", base+"/apps/"+apps[0]+"/role-assignments", registry.Row{"id": registry.NewID(), "user_id": outsider, "role_id": authz.AppManagerRole}, 0, 201)
	list := out.want("GET", base+"/apps", nil, 0, 200)["items"].([]any)
	if len(list) != 1 || registry.Str(list[0].(map[string]any)["id"]) != apps[0] {
		t.Fatal("App scope leaked a sibling")
	}
	paged := out.want("GET", base+"/apps?page=1&limit=1", nil, 0, 200)["items"].([]any)
	if len(paged) != 1 || registry.Str(paged[0].(map[string]any)["id"]) != apps[0] {
		t.Fatal("pagination did not preserve permission filtering")
	}
	if len(out.want("GET", base+"/apps?page=2&limit=1", nil, 0, 200)["items"].([]any)) != 0 {
		t.Fatal("pagination leaked a sibling")
	}
	out.want("GET", base+"/apps?limit=100000", nil, 0, 400)
	out.want("PATCH", base+"/apps/"+apps[1], registry.Row{"display_name": "Forbidden"}, 1, 403)
	out.want("PATCH", base+"/apps/"+apps[0], registry.Row{"display_name": "Allowed"}, 1, 200)
	out.want("PATCH", base+"/apps/"+apps[0], registry.Row{"display_name": "Stale"}, 1, 412)
	out.want("POST", base+"/spaces/"+spaceIDs[0]+"/apps", registry.Row{"initial_manager_user_id": owner, "id": registry.NewID(), "slug": "forbidden", "display_name": "Forbidden"}, 0, 403)
	group := registry.NewID()
	ownerClient.want("POST", base+"/teams", registry.Row{"id": group, "slug": "reviewers", "display_name": "Reviewers"}, 0, 201)
	ownerClient.want("POST", base+"/teams/"+group+"/members", registry.Row{"user_id": outsider}, 1, 200)
	role := registry.NewID()
	rolePath := base + "/spaces/" + spaceIDs[0] + "/roles"
	ownerClient.want("POST", rolePath, registry.Row{"id": role, "code": "reviewer", "display_name": "Business Reviewer"}, 0, 201)
	ownerClient.want("POST", base+"/spaces/"+spaceIDs[0]+"/role-assignments", registry.Row{"id": registry.NewID(), "group_id": group, "role_id": role}, 0, 201)
	inherited := out.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"].([]any)
	if len(inherited) != 2 {
		t.Fatal("team business role not inherited", inherited)
	}
	list = out.want("GET", base+"/apps", nil, 0, 200)["items"].([]any)
	if len(list) != 1 {
		t.Fatal("business role granted management access")
	}
	ownerClient.want("PATCH", base+"/teams/"+group, registry.Row{"state": "disabled"}, 2, 200)
	inherited = out.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"].([]any)
	if len(inherited) != 1 {
		t.Fatal("disabled team retained business role")
	}
	ownerClient.want("POST", rolePath, registry.Row{"id": registry.NewID(), "code": "bad-escalation", "display_name": "Invalid", "permissions": []string{"platform.admin"}}, 0, 400)
	t.Log("scope filtering, stale revision, role isolation and group revocation passed")
	// Existing registration validates grants and does not modify business tables.
	var state struct {
		Legacy string `json:"legacy_password"`
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(configPath), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &state); e != nil {
		t.Fatal(e)
	}
	adminConfig, e := LoadConfig(filepath.Join(filepath.Dir(configPath), "migrate.json"))
	if e != nil {
		t.Fatal(e)
	}
	adminServer, e := Open(adminConfig)
	if e != nil {
		t.Fatal(e)
	}
	defer adminServer.DB.Close()
	fixtureDB := "reg_" + strings.ReplaceAll(registry.NewID(), "-", "")
	if _, e = adminServer.DB.Exec("CREATE DATABASE `" + fixtureDB + "`"); e != nil {
		t.Fatal(e)
	}
	if _, e = adminServer.DB.Exec("CREATE TABLE `" + fixtureDB + "`.contacts (id INT PRIMARY KEY,name VARCHAR(100))"); e != nil {
		t.Fatal(e)
	}
	if _, e = adminServer.DB.Exec("INSERT INTO `" + fixtureDB + "`.contacts VALUES (1,'Existing customer')"); e != nil {
		t.Fatal(e)
	}
	if _, e = adminServer.DB.Exec("CREATE USER '" + fixtureDB + "'@'" + cfg.RuntimeAccountHost + "' IDENTIFIED BY '" + state.Legacy + "'"); e != nil {
		t.Fatal(e)
	}
	if _, e = adminServer.DB.Exec("GRANT SELECT,INSERT,UPDATE,DELETE ON `" + strings.ReplaceAll(fixtureDB, "_", `\_`) + "`.* TO '" + fixtureDB + "'@'" + cfg.RuntimeAccountHost + "'"); e != nil {
		t.Fatal(e)
	}
	registered := registry.NewID()
	ownerClient.want("POST", base+"/database-resources", registry.Row{"id": registered, "target_id": target, "management_mode": "registered", "layout": "dedicated", "database_name": fixtureDB, "credentials": registry.Row{"username": fixtureDB, "password": state.Legacy}}, 0, 202)
	c.waitDB(tenant, registered, "ready")
	audit := c.want("GET", base+"/audit-events", nil, 0, 200)
	encoded, _ := json.Marshal(audit)
	if bytes.Contains(encoded, []byte(state.Legacy)) || bytes.Contains(encoded, []byte("credentials")) {
		t.Fatal("audit leaked credential payload")
	}
	var runtime databases.Credentials
	var enc []byte
	if e := s.DB.QueryRow("SELECT runtime_credentials_enc FROM database_resources WHERE id=?", registered).Scan(&enc); e != nil {
		t.Fatal(e)
	}
	if e := s.sealer.Unseal("database", registered, tenant, enc, &runtime); e != nil {
		t.Fatal(e)
	}
	targetRow, _ := databases.TargetEndpoint(ctx, s.DB, target)
	db, e := s.targetDB(ctx, targetRow, runtime, fixtureDB)
	if e != nil {
		t.Fatal(e)
	}
	var contact string
	e = db.QueryRow("SELECT name FROM contacts WHERE id=1").Scan(&contact)
	db.Close()
	if e != nil || contact != "Existing customer" {
		t.Fatal("registered database was changed", e)
	}
	t.Log("existing registration verified without modifying business data")
	// A registered connection with bad credentials can be repaired and retried.
	ownerClient.want("PATCH", base+"/database-resources/"+registered, registry.Row{"credentials": registry.Row{"username": fixtureDB, "password": "incorrect"}}, registry.Integer(c.waitDB(tenant, registered, "ready")["revision"]), 202)
	failed := c.waitDB(tenant, registered, "failed")
	if registry.Str(failed["last_error_code"]) != "authentication_failed" {
		t.Fatalf("unexpected credential failure: %v", failed)
	}
	ownerClient.want("PATCH", base+"/database-resources/"+registered, registry.Row{"credentials": registry.Row{"username": fixtureDB, "password": state.Legacy}}, registry.Integer(failed["revision"]), 202)
	c.waitDB(tenant, registered, "ready")
	// Unknown database ownership is never adopted, even if its name is generated.
	collision := registry.NewID()
	collisionName := "la_" + strings.ReplaceAll(collision, "-", "")
	if _, e = adminServer.DB.Exec("CREATE DATABASE `" + collisionName + "` COMMENT='owned-by-someone-else'"); e != nil {
		t.Fatal(e)
	}
	ownerClient.want("POST", base+"/database-resources", registry.Row{"id": collision, "target_id": target, "management_mode": "managed", "layout": "dedicated"}, 0, 202)
	failed = c.waitDB(tenant, collision, "failed")
	if registry.Str(failed["last_error_code"]) != "ownership_conflict" {
		t.Fatalf("collision adopted: %v", failed)
	}
	var comment string
	if e := adminServer.DB.QueryRow("SELECT SCHEMA_COMMENT FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", collisionName).Scan(&comment); e != nil {
		t.Fatal(e)
	}
	if comment != "owned-by-someone-else" {
		t.Fatal("unknown database changed")
	}
	accountCollision := registry.NewID()
	accountName := "la_" + strings.ReplaceAll(accountCollision, "-", "")
	if _, e = adminServer.DB.Exec("CREATE USER '" + accountName + "'@'" + cfg.RuntimeAccountHost + "' IDENTIFIED BY 'unrelated-account-password'"); e != nil {
		t.Fatal(e)
	}
	ownerClient.want("POST", base+"/database-resources", registry.Row{"id": accountCollision, "target_id": target, "management_mode": "managed", "layout": "dedicated"}, 0, 202)
	failed = c.waitDB(tenant, accountCollision, "failed")
	if registry.Str(failed["last_error_code"]) != "ownership_conflict" {
		t.Fatalf("account collision adopted: %v", failed)
	}
	unrelated, e := s.targetDB(ctx, targetRow, databases.Credentials{Username: accountName, Password: "unrelated-account-password"}, "")
	if e != nil {
		t.Fatal("unrelated account password was changed")
	}
	unrelated.Close()
	// Kill the leader connection during target work: the replacement resumes safely.
	recovery := registry.NewID()
	held, e := adminServer.DB.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	var locked int
	if e = held.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", "lastapp:resource:"+recovery).Scan(&locked); e != nil || locked != 1 {
		t.Fatal("cannot hold recovery lock", e)
	}
	ownerClient.want("POST", base+"/database-resources", registry.Row{"id": recovery, "target_id": target, "management_mode": "managed", "layout": "dedicated"}, 0, 202)
	c.waitDB(tenant, recovery, "provisioning")
	var leader uint64
	if e = adminServer.DB.QueryRow("SELECT IS_USED_LOCK(?)", s.worker.Lock).Scan(&leader); e != nil {
		t.Fatal(e)
	}
	if _, e = adminServer.DB.Exec("KILL CONNECTION " + registry.Str(leader)); e != nil {
		t.Fatal(e)
	}
	if _, e = held.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", "lastapp:resource:"+recovery); e != nil {
		t.Fatal(e)
	}
	c.waitDB(tenant, recovery, "ready")
	t.Log("credential repair, collision refusal and leadership-loss recovery passed")

	// Plans are enforced for Tenant administrators.
	starter := registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": starter, "slug": "starter-" + starter, "display_name": "Quota test", "plan_id": "10000000-0000-4000-8000-000000000002", "initial_admin_user_id": owner}, 0, 201)
	sb := "/v1/tenants/" + starter
	ownerClient.want("POST", sb+"/database-resources", registry.Row{"id": registry.NewID(), "target_id": target, "management_mode": "managed", "layout": "dedicated"}, 0, 403)
	ownerClient.want("POST", sb+"/database-resources", registry.Row{"id": registry.NewID(), "target_id": target, "management_mode": "managed", "layout": "shared"}, 0, 202)
	ownerClient.want("POST", sb+"/database-resources", registry.Row{"id": registry.NewID(), "target_id": target, "management_mode": "managed", "layout": "shared"}, 0, 409)
	// Cross-Tenant references are rejected by the relational constraints.
	ownerClient.want("POST", sb+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": registry.NewID(), "slug": "cross-tenant", "display_name": "Cross tenant", "database": registry.Row{"resource_id": shared}}, 0, 404)
	out.want("GET", sb+"/apps", nil, 0, 404)
	// Suspended Tenant is readable but cannot allocate new resources.
	tenantRow := c.want("GET", base, nil, 0, 200)
	c.want("PATCH", "/admin/v1/tenants/"+tenant, registry.Row{"state": "suspended"}, registry.Integer(tenantRow["revision"]), 200)
	ownerClient.want("POST", base+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": registry.NewID(), "slug": "suspended", "display_name": "Suspended", "database": registry.Row{"mode": "managed", "layout": "shared", "target_id": target}}, 0, 403)
	ownerClient.want("POST", base+"/database-resources", registry.Row{"id": registry.NewID(), "target_id": target, "management_mode": "managed", "layout": "shared"}, 0, 403)
	t.Log("plan quotas, cross-Tenant isolation and suspended lifecycle passed")
	// Encryption AAD is checked independently of SQL ownership constraints.
	if s.sealer.Unseal("database", registered, starter, enc, &runtime) == nil {
		t.Fatal("credential moved across Tenants")
	}
	if strings.Contains(registry.Str(c.want("GET", base+"/database-resources", nil, 0, 200)), state.Legacy) {
		t.Fatal("API leaked credentials")
	}
}
