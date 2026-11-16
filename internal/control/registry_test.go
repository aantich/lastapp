package control

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// sameJSON requires that got, an operation's typed registry result, marshals
// byte-for-byte like the untyped Row reader's result of the statement the
// operation ran before it was typed (keys, order, nulls, *_json documents,
// numbers and timestamps). A slice compares with every row; anything else
// with the first row. The statement must return at least one row.
func sameJSON(t *testing.T, q registry.Queryer, got any, untyped string, args ...any) {
	t.Helper()
	v, e := registry.QueryRows(context.Background(), q, untyped, args...)
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
func sameRaw(t *testing.T, q registry.Queryer, got json.RawMessage, untyped string, args ...any) {
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
	s, c, p, tenant, reviewSpace, app := agentSpaceFixture(t)
	ctx := context.Background()
	admin := op.Actor{Principal: access.Principal{ID: p.ID}, RequestID: registry.NewID()}
	limited := registry.Row{"id": registry.NewID(), "code": "limited-" + registry.NewID()[:8], "display_name": "Limited", "allow_shared_database": true, "allow_managed_database": true, "default_layout": "shared", "max_spaces": 2, "max_apps": 0}
	var operator string
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject='operator'", s.Config.OIDCIssuer).Scan(&operator); e != nil {
		t.Fatal(e)
	}
	c.as(operator).want("POST", "/admin/v1/plans", limited, 0, 201)
	root := op.Actor{Principal: access.Principal{ID: operator}, RequestID: registry.NewID()}

	plans, e := s.inv.Plans(ctx, root, op.PageRequest{Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, plans.Items, "SELECT id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases,revision,created_at FROM plans ORDER BY display_name,id LIMIT 200")
	plan, e := s.inv.Plan(ctx, root, registry.Str(limited["id"]))
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, plan, "SELECT id,code,display_name,allow_shared_database,allow_dedicated_database,allow_managed_database,allow_registered_database,allow_dedicated_compute,default_layout,max_spaces,max_apps,max_databases,revision,created_at FROM plans WHERE id=?", limited["id"])

	for i, artifact := range []string{"", "https://example.test/a.js"} {
		in := microappCreateInput{NamedInput: op.NamedInput{ID: registry.NewID(), Slug: "typed-" + registry.NewID()[:8], Name: "Typed"}, Artifact: artifact, Order: i}
		if _, e = s.inv.CreateMicroapp(ctx, admin, tenant, app, in); e != nil {
			t.Fatal(e)
		}
		m, e := s.inv.Microapp(ctx, admin, tenant, in.ID)
		if e != nil {
			t.Fatal(e)
		}
		sameJSON(t, s.DB, m, "SELECT id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order,state,revision FROM microapp_installations WHERE tenant_id=? AND id=?", tenant, in.ID)
	}
	microapps, e := s.inv.Microapps(ctx, admin, tenant, app, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, microapps.Items, "SELECT m.id,m.tenant_id,m.app_id,m.slug,m.display_name,m.artifact_ref,m.navigation_order,m.state,m.revision,m.created_at,a.display_name AS app_name,a.space_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.tenant_id=? AND m.app_id=? ORDER BY m.navigation_order,m.created_at DESC", tenant, app)

	// Tenants, Spaces and Apps, with a Tenant default layout set and unset.
	if _, e = s.DB.Exec("UPDATE tenants SET default_layout='shared' WHERE id=?", tenant); e != nil {
		t.Fatal(e)
	}
	c.as(operator).want("POST", "/admin/v1/tenants", registry.Row{"id": registry.NewID(), "slug": "typed-" + registry.NewID()[:8], "display_name": "Typed", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": p.ID}, 0, 201)
	tenants, e := s.inv.Tenants(ctx, root, op.PageRequest{Limit: 200})
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
	spaces, e := s.inv.Spaces(ctx, admin, tenant, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, spaces.Items, "SELECT sp.id,sp.tenant_id,sp.slug,sp.display_name,sp.database_resource_id,sp.database_layout,sp.state,sp.revision,sp.created_at,d.database_name,d.state AS database_state,(SELECT COUNT(*) FROM apps a WHERE a.space_id=sp.id AND a.state<>'archived') AS app_count FROM spaces sp JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.workspace_id IS NULL ORDER BY sp.created_at DESC,sp.id", tenant)
	space, e := s.inv.Space(ctx, admin, tenant, spaces.Items[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, space, "SELECT sp.id,sp.workspace_id,sp.tenant_id,t.display_name AS tenant_name,sp.slug,sp.display_name,sp.state,sp.revision,sp.database_resource_id,sp.database_layout,d.database_name,d.state AS database_state FROM spaces sp JOIN tenants t ON t.id=sp.tenant_id JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.id=?", tenant, space.ID)
	apps, e := s.inv.Apps(ctx, admin, tenant, "", op.PageRequest{})
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
		if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc,last_error_message) VALUES(?,?,?,?,'registered','shared',?,X'00',?)", registry.NewID(), tenant, target, "typed_attention_"+state, state, map[int]any{0: "Failed <check>", 1: nil}[i]); e != nil {
			t.Fatal(e)
		}
	}
	overview := rawFields(t, c.as(operator), "/v1/tenants/"+tenant+"/overview")
	sameRaw(t, s.DB, overview["tenant"], "SELECT t.id,t.slug,t.display_name,t.state,t.revision,t.plan_id,CASE WHEN t.default_layout='shared' AND p.allow_shared_database=1 THEN 'shared' WHEN t.default_layout='dedicated' AND p.allow_dedicated_database=1 THEN 'dedicated' ELSE p.default_layout END AS default_layout,p.display_name AS plan_name,p.max_spaces,p.max_apps,p.max_databases,p.allow_shared_database,p.allow_dedicated_database,p.allow_managed_database,p.allow_registered_database,p.allow_dedicated_compute FROM tenants t JOIN plans p ON p.id=t.plan_id WHERE t.id=?", tenant)
	sameRaw(t, s.DB, overview["recent_apps"], "SELECT a.id,a.space_id,a.display_name,a.state,sp.display_name AS space_name FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND a.state<>'archived' AND NOT EXISTS(SELECT 1 FROM spaces dsp WHERE dsp.id=a.space_id AND dsp.workspace_id IS NOT NULL) ORDER BY a.created_at DESC,a.id LIMIT 6", tenant)
	sameRaw(t, s.DB, overview["attention"], "SELECT id,database_name,state,last_error_message FROM database_resources WHERE tenant_id=? AND state IN ('pending','provisioning','failed') ORDER BY created_at DESC LIMIT 6", tenant)

	// Memberships, administrators, teams, team members and users; audited
	// prior states keep their JSON.
	var joe string
	if e = s.DB.QueryRow("SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject='outsider'", s.Config.OIDCIssuer).Scan(&joe); e != nil {
		t.Fatal(e)
	}
	if _, e = s.members.AddMembership(ctx, admin, tenant, accountChoice{User: joe}); e != nil {
		t.Fatal(e)
	}
	auditRequest := registry.NewID()
	if _, e = s.members.UpdateMembership(ctx, op.Actor{Principal: admin.Principal, RequestID: auditRequest}, tenant, joe, registry.StateDisabled, 1); e != nil {
		t.Fatal(e)
	}
	var details []byte
	if e = s.DB.QueryRow("SELECT details_json FROM audit_events WHERE request_id=? AND action='membership.updated'", auditRequest).Scan(&details); e != nil {
		t.Fatal(e)
	}
	var audited map[string]json.RawMessage
	if e = json.Unmarshal(details, &audited); e != nil || string(audited["before"]) != `{"revision":1,"state":"active","user_id":"`+joe+`"}` {
		t.Fatalf("audited membership before: %s %v", details, e)
	}
	members, e := s.members.Memberships(ctx, admin, tenant, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, members.Items, "SELECT m.user_id,m.state,m.revision,m.created_at,u.display_name,u.email,u.state AS user_state,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) AS platform_admin,ag.id AS administrator_id,ag.revision AS administrator_revision FROM tenant_memberships m JOIN users u ON u.id=m.user_id LEFT JOIN access_grants ag ON ag.user_id=m.user_id AND ag.tenant_id=m.tenant_id AND ag.role_id=? WHERE m.tenant_id=? ORDER BY u.display_name,m.user_id", access.SuperRole, access.TenantAdminRole, tenant)
	administrators, e := s.members.Administrators(ctx, root, tenant, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, administrators.Items, "SELECT g.id,g.user_id,g.revision,u.display_name,u.email,u.state AS user_state,m.state FROM access_grants g JOIN users u ON u.id=g.user_id JOIN tenant_memberships m ON m.tenant_id=g.tenant_id AND m.user_id=g.user_id WHERE g.tenant_id=? AND g.role_id=? ORDER BY u.display_name,g.id", tenant, access.TenantAdminRole)
	if _, e = s.members.UpdateMembership(ctx, admin, tenant, joe, registry.StateActive, 2); e != nil {
		t.Fatal(e)
	}
	team := op.NamedInput{ID: registry.NewID(), Slug: "typed", Name: "Typed"}
	if _, e = s.members.CreateTeam(ctx, admin, tenant, team); e != nil {
		t.Fatal(e)
	}
	if _, e = s.members.AddTeamMember(ctx, admin, tenant, team.ID, joe, 1); e != nil {
		t.Fatal(e)
	}
	teams, e := s.members.Teams(ctx, admin, tenant, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, teams.Items, "SELECT g.id,g.slug,g.display_name,g.state,g.revision FROM groups g WHERE g.tenant_id=? ORDER BY g.display_name,g.id", tenant)
	one3, e := s.members.Team(ctx, admin, tenant, team.ID)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, one3, "SELECT id,slug,display_name,state,revision FROM groups WHERE tenant_id=? AND id=?", tenant, team.ID)
	teamMembers, e := s.members.TeamMembers(ctx, admin, tenant, team.ID, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, teamMembers.Items, "SELECT gm.user_id,gm.state,u.display_name,u.state AS user_state,tm.state AS tenant_state,ga.id AS administrator_id,ga.revision AS administrator_revision,gr.id AS reviewer_id,gr.revision AS reviewer_revision,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) AS platform_admin FROM group_memberships gm JOIN users u ON u.id=gm.user_id JOIN tenant_memberships tm ON tm.tenant_id=gm.tenant_id AND tm.user_id=gm.user_id LEFT JOIN access_grants ga ON ga.tenant_id=gm.tenant_id AND ga.scope_team_id=gm.group_id AND ga.user_id=gm.user_id AND ga.role_id=? LEFT JOIN access_grants gr ON gr.tenant_id=gm.tenant_id AND gr.scope_team_id=gm.group_id AND gr.user_id=gm.user_id AND gr.role_id=? WHERE gm.tenant_id=? AND gm.group_id=? ORDER BY u.display_name,gm.user_id", access.SuperRole, authz.TeamAdminRole, authz.TeamReviewerRole, tenant, team.ID)
	users, e := s.Users(ctx, root, op.PageRequest{Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, users.Items, "SELECT id,display_name,email,oidc_issuer,oidc_subject,state,revision,created_at,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_admin,(SELECT pg.id FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_id,(SELECT pg.revision FROM access_grants pg WHERE pg.user_id=users.id AND pg.role_id=?) AS platform_assignment_revision FROM users ORDER BY display_name,id LIMIT 200", access.SuperRole, access.SuperRole, access.SuperRole)

	// Roles, assignments, recipients, effective assignments and the audited
	// security snapshots.
	spaceID := spaces.Items[0].ID
	scope, res, e := scopeOf(tenant, "", spaceID, "")
	if e != nil {
		t.Fatal(e)
	}
	role := roleInput{ID: registry.NewID(), Code: "typed", Name: "Typed"}
	if _, e = s.authority.CreateRole(ctx, admin, scope, res, role); e != nil {
		t.Fatal(e)
	}
	renamed, request := "Typed role", registry.NewID()
	if _, e = s.authority.UpdateRole(ctx, op.Actor{Principal: admin.Principal, RequestID: request}, scope, res, role.ID, rolePatch{Name: &renamed}, 1); e != nil {
		t.Fatal(e)
	}
	roleSnapshotSQL := "SELECT id,tenant_id,space_id,app_id,scope_kind,domain,code,display_name,state,revision FROM roles WHERE id=?"
	sameAudit := func(request, action, side, untyped string, extra registry.Row, args ...any) {
		t.Helper()
		var details []byte
		if e := s.DB.QueryRow("SELECT details_json FROM audit_events WHERE request_id=? AND action=?", request, action).Scan(&details); e != nil {
			t.Fatal(action, e)
		}
		v, e := registry.QueryRows(ctx, s.DB, untyped, args...)
		if e != nil || len(v) != 1 {
			t.Fatal(untyped, e)
		}
		for k, x := range extra {
			v[0][k] = x
		}
		var audited map[string]json.RawMessage
		want, _ := json.Marshal(v[0])
		if e = json.Unmarshal(details, &audited); e != nil || string(audited[side]) != string(want) {
			t.Fatalf("audited %s %s:\nwant %s\ngot  %s", action, side, want, audited[side])
		}
	}
	sameAudit(request, "role.updated", "after", roleSnapshotSQL, registry.Row{"permissions": []string{}}, role.ID)
	roles, e := s.authority.Roles(ctx, admin, scope, res, op.PageRequest{Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	untypedRoles, e := untypedPage(ctx, s.DB, op.PageRequest{Limit: 200}, op.CollectionFor("roles"), "SELECT id,tenant_id,space_id,app_id,code,display_name,state,revision,scope_kind,domain,(domain='control') AS protected FROM roles WHERE scope_kind=? AND (owner_key='builtin' OR owner_key=?) ORDER BY domain,display_name,id", scope.Kind, scope.OwnerKey())
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range untypedRoles.Items {
		v["permissions"] = []string{}
		if def, ok := authz.Builtin(registry.Str(v["id"])); ok {
			v["permissions"] = def.Permissions
		} else {
			v["data_permissions"] = []registry.Row{}
		}
	}
	if want, got := mustJSON(t, untypedRoles), mustJSON(t, roles); want != got {
		t.Fatalf("roles differ\nwant %s\ngot  %s", want, got)
	}
	one4, e := s.authority.Role(ctx, admin, scope, res, role.ID)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, one4, "SELECT id,tenant_id,scope_kind,domain,space_id,app_id,code,display_name,state,revision FROM roles WHERE id=?", role.ID)
	grant := assignmentInput{ID: registry.NewID(), User: joe, Role: role.ID}
	request = registry.NewID()
	if _, e = s.authority.Grant(ctx, op.Actor{Principal: admin.Principal, RequestID: request}, scope, res, grant); e != nil {
		t.Fatal(e)
	}
	grantSnapshotSQL := "SELECT id,tenant_id,scope_kind,domain,user_id,group_id,scope_team_id,space_id,app_id,role_id,revision FROM access_grants WHERE id=?"
	sameAudit(request, "grant.created", "after", grantSnapshotSQL, nil, grant.ID)
	assignments, e := s.authority.Assignments(ctx, admin, scope, res, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	where, args := assignmentFilter(scope)
	sameJSON(t, s.DB, assignments.Items, `SELECT g.id,g.scope_kind,g.domain,g.user_id,g.group_id,g.scope_team_id,g.space_id,g.app_id,g.role_id,g.revision,g.created_at,COALESCE(u.display_name,gr.display_name) AS principal_name,ro.display_name AS access_name,ro.state AS role_state,CASE WHEN g.scope_kind='platform' THEN 'Platform' ELSE COALESCE(team.display_name,sp.display_name,a.display_name,'Entire Tenant') END AS scope_name FROM access_grants g LEFT JOIN users u ON u.id=g.user_id LEFT JOIN groups gr ON gr.id=g.group_id JOIN roles ro ON ro.id=g.role_id LEFT JOIN groups team ON team.id=g.scope_team_id LEFT JOIN spaces sp ON sp.id=g.space_id LEFT JOIN apps a ON a.id=g.app_id WHERE `+where+" ORDER BY g.created_at DESC,g.id", args...)
	one5, e := s.authority.Assignment(ctx, admin, scope, res, grant.ID)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, one5, grantSnapshotSQL, grant.ID)
	recipients, e := s.authority.Recipients(ctx, admin, scope, res, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, recipients.Items, "SELECT u.id,u.id AS user_id,u.display_name,u.email FROM users u JOIN tenant_memberships m ON m.user_id=u.id WHERE m.tenant_id=? AND m.state='active' AND u.state='active' AND NOT EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) ORDER BY u.display_name,u.id", tenant, access.SuperRole)
	teamRecipients, e := s.authority.TeamRecipients(ctx, admin, scope, res, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, teamRecipients.Items, "SELECT id,display_name FROM groups WHERE tenant_id=? AND state='active' ORDER BY display_name,id", tenant)
	request = registry.NewID()
	if _, e = s.authority.Revoke(ctx, op.Actor{Principal: admin.Principal, RequestID: request}, scope, res, grant.ID, 1); e != nil {
		t.Fatal(e)
	}
	mine := rawFields(t, c, "/v1/tenants/"+tenant+"/my-role-assignments")
	sameRaw(t, s.DB, mine["items"], "SELECT * FROM (SELECT g.id,g.scope_kind,g.scope_team_id,g.space_id,g.app_id,g.role_id,g.group_id,ro.code,ro.display_name,ro.domain"+access.EffectiveAssignmentSQL[strings.Index(access.EffectiveAssignmentSQL, "\n"):]+") AS collection ORDER BY scope_kind,id", p.ID, tenant)
	if string(mine["pagination"]) != `{"page":1,"limit":100,"has_more":false}` {
		t.Fatal("my-role-assignments pagination", string(mine["pagination"]))
	}
	teamName := "Typed team"
	request = registry.NewID()
	if _, e = s.members.UpdateTeam(ctx, op.Actor{Principal: admin.Principal, RequestID: request}, tenant, team.ID, teamPatch{Name: &teamName}, 2); e != nil {
		t.Fatal(e)
	}
	sameAudit(request, "group.updated", "after", "SELECT id,display_name,state,revision FROM groups WHERE id=?", nil, team.ID)

	// Database targets and resources.
	if _, e = s.DB.Exec("UPDATE database_resources SET last_verified_at=UTC_TIMESTAMP(6),last_error_code='old' WHERE tenant_id=? AND state='ready'", tenant); e != nil {
		t.Fatal(e)
	}
	targets, e := s.storage.Targets(ctx, root, op.PageRequest{Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, targets.Items, "SELECT id,display_name,canonical_host,port,tls_mode,state,managed_enabled,revision,created_at FROM database_targets ORDER BY display_name,id LIMIT 200")
	for _, managed := range []bool{false, true} {
		available, e := s.storage.AvailableTargets(ctx, admin, tenant, managed, op.PageRequest{Limit: 200})
		if e != nil {
			t.Fatal(e)
		}
		filter := ""
		if managed {
			filter = " AND managed_enabled=1"
		}
		sameJSON(t, s.DB, available.Items, "SELECT id,display_name,managed_enabled FROM database_targets WHERE state='active'"+filter+" ORDER BY display_name,id LIMIT 200")
	}
	databaseSQL := "SELECT id,tenant_id,target_id,database_name,management_mode,layout,state,attempts,last_error_code,last_error_message,last_verified_at,revision,created_at FROM database_resources WHERE tenant_id=?"
	databases, e := s.storage.Databases(ctx, admin, tenant, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, databases.Items, databaseSQL+" ORDER BY created_at DESC,id", tenant)
	for _, d := range databases.Items {
		one, e := s.storage.Database(ctx, admin, tenant, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		sameJSON(t, s.DB, one, databaseSQL+" AND id=?", tenant, d.ID)
	}

	// Entity definitions, publications and inventories.
	customer := agentPublishedFixture(t, s, p, tenant, reviewSpace, entity.Definition{Format: 1, Key: "customer", Label: "Customer", Fields: []entity.Field{{Key: "name", Label: "Name", Type: "text", MaxLength: 80}}, Projection: []string{"name"}, Rules: []entity.Rule{}, Permissions: []entity.Permission{}})
	entityID := agentPublishedFixture(t, s, p, tenant, reviewSpace, entity.Definition{Format: 1, Key: "order", Label: "Order", Description: "Orders", Fields: []entity.Field{{Key: "title", Label: "Title", Type: "text", MaxLength: 80}, {Key: "customer", Label: "Customer", Type: "reference", Target: customer}}, Projection: []string{"title"}, Rules: []entity.Rule{}, Permissions: []entity.Permission{}})
	draft, e := s.authoring.CreateEntity(ctx, admin, reviewSpace, entityCreateInput{Definition: entity.Definition{Format: 1, Key: "note", Label: "Note", Fields: []entity.Field{{Key: "body", Label: "Body", Type: "text", MaxLength: 80}}, Projection: []string{"body"}, Rules: []entity.Rule{}, Permissions: []entity.Permission{}}})
	if e != nil {
		t.Fatal(e)
	}
	entities, e := s.authoring.Entities(ctx, admin, reviewSpace)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, entities["items"], "SELECT id,entity_key,label,state,revision,active_version_id,pending_version_id FROM entity_definitions WHERE space_id=? AND tenant_id=? ORDER BY label,id", reviewSpace, tenant)
	for _, key := range []string{entityID, draft.ID} {
		detail, e := s.authoring.detailOf(ctx, admin, key)
		if e != nil {
			t.Fatal(e)
		}
		if detail.Diagnostics == nil || detail.DraftContract == nil || detail.Publications == nil || (detail.ActiveVersion != nil) != (detail.PublishedContract != nil) {
			t.Fatal("entity view sections", mustJSON(t, detail))
		}
		if len(*detail.Publications) == 0 {
			if old, _ := registry.QueryRows(ctx, s.DB, "SELECT id FROM entity_versions WHERE entity_id=?", key); mustJSON(t, *detail.Publications) != mustJSON(t, old) {
				t.Fatal("empty publications", mustJSON(t, old))
			}
		} else {
			sameJSON(t, s.DB, *detail.Publications, "SELECT id,state,error_message,definition_hash,base_version_id,created_at FROM entity_versions WHERE entity_id=? ORDER BY created_at DESC LIMIT 20", key)
		}
		if detail.ActiveDefinition != nil {
			stored, e := registry.QueryRows(ctx, s.DB, "SELECT definition_json FROM entity_versions WHERE id=?", *detail.ActiveVersion)
			if e != nil || mustJSON(t, detail.ActiveDefinition) != mustJSON(t, stored[0]["definition_json"]) {
				t.Fatal("active definition", e)
			}
			publication, e := s.authoring.Publication(ctx, admin, key, *detail.ActiveVersion)
			if e != nil {
				t.Fatal(e)
			}
			sameJSON(t, s.DB, publication, "SELECT id,base_version_id,state,error_message,definition_hash,created_at,finished_at FROM entity_versions WHERE id=?", *detail.ActiveVersion)
		}
		detail.ActiveDefinition, detail.Diagnostics, detail.DraftContract, detail.Publications, detail.PublishedContract = nil, nil, nil, nil, nil
		sameJSON(t, s.DB, detail, "SELECT * FROM entity_definitions WHERE id=?", key)
	}
	// The agent inventory summarizes each draft as the untyped read did.
	inventory, e := registry.ScanAll[agentEntity](ctx, s.DB, `SELECT e.active_version_id,v.definition_hash,e.draft_json,e.entity_key,e.id,e.label,e.pending_version_id,e.revision,e.state FROM entity_definitions e LEFT JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? AND e.tenant_id=? ORDER BY e.label,e.id`, reviewSpace, tenant)
	if e != nil {
		t.Fatal(e)
	}
	untyped, e := registry.QueryRows(ctx, s.DB, `SELECT e.id,e.entity_key,e.label,e.state,e.revision,e.active_version_id,e.pending_version_id,e.draft_json,v.definition_hash FROM entity_definitions e LEFT JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? AND e.tenant_id=? ORDER BY e.label,e.id`, reviewSpace, tenant)
	if e != nil || len(untyped) != len(inventory) {
		t.Fatal("entity inventory", e)
	}
	for i, v := range untyped {
		d, e := defOf(v["draft_json"])
		if e != nil {
			t.Fatal(e)
		}
		fields, relationships := []registry.Row{}, []registry.Row{}
		for _, f := range d.Fields {
			fields = append(fields, registry.Row{"key": f.Key, "label": f.Label, "type": f.Type})
			if f.Target != "" {
				relationships = append(relationships, registry.Row{"field": f.Key, "target_entity_id": f.Target})
			}
		}
		v["description"], v["draft_fields"], v["draft_relationships"] = d.Description, fields, relationships
		delete(v, "draft_json")
		inventory[i].Fields, inventory[i].Relationships, inventory[i].Description = fields, relationships, d.Description
		if mustJSON(t, v) != mustJSON(t, inventory[i]) || inventory[i].SearchText() != v.SearchText() {
			t.Fatalf("entity inventory differs\nwant: %s\ngot:  %s", mustJSON(t, v), mustJSON(t, inventory[i]))
		}
	}
	if _, e = s.DB.Exec("INSERT INTO permissions(id,code,domain,description,owner_key,tenant_id,space_id,entity_id,version_id) SELECT ?,'order.read','data','Read orders',CONCAT('e:',id),tenant_id,space_id,id,active_version_id FROM entity_definitions WHERE id=?", registry.NewID(), entityID); e != nil {
		t.Fatal(e)
	}
	permissions, e := registry.ScanAll[agentPermission](ctx, s.DB, "SELECT code,description,entity_id,id,state,version_id FROM permissions WHERE tenant_id=? AND space_id=? AND domain='data' ORDER BY code,id", tenant, reviewSpace)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, permissions, "SELECT id,entity_id,version_id,code,description,state FROM permissions WHERE tenant_id=? AND space_id=? AND domain='data' ORDER BY code,id", tenant, reviewSpace)

	// The full App record.
	if _, e = s.DB.Exec("UPDATE apps SET approved_origin='https://typed.example',origin_host='typed.example' WHERE id=?", app); e != nil {
		t.Fatal(e)
	}
	record, e := s.appRow(ctx, s.DB, app)
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, record, "SELECT * FROM apps WHERE id=?", app)

	// Development workspaces (an unpublished draft blocks capture).
	if _, e = s.DB.Exec("UPDATE entity_definitions SET state='archived' WHERE id=?", draft.ID); e != nil {
		t.Fatal(e)
	}
	workspace, e := s.CreateWorkspace(ctx, admin, reviewSpace, workspaceCreateInput{Name: "Typed"})
	if e != nil {
		t.Fatal(e)
	}
	workspace, e = s.Workspace(ctx, admin, workspace.ID)
	if e != nil || workspace.Mappings == nil || len(*workspace.Mappings) == 0 {
		t.Fatal("workspace view", e)
	}
	untypedWorkspace, e := registry.QueryRows(ctx, s.DB, "SELECT w.*,sp.database_resource_id,d.state AS database_state FROM development_workspaces w JOIN spaces sp ON sp.id=w.backing_space_id JOIN database_resources d ON d.id=sp.database_resource_id WHERE w.id=?", workspace.ID)
	if e != nil {
		t.Fatal(e)
	}
	delete(untypedWorkspace[0], "base_json")
	untypedWorkspace[0]["mappings"], e = registry.QueryRows(ctx, s.DB, "SELECT kind,development_id,production_id FROM workspace_items WHERE workspace_id=? ORDER BY kind,development_id", workspace.ID)
	if e != nil || mustJSON(t, workspace) != mustJSON(t, untypedWorkspace[0]) {
		t.Fatalf("workspace differs\nwant: %s\ngot:  %s", mustJSON(t, untypedWorkspace[0]), mustJSON(t, workspace))
	}
	workspaces, e := s.Workspaces(ctx, admin, reviewSpace, op.PageRequest{})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, workspaces.Items, "SELECT id,display_name,production_space_id,backing_space_id,state,error_message,revision,created_at,owner_agent_grant_id FROM development_workspaces WHERE production_space_id=? AND owner_user_id=? ORDER BY created_at DESC,id", reviewSpace, p.ID)

	// The principal's Tenants and agent connections.
	myTenants := rawFields(t, c, "/v1/my-tenants")
	sameRaw(t, s.DB, myTenants["items"], "SELECT t.id,t.slug,t.display_name,t.state FROM tenant_memberships m JOIN tenants t ON t.id=m.tenant_id WHERE m.user_id=? AND m.state='active' ORDER BY t.display_name,t.id LIMIT 50", p.ID)
	agentGrants := rawFields(t, c, "/v1/agent-access")
	sameRaw(t, s.DB, agentGrants["items"], `SELECT g.id,g.user_id,g.client_id,g.client_name,g.tenant_id,g.scope_kind,g.scope_id,g.permissions_json,g.limits_json,g.state,g.revision,g.created_at,g.expires_at,g.last_used_at,t.display_name AS tenant_name,u.display_name AS owner_name,
 COALESCE(sp.display_name,a.display_name,team.display_name,t.display_name,'Platform') AS scope_name
 FROM agent_grants g JOIN users u ON u.id=g.user_id LEFT JOIN tenants t ON t.id=g.tenant_id
 LEFT JOIN spaces sp ON g.scope_kind='space' AND sp.id=g.scope_id AND sp.tenant_id=g.tenant_id
 LEFT JOIN apps a ON g.scope_kind='app' AND a.id=g.scope_id AND a.tenant_id=g.tenant_id
 LEFT JOIN groups team ON g.scope_kind='team' AND team.id=g.scope_id AND team.tenant_id=g.tenant_id WHERE g.user_id=? ORDER BY g.created_at DESC,g.id LIMIT 50`, p.ID)

	// The platform overview and the audit trail.
	platform := rawFields(t, c.as(operator), "/admin/v1/overview")
	sameRaw(t, s.DB, platform["database_states"], "SELECT state,COUNT(*) AS count FROM database_resources GROUP BY state")
	sameRaw(t, s.DB, platform["recent_databases"], "SELECT d.id,d.tenant_id,d.database_name,d.state,d.last_error_message,t.display_name AS tenant_name FROM database_resources d JOIN tenants t ON t.id=d.tenant_id ORDER BY d.created_at DESC LIMIT 6")
	audits, e := s.Audits(ctx, admin, tenant, op.PageRequest{Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	sameJSON(t, s.DB, audits.Items, "SELECT a.id,a.tenant_id,a.action,a.resource_type,a.resource_id,a.request_id,a.details_json,a.created_at,COALESCE(u.display_name,'System worker') AS actor_name FROM audit_events a LEFT JOIN users u ON u.id=a.actor_user_id WHERE a.tenant_id=? ORDER BY a.id DESC LIMIT 200", tenant)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

// jsonKeys lists a struct's JSON member names in field order, inlining
// embedded structs as encoding/json does.
func jsonKeys(t reflect.Type) []string {
	keys := []string{}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			keys = append(keys, jsonKeys(f.Type)...)
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.IsExported() && name != "" && name != "-" {
			keys = append(keys, name)
		}
	}
	return keys
}

// Typed records replace Rows, which marshal with sorted keys. Their JSON
// members must therefore be declared in sorted order (snapshot records keep
// their own documented order and are not Row replacements).
func TestRegistryRecordKeysAreSorted(t *testing.T) {
	for _, v := range []any{registry.Plan{}, Tenant{}, tenantItem{}, Space{}, SpaceItem{}, App{}, AppItem{}, Microapp{}, microappItem{}, Membership{}, Team{}, TeamMember{}, Administrator{}, User{}, Role{}, roleItem{}, rolePermission{}, Assignment{}, AssignmentItem{}, Recipient{}, Named{}, access.EffectiveAssignment{}, Target{}, AvailableTarget{}, Database{}, databaseAttention{}, recentApp{}, tenantProfile{}, EntityDefinition{}, entityItem{}, publicationItem{}, Publication{}, agentEntity{}, agentPermission{}, registry.AppRecord{}, registry.Workspace{}, WorkspaceItem{}, registry.WorkspaceMapping{}, Deployment{}, Release{}, ReleaseItem{}, Build{}, TenantRef{}, AgentGrantItem{}, stateCount{}, recentDatabase{}, AuditEvent{}} {
		keys := jsonKeys(reflect.TypeOf(v))
		if len(keys) == 0 || !slices.IsSorted(keys) {
			t.Errorf("%T JSON members are not in sorted order: %v", v, keys)
		}
	}
}

// untypedPage is the Row paging that the typed collections replaced.
func untypedPage(ctx context.Context, q registry.Queryer, req op.PageRequest, c op.Collection, query string, args ...any) (op.Page[registry.Row], error) {
	info, query, args, e := op.PageSQL(req, c, query, args)
	if e != nil {
		return op.Page[registry.Row]{}, e
	}
	v, e := registry.QueryRows(ctx, q, query, args...)
	if e != nil {
		return op.Page[registry.Row]{}, e
	}
	return op.PageOf(info, v), nil
}
