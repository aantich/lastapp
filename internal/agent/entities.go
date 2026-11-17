package agent

import (
	"context"
	"encoding/json"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// EntityInput is the lastapp_entity_save argument.
type EntityInput struct {
	Entity     string             `json:"entity_id,omitempty"`
	Target     *Target            `json:"target,omitempty"`
	Definition *entity.Definition `json:"definition,omitempty" jsonschema:"Full authoritative entity definition. For updates preserve all unrelated fields/rules/ACLs; obtain it with lastapp_get first."`
	Revision   int64              `json:"expected_revision,omitempty"`
	RequestKey string             `json:"request_key"`
}

// Case is a synthetic lastapp_entity_check case.
type Case struct {
	App       string         `json:"app_id,omitempty"`
	User      string         `json:"user_id,omitempty"`
	Operation string         `json:"operation"`
	Record    map[string]any `json:"record,omitempty"`
	Values    map[string]any `json:"values,omitempty"`
}

// CheckInput is the lastapp_entity_check argument.
type CheckInput struct {
	Entity     string             `json:"entity_id"`
	Definition *entity.Definition `json:"definition,omitempty"`
	Cases      []Case             `json:"cases,omitempty" jsonschema:"Bounded synthetic cases; the schema's maxItems is authoritative. No actual business records are fetched or written."`
}

// PublishInput is the lastapp_entity_publish argument.
type PublishInput struct {
	Entity     string `json:"entity_id"`
	Revision   int64  `json:"expected_revision"`
	RequestKey string `json:"request_key"`
}

func (s *Server) toolAgentEntitySave(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in EntityInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	if in.Definition == nil {
		return nil, registry.Bad("definition is required.")
	}
	key := in.Entity
	var call func(context.Context, op.Actor) (any, error)
	if key == "" {
		res, target, e := s.agentRoot(p, in.Target)
		if e != nil {
			return nil, e
		}
		if target.Kind != "space" {
			return nil, registry.Bad("New entities require a Space target.")
		}
		key = StableID(p, in.RequestKey)
		create := entities.EntityCreateInput{ID: key, Definition: *in.Definition}
		space := agentPaths(res, target)["space"]
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Authoring.CreateEntity(ctx, a, space, create)
		}
	} else {
		if e := registry.ID(key); e != nil {
			return nil, e
		}
		if in.Revision < 1 {
			return nil, registry.Problem(428, "revision_required", "Existing drafts require expected_revision.")
		}
		save := entities.EntitySaveInput{Definition: *in.Definition}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Authoring.SaveEntity(ctx, a, key, save, in.Revision)
		}
	}
	ctx, e := s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": "entity", "id": key}, in, "entities.draft")
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(call(ctx, op.AgentActor(ctx, p)))
	return agentWriteResult(ctx, data, e)
}

func (s *Server) toolAgentEntityPublish(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in PublishInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	if e := registry.ID(in.Entity); e != nil {
		return nil, e
	}
	if in.Revision < 1 {
		return nil, registry.Problem(428, "revision_required", "expected_revision is required.")
	}
	version := StableID(p, in.RequestKey)
	ctx, e := s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": "publication", "id": version, "entity_id": in.Entity}, in, "entities.publish")
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(s.Authoring.PublishEntity(ctx, op.AgentActor(ctx, p), in.Entity, in.Revision))
	return agentWriteResult(ctx, data, e)
}

func (s *Server) toolAgentEntityCheck(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in CheckInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	if len(in.Cases) > agentMaxSyntheticCases {
		return nil, registry.Bad("Synthetic case count exceeds the published schema limit.")
	}
	a := op.AgentActor(ctx, p)
	v, e := s.Authoring.EntityRow(ctx, s.DB, a, in.Entity, "entity.manage")
	if e != nil {
		return nil, e
	}
	if in.Definition == nil {
		d, e := v.DraftDefinition()
		if e != nil {
			return nil, e
		}
		in.Definition = &d
	}
	result, e := s.Authoring.ValidateEntity(ctx, a, in.Entity, *in.Definition, "diagnostics")
	if e == nil {
		e = agentFits(result)
	}
	if e != nil {
		return nil, e
	}
	result["draft_revision"] = v.Revision
	result["candidate_hash"] = entity.Hash(in.Definition)
	result["publication_authority_required"] = []string{"entities.publish"}
	result["changes"] = registry.Row{"candidate_differs_from_draft": entity.Hash(in.Definition) != entity.Hash(v.Draft), "published_version": v.ActiveVersion, "activation_includes": "schema, rules and ACLs; supported storage changes are checked by the publication worker"}
	cases := []registry.Row{}
	for _, c := range in.Cases {
		data, e := s.Authoring.TestEntity(ctx, a, in.Entity, entities.EntityTestInput{Definition: *in.Definition, App: c.App, User: c.User, Operation: c.Operation, Record: c.Record, Values: c.Values})
		if e == nil {
			e = agentFits(data)
		}
		if e != nil {
			return nil, e
		}
		cases = append(cases, data)
	}
	result["cases"] = cases
	result["contract_guidance"] = "Inspect generated contracts with lastapp_get resource=entity and section selectors for saved drafts. Supplied candidates are not persisted."
	return result, nil
}
