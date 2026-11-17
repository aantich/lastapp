package control

import (
	"net/url"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/registry"
)

func TestTenantAdministratorTask(t *testing.T) {
	s, platform, operator, owner, joe := foundationFixture(t)
	tenant := registry.NewID()
	base := "/v1/tenants/" + tenant
	task := "/admin/v1/tenants/" + tenant + "/administrators"
	platform.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "simple-" + tenant, "display_name": "Simple administration", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	email, password := "admin-"+registry.NewID()+"@example.test", "A temporary administrator password!"
	input := registry.Row{"account": registry.Row{"display_name": "New administrator", "email": email, "password": password}}
	created := platform.want("POST", task, input, 0, 201)
	user := registry.Str(created["user_id"])
	match := platform.want("GET", task+"?q="+url.QueryEscape(email), nil, 0, 200)["items"].([]any)
	if len(match) != 1 || registry.Str(match[0].(map[string]any)["user_id"]) != user {
		t.Fatal("administrator email search missed the account", match)
	}
	member := platform.want("GET", base+"/memberships?q="+user, nil, 0, 200)["items"].([]any)[0].(map[string]any)
	if registry.Str(member["administrator_id"]) == "" || member["state"] != "active" {
		t.Fatal("incomplete administrator", member)
	}
	passwordSignIn(platform, email, password, 200)
	platform.want("POST", task, registry.Row{"account": registry.Row{"email": email}}, 0, 201)
	platform.want("POST", task, input, 0, 409)
	var count int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM access_grants WHERE tenant_id=? AND user_id=? AND role_id=?", tenant, user, access.TenantAdminRole).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate or missing administrator", e)
	}
	platform.as(owner).want("POST", task, input, 0, 403)
	platform.as(owner).want("GET", task, nil, 0, 403)
	platform.want("POST", task, registry.Row{"user_id": operator}, 0, 400)
	platform.want("POST", task, registry.Row{"user_id": joe}, 0, 201)
	platform.want("PATCH", base+"/memberships/"+joe, registry.Row{"state": "disabled"}, 1, 200)
	platform.want("POST", task, registry.Row{"user_id": joe}, 0, 201)
	restored := platform.want("GET", base+"/memberships?q="+joe, nil, 0, 200)["items"].([]any)[0].(map[string]any)
	if restored["state"] != "active" || registry.Integer(restored["revision"]) != 3 {
		t.Fatal("administrator recovery did not restore membership", restored)
	}
	// Failure after account creation must roll back credentials and membership.
	if _, e := s.DB.Exec("CREATE TRIGGER reject_review_member BEFORE INSERT ON audit_events FOR EACH ROW BEGIN IF NEW.action='membership.created' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='review rollback'; END IF; END"); e != nil {
		t.Fatal(e)
	}
	rollbackEmail := "rollback-" + registry.NewID() + "@example.test"
	platform.want("POST", task, registry.Row{"account": registry.Row{"display_name": "Rollback", "email": rollbackEmail, "password": password}}, 0, 500)
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email=?", rollbackEmail).Scan(&count); e != nil || count != 0 {
		t.Fatal("task left a partial account", e)
	}
	if _, e := s.DB.Exec("DROP TRIGGER reject_review_member"); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE CAST(details_json AS CHAR) LIKE ?", "%"+password+"%").Scan(&count); e != nil || count != 0 {
		t.Fatal("credential in audit", e)
	}
	admins := platform.want("GET", task, nil, 0, 200)
	if strings.Contains(registry.Str(admins), password) {
		t.Fatal("credential in administrator list")
	}
}

func TestScopedRecipientOptions(t *testing.T) {
	s, platform, operator, owner, joe := foundationFixture(t)
	tenant := registry.NewID()
	base := "/v1/tenants/" + tenant
	platform.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "recipients-" + tenant, "display_name": "Recipient options", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	ownerClient := platform.as(owner)
	ownerClient.want("POST", base+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	ownerClient.want("POST", base+"/memberships", registry.Row{"user_id": operator}, 0, 201)
	target, db, space, app, sibling := registry.NewID(), registry.NewID(), registry.NewID(), registry.NewID(), registry.NewID()
	if _, e := s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode) VALUES (?,'Recipients','127.0.0.1',33317,'disabled')", target); e != nil {
		t.Fatal(e)
	}
	enc, e := s.sealer.Seal("database", db, tenant, databases.Credentials{Username: "fixture", Password: registry.Token(32)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES (?,?,?,?,'registered','shared','ready',?)", db, tenant, target, "fixture_"+db, enc); e != nil {
		t.Fatal(e)
	}
	ownerClient.want("POST", base+"/spaces", registry.Row{"id": space, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": db}}, 0, 201)
	for _, id := range []string{app, sibling} {
		ownerClient.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": id, "slug": "app-" + id, "display_name": "App", "initial_manager_user_id": owner}, 0, 201)
	}
	ownerClient.want("POST", base+"/apps/"+app+"/role-assignments", registry.Row{"id": registry.NewID(), "user_id": joe, "role_id": "00000000-0000-4000-8000-000000000006"}, 0, 201)
	manager := platform.as(joe)
	manager.want("GET", base+"/memberships", nil, 0, 403)
	options := manager.want("GET", base+"/apps/"+app+"/recipients", nil, 0, 200)["items"].([]any)
	if len(options) != 2 {
		t.Fatal("wrong eligible recipient count", options)
	}
	var email string
	if e = s.DB.QueryRow("SELECT email FROM users WHERE id=?", joe).Scan(&email); e != nil {
		t.Fatal(e)
	}
	match := manager.want("GET", base+"/apps/"+app+"/recipients?q="+url.QueryEscape(email), nil, 0, 200)["items"].([]any)
	if len(match) != 1 || registry.Str(match[0].(map[string]any)["id"]) != joe {
		t.Fatal("recipient email search missed the account", match)
	}
	team := registry.NewID()
	ownerClient.want("POST", base+"/teams", registry.Row{"id": team, "slug": "search-team", "display_name": "Search team"}, 0, 201)
	teams := manager.want("GET", base+"/apps/"+app+"/recipients?type=team&q=Search", nil, 0, 200)["items"].([]any)
	if len(teams) != 1 || registry.Str(teams[0].(map[string]any)["id"]) != team {
		t.Fatal("team recipient search returned incorrect rows", teams)
	}
	if len(manager.want("GET", base+"/apps/"+app+"/recipients?type=team&q="+url.QueryEscape(email), nil, 0, 200)["items"].([]any)) != 0 {
		t.Fatal("email search returned a team")
	}
	manager.want("GET", base+"/apps/"+sibling+"/recipients", nil, 0, 403)
	manager.want("GET", base+"/spaces/"+space+"/recipients", nil, 0, 403)
}

func TestRecommendedSpaceDatabase(t *testing.T) {
	s, platform, _, owner, _ := foundationFixture(t)
	tenant := registry.NewID()
	platform.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "recommended-" + tenant, "display_name": "Recommended database", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	client := platform.as(owner)
	base := "/v1/tenants/" + tenant
	create := func(want int) registry.Row {
		created := client.want("POST", base+"/spaces", registry.Row{"id": registry.NewID(), "slug": "space-" + registry.NewID(), "display_name": "Space", "initial_manager_user_id": owner, "database": registry.Row{"mode": "managed"}}, 0, want)
		if want == 201 {
			return client.want("GET", base+"/spaces/"+registry.Str(created["id"]), nil, 0, 200)
		}
		return created
	}
	create(409) // A missing managed target gives actionable setup feedback, without allocation.
	target := registry.NewID()
	platform.want("POST", "/admin/v1/database-targets", registry.Row{"id": target, "display_name": "Recommended", "canonical_host": "127.0.0.1", "port": 33317, "tls_mode": "disabled", "managed_enabled": true, "credentials": registry.Row{"username": "fixture", "password": "Never used: no worker in this test"}}, 0, 201)
	first := create(201)
	second := create(201)
	if first["database_resource_id"] != second["database_resource_id"] {
		t.Fatal("default shared database was not reused", first, second)
	}
	var selected string
	if e := s.DB.QueryRow("SELECT target_id FROM database_resources WHERE id=?", first["database_resource_id"]).Scan(&selected); e != nil || selected != target {
		t.Fatal("wrong recommended target", selected, e)
	}
	if _, e := s.DB.Exec("UPDATE plans SET allow_shared_database=0,default_layout='dedicated' WHERE id='10000000-0000-4000-8000-000000000001'"); e != nil {
		t.Fatal(e)
	}
	profile := client.want("GET", base+"/overview", nil, 0, 200)["tenant"].(map[string]any)
	if profile["default_layout"] != "dedicated" {
		t.Fatal("invalid layout recommended after plan change", profile)
	}
	dedicated := create(201)
	if dedicated["database_layout"] != "dedicated" || dedicated["database_resource_id"] == first["database_resource_id"] {
		t.Fatal("plan default not respected", dedicated)
	}
	options := client.want("GET", base+"/database-targets?managed=true", nil, 0, 200)["items"].([]any)
	if len(options) != 1 || registry.Str(options[0].(map[string]any)["id"]) != target {
		t.Fatal("managed picker included ineligible targets", options)
	}
}
