package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/artifact"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/delivery"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/publisher"
	"github.com/aantich/lastapp/internal/registry"
)

func TestDevelopmentWorkspaceReleaseFlow(t *testing.T) {
	for _, failure := range []string{"", "missing_build", "activation_failure"} {
		name := failure
		if name == "" {
			name = "activation"
		}
		t.Run(name, func(t *testing.T) { testDevelopmentWorkspaceReleaseFlow(t, failure) })
	}
}

func testDevelopmentWorkspaceReleaseFlow(t *testing.T, failure string) {
	s, c, _, owner, joe := foundationFixture(t)
	s.Config.ArtifactDirectory = t.TempDir()
	ctx := context.Background()
	oc := c.as(owner)
	admin, e := LoadConfig(filepath.Join(filepath.Dir(os.Getenv("LASTAPP_TEST_CONFIG")), "migrate.json"))
	if e != nil {
		t.Fatal(e)
	}
	dsn, e := mariadb.ParseDSN(admin.RegistryDSN)
	if e != nil {
		t.Fatal(e)
	}
	tenant, space, app, target := registry.NewID(), registry.NewID(), registry.NewID(), registry.NewID()
	base := "/v1/tenants/" + tenant
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "work-" + tenant, "display_name": "Workspace tests", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	oc.want("POST", base+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	enc, e := s.sealer.Seal("target", target, "", databases.Credentials{Username: dsn.User, Password: dsn.Passwd})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_targets(id,display_name,canonical_host,port,tls_mode,managed_enabled,provisioning_credentials_enc) VALUES(?,'Workspace test','127.0.0.1',33317,'disabled',1,?)", target, enc); e != nil {
		t.Fatal(e)
	}
	cleanup := func() {
		s.publisher.Close()
		rs, _ := registry.QueryRows(ctx, s.DB, "SELECT id,database_name,runtime_credentials_enc FROM database_resources WHERE tenant_id=? AND management_mode='managed'", tenant)
		for _, v := range rs {
			var cr databases.Credentials
			_ = s.sealer.Unseal("database", registry.Str(v["id"]), tenant, []byte(registry.Str(v["runtime_credentials_enc"])), &cr)
			if generatedRE.MatchString(registry.Str(v["database_name"])) {
				_, _ = s.DB.Exec("DROP DATABASE `" + registry.Str(v["database_name"]) + "`")
			}
			if generatedRE.MatchString(cr.Username) {
				_, _ = s.DB.Exec("DROP USER IF EXISTS '" + cr.Username + "'@'" + cr.AccountHost + "'")
			}
		}
	}
	t.Cleanup(cleanup)
	oc.want("POST", base+"/spaces", registry.Row{"id": space, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner, "database": registry.Row{"mode": "managed", "target_id": target}}, 0, 201)
	var source string
	if e = s.DB.QueryRow("SELECT database_resource_id FROM spaces WHERE id=?", space).Scan(&source); e != nil {
		t.Fatal(e)
	}
	provision := func(resource string) {
		t.Helper()
		if e := s.provision(ctx, resource); e != nil {
			t.Fatal(e)
		}
		if _, e := s.DB.Exec("UPDATE database_resources SET state='ready' WHERE id=?", resource); e != nil {
			t.Fatal(e)
		}
	}
	provision(source)
	oc.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": app, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner}, 0, 201)
	definition := entity.Templates()[0]
	definition.Permissions = []entity.Permission{{Key: "read", Label: "Read", Operation: "read", Relationship: "member", Fields: []string{"name", "website"}}, {Key: "create", Label: "Create", Operation: "create", Relationship: "member", Fields: []string{"name", "website"}}}
	roleID, inactiveUser, inactiveTeam := registry.NewID(), registry.NewID(), registry.NewID()
	oc.want("POST", base+"/spaces/"+space+"/roles", registry.Row{"id": roleID, "code": "sensitive-reader", "display_name": "Sensitive reader"}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/role-assignments", registry.Row{"id": registry.NewID(), "user_id": joe, "role_id": roleID}, 0, 201)
	if _, e = s.DB.Exec("INSERT INTO users(id,oidc_issuer,oidc_subject,display_name) VALUES(?, 'test:inactive',?,'Former member')", inactiveUser, inactiveUser); e != nil {
		t.Fatal(e)
	}
	oc.want("POST", base+"/memberships", registry.Row{"user_id": inactiveUser}, 0, 201)
	oc.want("POST", base+"/teams", registry.Row{"id": inactiveTeam, "slug": "former-team", "display_name": "Former team"}, 0, 201)
	definition.Permissions[0].Relationship = "user"
	definition.Permissions[0].Subject = owner
	definition.Permissions = append(definition.Permissions,
		entity.Permission{Key: "sensitive", Label: "Sensitive", Operation: "read", Roles: []string{roleID}, Fields: []string{"name"}},
		entity.Permission{Key: "former_user", Label: "Former user", Operation: "read", Relationship: "user", Subject: inactiveUser, Fields: []string{"name"}},
		entity.Permission{Key: "former_team", Label: "Former team", Operation: "read", Relationship: "team", Subject: inactiveTeam, Fields: []string{"name"}},
	)
	made := oc.want("POST", "/v1/spaces/"+space+"/entities", registry.Row{"definition": definition}, 0, 200)
	eid := registry.Str(made["id"])
	oc.want("POST", "/v1/entities/"+eid+"/publications", registry.Row{}, 1, 202)
	if _, e = s.publisher.Job(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	// Baseline copying must also preserve a cyclic entity-reference graph.
	contact := entity.Templates()[1]
	contact.Fields = append(contact.Fields, entity.Field{Key: "company", Label: "Company", Type: "reference", Target: eid})
	contactRow := oc.want("POST", "/v1/spaces/"+space+"/entities", registry.Row{"definition": contact}, 0, 200)
	contactID := registry.Str(contactRow["id"])
	oc.want("POST", "/v1/entities/"+contactID+"/publications", registry.Row{}, 1, 202)
	if _, e = s.publisher.Job(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	definition.Fields = append(definition.Fields, entity.Field{Key: "contact", Label: "Contact", Type: "reference", Target: contactID})
	companyRow := oc.want("GET", "/v1/entities/"+eid, nil, 0, 200)
	companySaved := oc.want("PATCH", "/v1/entities/"+eid, registry.Row{"definition": definition}, registry.Integer(companyRow["revision"]), 200)
	oc.want("POST", "/v1/entities/"+eid+"/publications", registry.Row{}, registry.Integer(companySaved["revision"]), 202)
	if _, e = s.publisher.Job(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	oc.want("PATCH", "/v1/apps/"+app+"/data-access", entity.AppAccess{Origin: "http://localhost:5390", Inherit: true}, 1, 200)
	for _, change := range []string{"UPDATE roles SET state='disabled',revision=revision+1 WHERE id=?", "UPDATE groups SET state='disabled',revision=revision+1 WHERE id=?"} {
		id := roleID
		if strings.Contains(change, "groups") {
			id = inactiveTeam
		}
		if _, e = s.DB.Exec(change, id); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.DB.Exec("UPDATE tenant_memberships SET state='disabled',revision=revision+1 WHERE tenant_id=? AND user_id=?", tenant, inactiveUser); e != nil {
		t.Fatal(e)
	}
	// A new grant to an inactive recipient remains invalid outside a published copy.
	inactiveDefinition := definition
	inactiveDefinition.Permissions = definition.Permissions[3:]
	if entities.ValidateEntityRefs(ctx, s.DB, inactiveDefinition, entities.EntityDefinition{ID: eid, TenantID: tenant, SpaceID: space}) == nil {
		t.Fatal("ordinary publication accepted inactive recipients")
	}
	ws := oc.want("POST", "/v1/spaces/"+space+"/workspaces", delivery.WorkspaceCreateInput{ID: registry.NewID(), Name: "Private author workspace"}, 0, 200)
	wid, backing, resource := registry.Str(ws["id"]), registry.Str(ws["backing_space_id"]), registry.Str(ws["database_resource_id"])
	if resource == source {
		t.Fatal("workspace shares production database")
	}
	c.as(joe).want("GET", "/v1/workspaces/"+wid, nil, 0, 404)
	provision(resource)
	for i := 0; i < 10; i++ {
		if e = s.delivery.WorkspaceJob(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
		if _, e = s.publisher.Job(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
	}
	ws = oc.want("GET", "/v1/workspaces/"+wid, nil, 0, 200)
	if ws["state"] != "ready" {
		t.Fatal(ws)
	}
	var devApp, devEntity string
	for _, m := range ws["mappings"].([]any) {
		v := m.(map[string]any)
		if v["kind"] == "app" {
			devApp = registry.Str(v["development_id"])
		}
		if v["kind"] == "entity" && v["production_id"] == eid {
			devEntity = registry.Str(v["development_id"])
		}
	}
	if devApp == "" || devEntity == eid {
		t.Fatal("missing isolated identity mapping")
	}
	list := oc.want("GET", base+"/spaces", nil, 0, 200)
	if len(list["items"].([]any)) != 1 {
		t.Fatal("private workspace leaked into production inventory", list)
	}
	info := oc.want("GET", base+"/apps?space_id="+backing, nil, 0, 200)
	if len(info["items"].([]any)) != 1 {
		t.Fatal(info)
	}
	oc.want("PATCH", "/v1/apps/"+devApp+"/data-access", entity.AppAccess{Origin: "http://localhost:5391", Inherit: true}, 1, 200)
	requestData := func(origin, token, method, path string, body any) (int, registry.Row) {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/data/v1"+path, bytes.NewReader(b))
		r.Header.Set("Origin", origin)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		var v registry.Row
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return w.Code, v
	}
	launch := func(app, origin string) string {
		verifier, nonce := registry.Token(32), registry.Token(32)
		code := oc.want("POST", "/v1/apps/"+app+"/launch", registry.Row{"challenge": base64.RawURLEncoding.EncodeToString(registry.TokenHash(verifier)), "nonce": nonce}, 0, 200)
		status, v := requestData(origin, "", "POST", "/session/exchange", registry.Row{"code": code["code"], "verifier": verifier, "nonce": nonce})
		if status != 200 {
			t.Fatal(status, v)
		}
		return registry.Str(v["access_token"])
	}
	prodToken, devToken := launch(app, "http://localhost:5390"), launch(devApp, "http://localhost:5391")
	if status, v := requestData("http://localhost:5390", prodToken, "POST", "/entities/company/records", registry.Row{"id": registry.NewID(), "values": registry.Row{"name": "Production only"}}); status != 200 {
		t.Fatal(status, v)
	}
	if status, v := requestData("http://localhost:5391", devToken, "POST", "/entities/company/query", registry.Row{"include_count": true}); status != 200 || registry.Integer(v["count"]) != 0 {
		t.Fatal("production records copied", status, v)
	}
	if status, v := requestData("http://localhost:5391", devToken, "POST", "/entities/company/records", registry.Row{"id": registry.NewID(), "values": registry.Row{"name": "Fixture"}}); status != 200 {
		t.Fatal(status, v)
	}
	if status, v := requestData("http://localhost:5390", prodToken, "POST", "/entities/company/query", registry.Row{"include_count": true}); status != 200 || registry.Integer(v["count"]) != 1 {
		t.Fatal("workspace wrote production", status, v)
	}
	starter := oc.want("GET", "/v1/apps/"+devApp+"/starter", nil, 0, 200)
	if !strings.Contains(registry.Str(starter["files"]), "connectDevelopment") {
		t.Fatal("starter lacks SDK")
	}
	// Publish a compatible development-only backend change, then promote that
	// exact schema/rule/ACL snapshot while preserving production rows.
	currentDefinition := oc.want("GET", "/v1/entities/"+devEntity, nil, 0, 200)
	definition, e = entities.DefOf(currentDefinition["draft_json"])
	if e != nil {
		t.Fatal(e)
	}
	var developmentRole string
	if e = s.DB.QueryRow("SELECT development_id FROM workspace_items WHERE workspace_id=? AND kind='role' AND production_id=?", wid, roleID).Scan(&developmentRole); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("UPDATE roles SET state='active',revision=revision+1 WHERE id=?", developmentRole); e != nil {
		t.Fatal(e)
	}
	// Reactivating this role and removing its old permission must switch atomically.
	definition.Permissions = definition.Permissions[:2]
	definition.Fields = append(definition.Fields, entity.Field{Key: "notes", Label: "Notes", Type: "text", MaxLength: 200})
	for i := range definition.Permissions {
		definition.Permissions[i].Fields = append(definition.Permissions[i].Fields, "notes")
	}

	saved := oc.want("PATCH", "/v1/entities/"+devEntity, registry.Row{"definition": definition}, registry.Integer(currentDefinition["revision"]), 200)
	oc.want("POST", "/v1/entities/"+devEntity+"/publications", registry.Row{}, registry.Integer(saved["revision"]), 202)
	if _, e = s.publisher.Job(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	ticket := oc.want("POST", "/v1/apps/"+devApp+"/upload-tickets", registry.Row{}, 0, 201)
	upload := func(files []artifact.File, want int) registry.Row {
		b, _ := json.Marshal(registry.Row{"entrypoint": "index.html", "files": files})
		r := httptest.NewRequest("POST", "/artifact/v1/upload", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+registry.Str(ticket["upload_token"]))
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		var v registry.Row
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		if w.Code != want {
			t.Fatal("upload", w.Code, v)
		}
		return v
	}
	files := []artifact.File{{Path: "index.html", Content: []byte("<!doctype html><h1>CRM release</h1>")}, {Path: "assets/main.js", Content: []byte("console.log('static')")}}
	build := upload(files, 201)
	if again := upload(files, 200); again["id"] != build["id"] {
		t.Fatal("upload replay changed build")
	}
	upload([]artifact.File{{Path: "../secret", Content: []byte("x")}}, 400)
	buildList := oc.want("GET", "/v1/apps/"+devApp+"/builds", nil, 0, 200)
	if len(buildList["items"].([]any)) != 1 {
		t.Fatal("missing uploaded build inventory")
	}
	rel := oc.want("POST", "/v1/apps/"+devApp+"/releases", delivery.ReleaseCreateInput{Name: "1.0.0", Build: registry.Str(build["id"]), Manifest: delivery.ReleaseManifest{Microapps: map[string]string{}, Dependencies: map[string]string{"react": "19.2.0"}}}, 2, 200)
	oc.want("POST", "/v1/releases/"+registry.Str(rel["id"])+"/preview", registry.Row{}, 2, 200)
	job := oc.want("POST", "/v1/releases/"+registry.Str(rel["id"])+"/deploy", registry.Row{}, 2, 202)
	oc.want("PATCH", "/v1/entities/"+eid, registry.Row{"definition": definition}, 3, 409)
	assertSecurity := func() {
		t.Helper()
		actor, access, err := s.authoring.ActorContext(ctx, s.DB, joe, tenant, space, app)
		if err != nil {
			t.Fatal(err)
		}
		active, err := registry.QueryRow(ctx, s.DB, "SELECT ev.definition_json FROM entity_definitions e JOIN entity_versions ev ON ev.id=e.active_version_id WHERE e.id=?", eid)
		if err != nil {
			t.Fatal(err)
		}
		d, err := entities.DefOf(active["definition_json"])
		if err != nil {
			t.Fatal(err)
		}
		if d.Allowed(eid, "read", []string{"name"}, actor, access, nil) {
			t.Fatal("deployment exposed a mixture of old ACLs and new role state")
		}
	}
	interrupted := false
	for i := 0; i < 8; i++ {
		if failure == "" && !interrupted {
			prepared, err := registry.ScanAll[publisher.Version](ctx, s.DB, "SELECT "+publisher.VersionColumns+" FROM entity_versions v JOIN spaces sp ON sp.id=v.space_id WHERE v.deployment_id=? AND v.state='prepared'", job["id"])
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared) > 0 {
				// Model a process loss after physical opening and before registry
				// commit. The old runtime must fail closed, and retry must resume.
				db, err := s.publisher.BusinessDB(ctx, source, true)
				if err != nil {
					t.Fatal(err)
				}
				for _, version := range prepared {
					var plan data.StoragePlan
					encoded, _ := json.Marshal(version.StoragePlan)
					if err = json.Unmarshal(encoded, &plan); err != nil {
						t.Fatal(err)
					}
					if err = publisher.OpenGate(ctx, db, version, plan.FieldsHash); err != nil {
						t.Fatal(err)
					}
				}
				db.Close()
				if status, _ := requestData("http://localhost:5390", prodToken, "POST", "/entities/company/query", registry.Row{}); status != 409 {
					t.Fatal("old runtime accepted an uncommitted storage version", status)
				}
				interrupted = true
			}
		}
		if failure != "" {
			var prepared int
			if e = s.DB.QueryRow("SELECT COUNT(*) FROM entity_versions WHERE deployment_id=? AND state='prepared'", job["id"]).Scan(&prepared); e != nil {
				t.Fatal(e)
			}
			if prepared > 0 && failure == "activation_failure" {
				// Fault injection after storage preparation: a unique origin conflict
				// forces rollback after policy activation and physical gate opening.
				host := strings.TrimPrefix(s.delivery.HostedOrigin(app), "http://")
				host = strings.Split(host, ":")[0]
				if _, e = s.DB.Exec("UPDATE apps SET origin_host=? WHERE id=?", host, devApp); e != nil {
					t.Fatal(e)
				}
			}
			if prepared > 0 && failure == "missing_build" {
				buildRow, err := registry.QueryRow(ctx, s.DB, "SELECT content_hash FROM app_builds WHERE id=?", build["id"])
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(s.Config.ArtifactDirectory, registry.Str(buildRow["content_hash"]), "index.html"), []byte("corrupt"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}

		if e = s.delivery.DeploymentJob(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
		if _, e = s.publisher.Job(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
		assertSecurity()
	}
	deployed := oc.want("GET", "/v1/deployments/"+registry.Str(job["id"]), nil, 0, 200)
	for _, id := range []string{app, devApp} {
		recovered := oc.want("GET", "/v1/apps/"+id+"/deployment", nil, 0, 200)
		if registry.Str(recovered["deployment"].(map[string]any)["id"]) != registry.Str(job["id"]) {
			t.Fatal("could not recover deployment", recovered)
		}
	}
	c.as(joe).want("GET", "/v1/apps/"+app+"/deployment", nil, 0, 403)
	if failure != "" {
		if deployed["state"] != "failed" {
			t.Fatal(deployed)
		}
		var pending int
		if e = s.DB.QueryRow("SELECT COUNT(*) FROM entity_definitions WHERE space_id=? AND pending_version_id IS NOT NULL", space).Scan(&pending); e != nil || pending != 0 {
			t.Fatal("failed deployment left pending versions", pending, e)
		}
		if status, v := requestData("http://localhost:5390", prodToken, "POST", "/entities/company/query", registry.Row{"include_count": true}); status != 200 || registry.Integer(v["count"]) != 1 {
			t.Fatal("failed deployment did not restore compatible old API", status, v)
		}
		return
	}
	if deployed["state"] != "active" {
		t.Fatal(deployed)
	}
	captured := oc.want("GET", "/v1/releases/"+registry.Str(rel["id"]), nil, 0, 200)
	rebased := oc.want("GET", "/v1/workspaces/"+wid, nil, 0, 200)
	if captured["base_hash"] != rel["base_hash"] || rebased["base_hash"] == rel["base_hash"] {
		t.Fatal("release base was not immutable or workspace was not rebased")
	}
	productionEntity := oc.want("GET", "/v1/entities/"+eid, nil, 0, 200)
	if !strings.Contains(registry.Str(productionEntity["active_definition"]), "notes") {
		t.Fatal("captured backend schema did not deploy")
	}
	host := strings.TrimPrefix(s.delivery.HostedOrigin(app), "http://")
	r := httptest.NewRequest("GET", "http://"+host+"/_lastapp/config.json", nil)
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), app) || strings.Contains(w.Body.String(), devApp) {
		t.Fatal("runtime config not rebound", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "http://"+host+"/_lastapp/builds/"+registry.Str(build["id"])+"/index.html", nil)
	w = httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "CRM release") {
		t.Fatal("static build", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "http://"+host+"/console/", nil)
	w = httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("App origin exposes console")
	}
	currentApp := oc.want("GET", base+"/apps/"+app, nil, 0, 200)
	oc.want("POST", "/v1/releases/"+registry.Str(rel["id"])+"/deploy", registry.Row{}, registry.Integer(currentApp["revision"]), 409)
	ws = oc.want("GET", "/v1/workspaces/"+wid, nil, 0, 200)
	oc.want("POST", "/v1/workspaces/"+wid+"/archive", registry.Row{}, registry.Integer(ws["revision"]), 200)
	if status, _ := requestData("http://localhost:5391", devToken, "POST", "/entities/company/query", registry.Row{}); status != 401 {
		t.Fatal("archived workspace token survived", status)
	}
}
