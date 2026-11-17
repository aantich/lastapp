package agent

import (
	"context"
	"encoding/json"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// DatabaseInput is the lastapp_database_provision argument.
type DatabaseInput struct {
	Action     string `json:"action" jsonschema:"provision or retry"`
	ID         string `json:"id"`
	TargetID   string `json:"target_id,omitempty"`
	Layout     string `json:"layout,omitempty"`
	Revision   int64  `json:"expected_revision,omitempty"`
	RequestKey string `json:"request_key"`
}

func (s *Server) toolAgentDatabase(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in DatabaseInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	if e := registry.ID(in.ID); e != nil {
		return nil, e
	}
	if p.Agent.Kind != "tenant" {
		return nil, registry.Bad("Provisioning requires a Tenant-scoped connection.")
	}
	tenant := p.Agent.Tenant
	var call func(context.Context, op.Actor) (any, error)
	switch in.Action {
	case "provision":
		db := databases.DatabaseInput{ID: in.ID, Mode: "managed", Layout: in.Layout, Target: in.TargetID}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Storage.CreateDatabase(ctx, a, tenant, db)
		}
	case "retry":
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Storage.RetryDatabase(ctx, a, tenant, in.ID, in.Revision)
		}
		if in.Revision < 1 {
			return nil, registry.Problem(428, "revision_required", "Retry requires expected_revision.")
		}
	default:
		return nil, registry.Bad("Unsupported database action.")
	}
	ctx, e := s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": "database", "id": in.ID}, in, "databases.provision")
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(call(ctx, op.AgentActor(ctx, p)))
	return agentWriteResult(ctx, data, e)
}
