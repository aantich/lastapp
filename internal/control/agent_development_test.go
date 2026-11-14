package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/aantich/lastapp/internal/entity"
)

func TestAgentDevelopmentIsolation(t *testing.T) {
	s, oc, p, tenant, target, _ := agentSecurityFixture(t)
	ctx := context.Background()
	db, space := newID(), newID()
	if _, e := s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES(?,?,?,'workspace_metadata_fixture','registered','shared','ready',X'00')", db, tenant, target); e != nil {
		t.Fatal(e)
	}
	oc.want("POST", "/v1/tenants/"+tenant+"/spaces", Row{"id": space, "slug": "source", "display_name": "Source", "initial_manager_user_id": p.ID, "database": Row{"resource_id": db}}, 0, 201)
	permissions := []string{"read", "workspaces.write", "workspaces.test", "releases.write", "resources.write", "entities.draft", "entities.check", "entities.publish", "access.manage"}
	b, _ := json.Marshal(permissions)
	if _, e := s.DB.Exec("UPDATE agent_grants SET scope_kind='space',scope_id=?,permissions_json=?,limits_json='{}',revision=revision+1 WHERE id=?", space, b, p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	p.Agent, _ = s.readAgentGrant(ctx, s.DB, p.Agent.ID)
	other := p
	other.ID = p.ID
	grant := newID()
	if _, e := s.DB.Exec("INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,scope_id,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,'another-agent','Another agent',?,'space',?,?,'{}',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY),?)", grant, p.ID, tenant, space, b, s.mcpURL()); e != nil {
		t.Fatal(e)
	}
	other.Agent, _ = s.readAgentGrant(ctx, s.DB, grant)
	callWorkspace := func(actor Principal, name string) Row {
		t.Helper()
		key := newID()
		input := agentWorkspaceInput{Action: "create", Space: space, Name: name, RequestKey: key}
		raw, _ := json.Marshal(input)
		result, e := s.toolAgentWorkspace(ctx, actor, raw)
		if e != nil {
			t.Fatal(e)
		}
		ws := result.(Row)
		if _, e = s.toolAgentWorkspace(ctx, actor, raw); e != nil {
			t.Fatal("workspace replay", e)
		}
		if _, e = s.DB.Exec("UPDATE development_workspaces SET state='ready' WHERE id=?", ws["id"]); e != nil {
			t.Fatal(e)
		}
		if _, e = s.DB.Exec("UPDATE database_resources SET state='ready' WHERE id=?", ws["database_resource_id"]); e != nil {
			t.Fatal(e)
		}
		return ws
	}
	own, foreign := callWorkspace(p, "First agent"), callWorkspace(other, "Second agent")
	if _, e := s.workspaceRow(ctx, s.DB, p, str(foreign["id"])); e == nil {
		t.Fatal("agent accessed sibling workspace")
	}
	d := entity.Templates()[0]
	save := agentEntityInput{Target: &agentTarget{Kind: "space", ID: str(own["backing_space_id"])}, Definition: &d, RequestKey: newID()}
	raw, _ := json.Marshal(save)
	if _, e := s.toolAgentEntitySave(ctx, p, raw); e != nil {
		t.Fatal("owned workspace draft", e)
	}
	save.Target.ID = space
	save.RequestKey = newID()
	raw, _ = json.Marshal(save)
	if _, e := s.toolAgentEntitySave(ctx, p, raw); e == nil {
		t.Fatal("development grant wrote production")
	}
	save.Target.ID = str(foreign["backing_space_id"])
	save.RequestKey = newID()
	raw, _ = json.Marshal(save)
	if _, e := s.toolAgentEntitySave(ctx, p, raw); e == nil {
		t.Fatal("development grant wrote another agent workspace")
	}
	raw, _ = json.Marshal(agentReadInput{Target: &agentTarget{Kind: "space", ID: str(own["backing_space_id"])}})
	if _, e := s.toolAgentContext(ctx, p, raw); e != nil {
		t.Fatal("workspace context", e)
	}
	app := newID()
	oc.want("POST", "/v1/tenants/"+tenant+"/spaces/"+str(own["backing_space_id"])+"/apps", Row{"id": app, "slug": "client", "display_name": "Client", "initial_manager_user_id": p.ID}, 0, 201)
	for _, actor := range []Principal{p, other} {
		raw, _ = json.Marshal(agentReadInput{Resource: "app_deployment", ID: app})
		result, err := s.toolAgentGet(ctx, actor, raw)
		if actor.Agent.ID == p.Agent.ID {
			if err != nil || result.(Row)["deployment"] != nil {
				t.Fatal("owned App deployment recovery", result, err)
			}
		} else if err == nil {
			t.Fatal("another grant recovered a private workspace's deployment")
		}
	}
	// Workspaces support local origins even on a deployed HTTPS service; global
	// Development mode remains disabled for this policy check.
	s.Config.Development = false
	oc.want("PATCH", "/v1/apps/"+app+"/data-access", entity.AppAccess{Origin: "http://localhost:5190", Inherit: true}, 1, 200)
	verifier, nonce := token(32), token(32)
	raw, _ = json.Marshal(agentWorkspaceInput{Action: "launch", App: app, Launch: &developmentLaunchInput{Challenge: base64.RawURLEncoding.EncodeToString(hash(verifier)), Nonce: nonce, Origin: "http://localhost:5190"}, RequestKey: newID()})
	launch, e := s.toolAgentWorkspace(ctx, p, raw)
	if e != nil {
		t.Fatal(e)
	}
	code := launch.(Row)["code"]
	exchange, _ := json.Marshal(Row{"code": code, "verifier": verifier, "nonce": nonce})
	req := httptest.NewRequest("POST", "/data/v1/session/exchange", bytes.NewReader(exchange))
	req.Header.Set("Origin", "http://localhost:5190")
	response := httptest.NewRecorder()
	if e = s.exchange(response, req); e != nil {
		t.Fatal("agent PKCE exchange", e)
	}
	var session Row
	_ = json.Unmarshal(response.Body.Bytes(), &session)
	dataRequest := httptest.NewRequest("POST", "/data/v1/entities/company/query", nil)
	dataRequest.Header.Set("Origin", "http://localhost:5190")
	dataRequest.Header.Set("Authorization", "Bearer "+str(session["access_token"]))
	if _, e = s.dataSession(ctx, dataRequest); e != nil {
		t.Fatal("delegated development session", e)
	}
	if _, e = s.DB.Exec("UPDATE agent_grants SET revision=revision+1 WHERE id=?", p.Agent.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.dataSession(ctx, dataRequest); e == nil {
		t.Fatal("grant revision did not revoke development session")
	}
}
