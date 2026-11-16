package control

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aantich/lastapp/internal/authz"
)

// The fixture owns only its new registry. Business resources below are metadata
// for authorization tests; provisioning correctness has its separate live suite.
func foundationFixture(t *testing.T) (*Server, testClient, string, string, string) {
	t.Helper()
	file := os.Getenv("LASTAPP_TEST_CONFIG")
	if file == "" {
		t.Skip("set LASTAPP_TEST_CONFIG for the installed local MariaDB")
	}
	cfg, e := LoadConfig(file)
	if e != nil {
		t.Fatal(e)
	}
	if !cfg.Development {
		t.Fatal("local development configuration required")
	}
	adminCfg, e := LoadConfig(filepath.Join(filepath.Dir(file), "migrate.json"))
	if e != nil {
		t.Fatal(e)
	}
	admin, e := Open(adminCfg)
	if e != nil {
		t.Fatal(e)
	}
	name := "lastapp_foundation_test_" + strings.ReplaceAll(newID(), "-", "")
	if _, e = admin.DB.Exec("CREATE DATABASE `" + name + "`"); e != nil {
		t.Fatal(e)
	}
	cfg.RegistryDSN = adminCfg.RegistryDSN[:strings.LastIndex(adminCfg.RegistryDSN, "/")+1] + name
	s, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		s.DB.Close()
		if _, e := admin.DB.Exec("DROP DATABASE `" + name + "`"); e != nil {
			t.Error(e)
		}
		admin.DB.Close()
	})
	if e = Migrate(context.Background(), s.DB); e != nil {
		t.Fatal(e)
	}
	if e = s.SeedDevelopment(context.Background(), ""); e != nil {
		t.Fatal(e)
	}
	var operator, owner, joe string
	for subject, dest := range map[string]*string{"operator": &operator, "tenant-admin": &owner, "outsider": &joe} {
		if e = s.DB.QueryRow("SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject=?", cfg.OIDCIssuer, subject).Scan(dest); e != nil {
			t.Fatal(e)
		}
	}
	return s, (testClient{t: t, s: s, h: s.Handler()}).as(operator), operator, owner, joe
}

func hasAssignment(items any, role string) bool {
	for _, v := range items.([]any) {
		if str(v.(map[string]any)["role_id"]) == role {
			return true
		}
	}
	return false
}

func TestAuthorizationFoundation(t *testing.T) {
	s, c, operator, owner, joe := foundationFixture(t)
	ownerClient, joeClient := c.as(owner), c.as(joe)
	tenant, other := newID(), newID()
	for _, id := range []string{tenant, other} {
		c.want("POST", "/admin/v1/tenants", Row{"id": id, "slug": "foundation-" + id, "display_name": "Foundation " + id, "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	}
	base := "/v1/tenants/" + tenant
	ownerClient.want("POST", base+"/memberships", Row{"user_id": joe}, 0, 201)
	ownerClient.want("POST", base+"/memberships", Row{"user_id": operator}, 0, 201)
	target, db := newID(), newID()
	if _, e := s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode) VALUES (?,'Fixture','127.0.0.1',33317,'disabled')", target); e != nil {
		t.Fatal(e)
	}
	enc, e := s.seal("database", db, tenant, credentials{Username: "fixture", Password: token(32)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES (?,?,?,?,'registered','shared','ready',?)", db, tenant, target, "fixture_"+strings.ReplaceAll(db, "-", ""), enc); e != nil {
		t.Fatal(e)
	}
	spaces := []string{newID(), newID()}
	apps := []string{newID(), newID()}
	for i, id := range spaces {
		ownerClient.want("POST", base+"/spaces", Row{"id": id, "slug": "space-" + str(i), "display_name": "Space " + str(i), "initial_manager_user_id": owner, "database": Row{"resource_id": db}}, 0, 201)
	}
	for i, id := range apps {
		ownerClient.want("POST", base+"/spaces/"+spaces[0]+"/apps", Row{"id": id, "slug": "app-" + str(i), "display_name": "App " + str(i), "initial_manager_user_id": owner}, 0, 201)
	}
	marketing, engineering := newID(), newID()
	for i, id := range []string{marketing, engineering} {
		ownerClient.want("POST", base+"/teams", Row{"id": id, "slug": "team-" + str(i), "display_name": []string{"Marketing", "Engineering"}[i]}, 0, 201)
	}
	teamPath := base + "/teams/" + marketing
	ownerClient.want("POST", teamPath+"/members", Row{"user_id": joe}, 1, 200)
	ownerClient.want("POST", teamPath+"/members", Row{"user_id": owner}, 2, 200)
	ownerClient.want("POST", teamPath+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": authz.TeamReviewerRole}, 0, 201)
	roleContext := joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)
	if !hasAssignment(roleContext["items"], authz.TeamReviewerRole) {
		t.Fatal("true team Reviewer missing", roleContext)
	}
	for _, item := range roleContext["items"].([]any) {
		g := item.(map[string]any)
		if str(g["role_id"]) == authz.TeamReviewerRole && str(g["scope_team_id"]) != marketing {
			t.Fatal("incorrect team scope")
		}
	}
	if len(joeClient.want("GET", base+"/teams", nil, 0, 200)["items"].([]any)) != 1 {
		t.Fatal("team metadata leaked sibling")
	}
	joeClient.want("GET", base+"/teams/"+engineering, nil, 0, 403)
	joeClient.want("GET", teamPath+"/members", nil, 0, 403)
	ownerClient.want("POST", base+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": authz.TeamReviewerRole}, 0, 404)
	ownerClient.want("POST", teamPath+"/role-assignments", Row{"id": newID(), "group_id": marketing, "role_id": authz.TeamAdminRole}, 0, 400)
	ownerClient.want("POST", base+"/role-assignments", Row{"id": newID(), "user_id": joe, "permission_code": "app.manage"}, 0, 400)
	// Disabling preserves the assignment; leaving deletes it before its FK parent.
	ownerClient.want("PATCH", teamPath+"/members/"+joe, Row{"state": "disabled"}, 3, 200)
	if hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], authz.TeamReviewerRole) {
		t.Fatal("disabled team member retained role")
	}
	ownerClient.want("PATCH", teamPath+"/members/"+joe, Row{"state": "active"}, 4, 200)
	if !hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], authz.TeamReviewerRole) {
		t.Fatal("membership enable lost preserved assignment")
	}
	ownerClient.want("DELETE", teamPath+"/members/"+joe, nil, 5, 200)
	ownerClient.want("POST", teamPath+"/members", Row{"user_id": joe}, 6, 200)
	if hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], authz.TeamReviewerRole) {
		t.Fatal("rejoin restored deleted role")
	}
	// Team administrators can govern their team, never its Tenant or other scopes.
	ownerClient.want("POST", teamPath+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": authz.TeamAdminRole}, 0, 201)
	joeClient.want("POST", teamPath+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": authz.TeamReviewerRole}, 0, 201)
	joeClient.want("POST", base+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": TenantAdminRole}, 0, 403)
	joeClient.want("GET", base+"/memberships", nil, 0, 403)
	// Custom business roles have stable scope ownership and no administrative powers.
	scope := base + "/spaces/" + spaces[0]
	role := newID()
	ownerClient.want("POST", scope+"/roles", Row{"id": role, "code": "tenant-admin", "display_name": "Tenant administrator"}, 0, 201)
	ownerClient.want("POST", scope+"/role-assignments", Row{"id": newID(), "group_id": marketing, "role_id": role}, 0, 201)
	if !hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], role) {
		t.Fatal("team recipient did not confer business role")
	}
	caps := joeClient.want("GET", base+"/capabilities", nil, 0, 200)["permissions"].(map[string]any)
	if caps["app.manage"].(map[string]any)["available"] != false {
		t.Fatal("business role widened control authority")
	}
	ownerClient.want("POST", base+"/spaces/"+spaces[1]+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": role}, 0, 404)
	ownerClient.want("POST", "/v1/tenants/"+other+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": TenantAdminRole}, 0, 400)
	ownerClient.want("POST", scope+"/roles", Row{"id": newID(), "code": "escalate", "display_name": "Denied", "permissions": []string{"platform.admin"}}, 0, 400)
	ownerClient.want("PATCH", scope+"/roles/"+authz.SpaceManagerRole, Row{"state": "disabled"}, 1, 403)
	// Membership changes immediately change team-recipient business roles.
	ownerClient.want("PATCH", teamPath, Row{"state": "disabled"}, 7, 200)
	if hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], role) {
		t.Fatal("disabled team retained business role")
	}
	ownerClient.want("PATCH", teamPath, Row{"state": "active"}, 8, 200)
	ownerClient.want("PATCH", scope+"/roles/"+role, Row{"state": "disabled"}, 1, 200)
	if hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], role) {
		t.Fatal("disabled role retained access")
	}
	ownerClient.want("PATCH", scope+"/roles/"+role, Row{"state": "active"}, 2, 200)
	// App manager permissions cannot reach siblings or appoint another manager.
	appPath := base + "/apps/" + apps[0]
	ownerClient.want("POST", appPath+"/role-assignments", Row{"id": newID(), "user_id": joe, "role_id": authz.AppManagerRole}, 0, 201)
	list := joeClient.want("GET", base+"/apps", nil, 0, 200)["items"].([]any)
	if len(list) != 1 || str(list[0].(map[string]any)["id"]) != apps[0] {
		t.Fatal("App management leaked sibling", list)
	}
	joeClient.want("POST", appPath+"/role-assignments", Row{"id": newID(), "user_id": operator, "role_id": authz.AppManagerRole}, 0, 403)
	appRole := newID()
	joeClient.want("POST", appPath+"/roles", Row{"id": appRole, "code": "analyst", "display_name": "Analyst"}, 0, 201)
	joeClient.want("POST", appPath+"/role-assignments", Row{"id": newID(), "user_id": owner, "role_id": appRole}, 0, 201)
	joeClient.want("PATCH", base+"/apps/"+apps[1], Row{"display_name": "Denied"}, 1, 403)
	// Data/control join separation and nullable scope checks also hold in SQL.
	var permission string
	if e := s.DB.QueryRow("SELECT id FROM permissions WHERE code='app.manage'").Scan(&permission); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO role_permissions(role_id,permission_id,domain) VALUES (?,?,'control')", role, permission); e == nil {
		t.Fatal("data role acquired control permission through SQL")
	}
	for _, statement := range []string{
		"INSERT INTO access_grants(id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'tenant','control',?,?,'builtin')",
		"INSERT INTO access_grants(id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'team','control',?,?,'builtin')",
	} {
		if _, e = s.DB.Exec(statement, newID(), joe, authz.TeamReviewerRole); e == nil {
			t.Fatal("nullable scope bypass accepted")
		}
	}
	if e = s.authorize(context.Background(), s.DB, Principal{ID: operator, Platform: true}, "deals.read", Resource{}, false); e == nil {
		t.Fatal("unknown business action used platform bypass")
	}
	ownerClient.want("PATCH", base+"/memberships/"+joe, Row{"state": "disabled"}, 1, 200)
	joeClient.want("GET", base+"/my-role-assignments", nil, 0, 404)
	ownerClient.want("PATCH", base+"/memberships/"+joe, Row{"state": "active"}, 1, 412)
	ownerClient.want("PATCH", base+"/memberships/"+joe, Row{"state": "active"}, 2, 200)
	if !hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], role) {
		t.Fatal("Tenant membership re-enable lost assignments")
	}
	user := c.want("GET", "/admin/v1/users?q="+joe, nil, 0, 200)["items"].([]any)[0].(map[string]any)
	c.want("PATCH", "/admin/v1/users/"+joe, Row{"state": "disabled"}, integer(user["revision"]), 200)
	joeClient.want("GET", base+"/my-role-assignments", nil, 0, 401)
	c.want("PATCH", "/admin/v1/users/"+joe, Row{"state": "active"}, integer(user["revision"])+1, 200)
	joeClient = c.as(joe)
	if !hasAssignment(joeClient.want("GET", base+"/my-role-assignments", nil, 0, 200)["items"], role) {
		t.Fatal("user re-enable lost assignments")
	}
	page := joeClient.want("GET", base+"/my-role-assignments?limit=1", nil, 0, 200)
	if len(page["items"].([]any)) != 1 || page["pagination"].(map[string]any)["has_more"] != true {
		t.Fatal("caller role context is not bounded", page)
	}
	// Two concurrent self-removals cannot remove every Tenant administrator.
	assignments := ownerClient.want("GET", base+"/role-assignments", nil, 0, 200)["items"].([]any)
	var ownerGrant string
	for _, v := range assignments {
		g := v.(map[string]any)
		if str(g["user_id"]) == owner {
			ownerGrant = str(g["id"])
		}
	}
	joeGrant := newID()
	ownerClient.want("POST", base+"/role-assignments", Row{"id": joeGrant, "user_id": joe, "role_id": TenantAdminRole}, 0, 201)
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i, g := range []struct {
		client testClient
		grant  string
	}{{ownerClient, ownerGrant}, {joeClient, joeGrant}} {
		wg.Add(1)
		go func(i int, client testClient, grant string) {
			defer wg.Done()
			statuses[i], _ = client.req("DELETE", base+"/role-assignments/"+grant, nil, 1)
		}(i, g.client, g.grant)
	}
	wg.Wait()
	if !((statuses[0] == 200 && statuses[1] == 409) || (statuses[1] == 200 && statuses[0] == 409)) {
		t.Fatal("last-admin race", statuses)
	}
	t.Log("team scoping, revocation, protected roles, business/control separation, App isolation, SQL constraints and last-admin race passed")
}
