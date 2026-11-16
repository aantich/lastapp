package control

import (
	"context"
	"errors"
	"testing"

	"github.com/aantich/lastapp/internal/entity"
)

func wantProblem(t *testing.T, e error, status int, code string) {
	t.Helper()
	o := classify(e)
	if e == nil || o.Status != status || o.Code != code {
		t.Fatalf("got %v (%d %s), want %d %s", e, o.Status, o.Code, status, code)
	}
}

// agentCall is the delegated call an MCP adapter builds for one mutation.
func agentCall(p Principal, operation string, input any, permissions ...string) *AgentCall {
	return &AgentCall{Grant: p.Agent, Permission: permissions, Operation: operation, Key: newID(), Hash: entity.Hash(Row{"operation": operation, "input": input}), ResultRef: Row{"type": "test"}}
}

func TestInventoryOperations(t *testing.T) {
	s, _, p, tenant, _, app := agentSpaceFixture(t)
	ctx := context.Background()
	human := Actor{Principal: Principal{ID: p.ID}, RequestID: newID()}

	// Console actor: create, revision rules and audit attribution.
	in := microappCreateInput{namedInput: namedInput{ID: newID(), Slug: "ops", Name: "Ops"}, Order: 2}
	ref, e := s.inv.CreateMicroapp(ctx, human, tenant, app, in)
	if e != nil || ref.ID != in.ID || ref.Revision != 1 {
		t.Fatal("console create", ref, e)
	}
	var request, details string
	if e = s.DB.QueryRow("SELECT request_id,details_json FROM audit_events WHERE action='microapp.created' AND resource_id=?", in.ID).Scan(&request, &details); e != nil || request != human.RequestID || details != `{"app_id":"`+app+`"}` {
		t.Fatal("console audit attribution", request, details, e)
	}
	name := "Renamed"
	_, e = s.inv.UpdateMicroapp(ctx, human, tenant, in.ID, metadataPatch{Name: &name}, 0)
	wantProblem(t, e, 428, "revision_required")
	_, e = s.inv.UpdateMicroapp(ctx, human, tenant, in.ID, metadataPatch{Name: &name}, 7)
	wantProblem(t, e, 412, "stale_revision")
	if ref, e = s.inv.UpdateMicroapp(ctx, human, tenant, in.ID, metadataPatch{Name: &name}, 1); e != nil || ref.Revision != 2 {
		t.Fatal("console update", ref, e)
	}
	page, e := s.inv.Microapps(ctx, human, tenant, app, PageRequest{Limit: 1})
	if e != nil || len(page.Items) != 1 || page.Pagination.Limit != 1 || page.Pagination.HasMore {
		t.Fatal("console list", page, e)
	}
	_, e = s.inv.Microapps(ctx, human, tenant, app, PageRequest{Limit: 201})
	wantProblem(t, e, 400, "invalid_request")

	// Denied scope: an outsider and a non-platform plan author.
	var outsider string
	if e = s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='outsider'").Scan(&outsider); e != nil {
		t.Fatal(e)
	}
	_, e = s.inv.CreateMicroapp(ctx, Actor{Principal: Principal{ID: outsider}, RequestID: newID()}, tenant, app, microappCreateInput{namedInput: namedInput{ID: newID(), Slug: "denied", Name: "Denied"}})
	wantProblem(t, e, 404, "not_found")
	_, e = s.inv.Plans(ctx, human, PageRequest{})
	wantProblem(t, e, 403, "forbidden")

	// Agent actor: receipt, replay, conflicting reuse and grant attribution.
	agentIn := microappCreateInput{namedInput: namedInput{ID: newID(), Slug: "agent", Name: "Agent"}}
	call := agentCall(p, "lastapp_create", agentIn, "resources.write")
	agent := Actor{Principal: p, RequestID: newID(), Agent: call}
	if _, e = s.inv.CreateMicroapp(ctx, agent, tenant, app, agentIn); e != nil || !call.Committed {
		t.Fatal("agent create", e)
	}
	if e = s.DB.QueryRow("SELECT details_json FROM audit_events WHERE action='microapp.created' AND resource_id=?", agentIn.ID).Scan(&details); e != nil || details != `{"agent_client_id":"review-client","agent_grant_id":"`+p.Agent.ID+`","details":{"app_id":"`+app+`"}}` {
		t.Fatal("agent audit attribution", details, e)
	}
	replay := &AgentCall{Grant: call.Grant, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: call.Hash, ResultRef: call.ResultRef}
	if _, e = s.inv.CreateMicroapp(ctx, Actor{Principal: p, RequestID: newID(), Agent: replay}, tenant, app, agentIn); !errors.Is(e, errAgentReplay) {
		t.Fatal("replay executed again", e)
	}
	reused := &AgentCall{Grant: call.Grant, Permission: call.Permission, Operation: call.Operation, Key: call.Key, Hash: "different"}
	_, e = s.inv.UpdateMicroapp(ctx, Actor{Principal: p, RequestID: newID(), Agent: reused}, tenant, agentIn.ID, metadataPatch{Name: &name}, 1)
	wantProblem(t, e, 409, "conflict")
	missingScope := agentCall(p, "lastapp_lifecycle", nil, "lifecycle.manage")
	archived := stateArchived
	_, e = s.inv.UpdateMicroapp(ctx, Actor{Principal: p, RequestID: newID(), Agent: missingScope}, tenant, agentIn.ID, metadataPatch{State: &archived}, 1)
	wantProblem(t, e, 403, "needs_authorization")
	if o := classify(e); o.Authority == nil || len(o.Authority.Missing) != 1 || o.Authority.Missing[0] != "lifecycle.manage" {
		t.Fatal("missing scopes not classified", e)
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM agent_operation_receipts WHERE grant_id=?", p.Agent.ID).Scan(&n); e != nil || n != 1 {
		t.Fatal("receipts", n, e)
	}
}
