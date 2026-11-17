package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// Record-share recipient search applies the same Tenant-directory rule as
// reference pickers: a record manager without membership.read/team.read finds
// only themselves and their own teams, never other members' names or emails.
func TestDataShareRecipientDirectory(t *testing.T) {
	s, c, operator, owner, joe := foundationFixture(t)
	ctx := context.Background()
	defer s.publisher.Close()
	oc, jc := c.as(owner), c.as(joe)
	tenant, space, app, role, team := registry.NewID(), registry.NewID(), registry.NewID(), registry.NewID(), registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "shares-" + tenant[:8], "display_name": "Share directory", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	resource, _ := businessDatabaseFixture(t, s, tenant, false)
	base := "/v1/tenants/" + tenant
	oc.want("POST", base+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	oc.want("POST", base+"/memberships", registry.Row{"user_id": operator}, 0, 201)
	oc.want("POST", base+"/teams", registry.Row{"id": team, "slug": "finance", "display_name": "Finance"}, 0, 201)
	oc.want("POST", base+"/spaces", registry.Row{"id": space, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": resource}}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": app, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/roles", registry.Row{"id": role, "code": "reader", "display_name": "Reader"}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/role-assignments", registry.Row{"id": registry.NewID(), "role_id": role, "user_id": joe}, 0, 201)
	// joe manages record shares but holds no Tenant-directory permission.
	d := entity.Definition{
		Format: 1, Key: "deal", Label: "Deal", Sharing: true, Owner: "owner", Projection: []string{"name"}, Rules: []entity.Rule{},
		Fields: []entity.Field{{Key: "name", Label: "Name", Type: "text", MaxLength: 100, Required: true}, {Key: "owner", Label: "Owner", Type: "user", Default: &entity.Expr{Context: "actor.id"}}},
		Permissions: []entity.Permission{
			{Key: "read", Label: "Read", Operation: "read", Relationship: "owner", Fields: []string{"name", "owner"}},
			{Key: "create", Label: "Create", Operation: "create", Relationship: "member", Fields: []string{"name", "owner"}},
			{Key: "shared_read", Label: "Shared read", Operation: "read", Roles: []string{role}, Fields: []string{"name"}},
			{Key: "manage", Label: "Share access", Operation: "acl.manage", Relationship: "user", Subject: joe, Delegable: []string{"shared_read"}},
		},
	}
	created := oc.want("POST", "/v1/spaces/"+space+"/entities", registry.Row{"definition": d}, 0, 200)
	oc.want("POST", "/v1/entities/"+registry.Str(created["id"])+"/publications", registry.Row{}, 1, 202)
	if _, e := s.publisher.Job(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	origin := "http://localhost:5591"
	oc.want("PATCH", "/v1/apps/"+app+"/data-access", entity.AppAccess{Origin: origin, Inherit: true}, 1, 200)
	request := func(token, method, path string, body any, status int) registry.Row {
		t.Helper()
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
		if w.Code != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return v
	}
	launch := func(client testClient) string {
		t.Helper()
		verifier, nonce := registry.Token(32), registry.Token(32)
		code := client.want("POST", "/v1/apps/"+app+"/launch", registry.Row{"challenge": base64.RawURLEncoding.EncodeToString(registry.TokenHash(verifier)), "nonce": nonce}, 0, 200)
		return registry.Str(request("", "POST", "/session/exchange", registry.Row{"code": code["code"], "verifier": verifier, "nonce": nonce}, 200)["access_token"])
	}
	ownerBearer, joeBearer := launch(oc), launch(jc)
	record := registry.NewID()
	request(ownerBearer, "POST", "/entities/deal/records", registry.Row{"id": record, "values": registry.Row{"name": "Deal"}}, 200)

	var operatorEmail string
	if e := s.DB.QueryRow("SELECT COALESCE(email,'') FROM users WHERE id=?", operator).Scan(&operatorEmail); e != nil {
		t.Fatal(e)
	}
	if operatorEmail == "" {
		operatorEmail = "share-directory-" + operator[:8] + "@example.test"
		if _, e := s.DB.Exec("UPDATE users SET email=? WHERE id=?", operatorEmail, operator); e != nil {
			t.Fatal(e)
		}
	}
	ids := func(v registry.Row, key string) []string {
		out := []string{}
		items, _ := v[key].([]any)
		for _, item := range items {
			out = append(out, registry.Str(item.(map[string]any)["id"]))
		}
		return out
	}
	options := func(token, kind, q string) []string {
		t.Helper()
		v := request(token, "GET", "/entities/deal/records/"+record+"/shares?view=options&recipient_kind="+kind+"&q="+url.QueryEscape(q), nil, 200)
		if kind == "user" && len(v["permissions"].([]any)) != 1 {
			t.Fatal("record manager cannot delegate the shareable permission", v)
		}
		return ids(v, "recipients")
	}
	has := func(list []string, id string) bool { return strings.Contains(strings.Join(list, ","), id) }

	// Without directory access: an email search finds nothing, an empty
	// search lists only the caller, and no team the caller is not in.
	if got := options(joeBearer, "user", operatorEmail); len(got) != 0 {
		t.Fatal("record manager without directory access discovered a member by email", got)
	}
	if got := options(joeBearer, "user", ""); len(got) != 1 || got[0] != joe {
		t.Fatal("recipient search exposed members beyond the caller", got)
	}
	if got := options(joeBearer, "team", ""); len(got) != 0 {
		t.Fatal("recipient search exposed teams the caller is not in", got)
	}
	// The reference picker of the same caller agrees: one rule.
	if got := ids(request(joeBearer, "GET", "/entities/deal/reference-options?field=owner", nil, 200), "items"); len(got) != 1 || got[0] != joe {
		t.Fatal("reference picker and recipient search disagree", got)
	}
	// Joining the team makes it discoverable without wider directory access.
	oc.want("POST", base+"/teams/"+team+"/members", registry.Row{"user_id": joe}, 1, 200)
	if got := options(joeBearer, "team", ""); len(got) != 1 || got[0] != team {
		t.Fatal("caller's own team is not a recipient option", got)
	}
	// Granting directory access (Tenant administrator) takes effect live on
	// the same App session: the same search now finds the member by email.
	oc.want("POST", base+"/role-assignments", registry.Row{"id": registry.NewID(), "user_id": joe, "role_id": access.TenantAdminRole}, 0, 201)
	if got := options(joeBearer, "user", operatorEmail); len(got) != 1 || got[0] != operator {
		t.Fatal("directory holder cannot find a member by email", got)
	}
	if got := options(joeBearer, "user", ""); !has(got, joe) || !has(got, operator) || !has(got, owner) {
		t.Fatal("directory holder does not see the Tenant members", got)
	}
	if got := ids(request(joeBearer, "GET", "/entities/deal/reference-options?field=owner", nil, 200), "items"); !has(got, operator) {
		t.Fatal("reference picker and recipient search disagree for a directory holder", got)
	}
	// Sharing with a known member ID stays possible; only discovery is scoped.
	request(joeBearer, "POST", "/entities/deal/records/"+record+"/shares", registry.Row{"permission_id": entity.PermissionID(registry.Str(created["id"]), "shared_read"), "recipient_kind": "user", "recipient_id": operator}, 200)
}
