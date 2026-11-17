package control

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/agent"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// Audit attribution contract: a scripted flow of human console mutations, agent
// MCP mutations with request keys (including replays and a denied call) and
// worker-driven entity publications, dumped as normalised audit_events rows in
// commit order. IDs become stable labels; details keep only their keys and the
// agent attribution.
func TestAuditAttributionGolden(t *testing.T) {
	s, c, operator, owner, joe := foundationFixture(t)
	ctx := context.Background()
	rec := newShapeRecorder(t, c.h)
	oc := c.as(owner)
	var last int64
	if e := s.DB.QueryRow("SELECT COALESCE(MAX(id),0) FROM audit_events").Scan(&last); e != nil {
		t.Fatal(e)
	}
	labels := map[string]string{operator: "operator", owner: "owner", joe: "joe"}
	seen := map[string]string{}
	counts := map[string]int{}
	label := func(kind, id string) string {
		if id == "" {
			return "-"
		}
		if l, ok := labels[id]; ok {
			return l
		}
		if l, ok := seen[kind+":"+id]; ok {
			return l
		}
		counts[kind]++
		seen[kind+":"+id] = fmt.Sprintf("#%d", counts[kind])
		return seen[kind+":"+id]
	}
	var out strings.Builder
	// phase appends the audit rows committed since the previous phase.
	phase := func(name string) {
		t.Helper()
		rows, e := registry.QueryRows(ctx, s.DB, "SELECT id,tenant_id,actor_user_id,action,resource_type,resource_id,request_id,details_json FROM audit_events WHERE id>? ORDER BY id", last)
		if e != nil {
			t.Fatal(e)
		}
		fmt.Fprintf(&out, "## %s (%d)\n", name, len(rows))
		for _, row := range rows {
			last = registry.Integer(row["id"])
			actor := "system"
			if id := registry.Str(row["actor_user_id"]); id != "" {
				actor = "user:" + label("user", id)
			}
			details := map[string]any{}
			raw, _ := json.Marshal(row["details_json"]) // decoded by QueryRows
			if e := json.Unmarshal(raw, &details); e != nil {
				t.Fatal(e)
			}
			attribution := "human"
			if g, ok := details["agent_grant_id"].(string); ok {
				attribution = "agent grant:" + label("grant", g) + " client=" + registry.Str(details["agent_client_id"])
				inner, _ := details["details"].(map[string]any)
				details = inner
			}
			keys := make([]string, 0, len(details))
			for k := range details {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			fmt.Fprintf(&out, "%s actor=%s %s tenant=%s target=%s:%s request=%s details=[%s]\n", registry.Str(row["action"]), actor, attribution, label("tenant", registry.Str(row["tenant_id"])), registry.Str(row["resource_type"]), label(registry.Str(row["resource_type"]), registry.Str(row["resource_id"])), label("request", registry.Str(row["request_id"])), strings.Join(keys, ","))
		}
	}
	tenant, space, app, microapp := registry.NewID(), registry.NewID(), registry.NewID(), registry.NewID()
	labels[tenant], labels[space], labels[app], labels[microapp] = "tenant", "space", "app", "microapp"

	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "audit-" + tenant[:8], "display_name": "Audit golden", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	phase("console: platform operator creates a Tenant")
	resource, _ := businessDatabaseFixture(t, s, tenant, false)
	labels[resource] = "database"
	base := "/v1/tenants/" + tenant
	oc.want("POST", base+"/memberships", registry.Row{"user_id": joe}, 0, 201)
	oc.want("POST", base+"/spaces", registry.Row{"id": space, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner, "database": registry.Row{"resource_id": resource}}, 0, 201)
	oc.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"id": app, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": owner}, 0, 201)
	phase("console: Tenant administrator adds a member, Space and App")
	human := oc.want("POST", "/v1/spaces/"+space+"/entities", registry.Row{"definition": entity.Templates()[0]}, 0, 200)
	labels[registry.Str(human["id"])] = "human"
	oc.want("POST", "/v1/entities/"+registry.Str(human["id"])+"/publications", registry.Row{}, 1, 202)
	phase("console: draft and queue an entity publication")
	if _, e := s.publisher.Job(ctx, s.DB); e != nil {
		t.Fatal(e)
	}
	phase("worker: publish the human-requested entity")

	agentToken, grant := goldenGrant(t, s, owner, "tenant", tenant, "read", "resources.write", "entities.draft", "entities.check", "entities.publish", "access.manage", "production.deploy")
	readOnly, readGrant := goldenGrant(t, s, owner, "tenant", tenant, "read", "access.manage")
	labels[grant], labels[readGrant] = "agent", "read-only"
	create := registry.Row{"target": registry.Row{"kind": "app", "id": app}, "request_key": registry.NewID(), "microapp": registry.Row{"id": microapp, "slug": "audit", "display_name": "Audit"}}
	rec.mcp(s, agentToken, "create", "lastapp_create", create, true)
	phase("mcp: create a micro-app with a request key")
	rec.mcp(s, agentToken, "create replay", "lastapp_create", create, true)
	phase("mcp: replay the same request key")
	rec.mcp(s, readOnly, "create denied", "lastapp_create", registry.Row{"target": registry.Row{"kind": "app", "id": app}, "request_key": registry.NewID(), "microapp": registry.Row{"id": registry.NewID(), "slug": "denied", "display_name": "Denied"}}, false)
	phase("mcp: denied without resources.write")
	contact := entity.Templates()[1]
	saved := rec.mcp(s, agentToken, "entity save", "lastapp_entity_save", agent.EntityInput{Target: &agent.Target{Kind: "space", ID: space}, Definition: &contact, RequestKey: registry.NewID()}, true)
	agentEntity := registry.Str(mcpData(saved)["id"])
	labels[agentEntity] = "agent"
	publish := agent.PublishInput{Entity: agentEntity, Revision: 1, RequestKey: registry.NewID()}
	rec.mcp(s, agentToken, "entity publish", "lastapp_entity_publish", publish, true)
	rec.mcp(s, agentToken, "entity publish replay", "lastapp_entity_publish", publish, true)
	phase("mcp: draft and queue an entity publication, then replay it")
	for range 2 {
		if _, e := s.publisher.Job(ctx, s.DB); e != nil {
			t.Fatal(e)
		}
	}
	phase("worker: publish the agent-requested entity")
	oc.want("PATCH", base+"/apps/"+app, registry.Row{"display_name": "CRM audited"}, 1, 200)
	phase("console: human update after the agent's work")
	compareGolden(t, "audit_attribution.golden.txt", []byte(out.String()))
}
