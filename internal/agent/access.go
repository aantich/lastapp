package agent

import (
	"context"
	"encoding/json"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

// AssignmentInput is the lastapp_role_assignment argument.
type AssignmentInput struct {
	Action     string  `json:"action" jsonschema:"grant or revoke"`
	Target     *Target `json:"target,omitempty"`
	ID         string  `json:"id" jsonschema:"Assignment UUID: a new UUID on grant, existing UUID on revoke."`
	Role       string  `json:"role_id,omitempty"`
	User       string  `json:"user_id,omitempty"`
	Team       string  `json:"team_id,omitempty"`
	Revision   int64   `json:"expected_revision,omitempty"`
	RequestKey string  `json:"request_key"`
}

// MembershipInput is the lastapp_membership argument.
type MembershipInput struct {
	Action     string  `json:"action" jsonschema:"add, set_state or remove (remove is team-only)"`
	Target     *Target `json:"target,omitempty"`
	User       string  `json:"user_id"`
	State      string  `json:"state,omitempty"`
	Revision   int64   `json:"expected_revision,omitempty" jsonschema:"Tenant membership revision or containing team revision, as applicable."`
	RequestKey string  `json:"request_key"`
}

// AppAccessInput is the lastapp_app_access_update argument.
type AppAccessInput struct {
	App        string           `json:"app_id"`
	Access     entity.AppAccess `json:"access"`
	Revision   int64            `json:"expected_revision"`
	RequestKey string           `json:"request_key"`
}

func (s *Server) toolAgentAssignment(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in AssignmentInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	res, target, e := s.agentRoot(p, in.Target)
	if e != nil {
		return nil, e
	}
	var call func(context.Context, op.Actor, authz.Scope, access.Resource) (any, error)
	switch in.Action {
	case "grant":
		if (in.User == "") == (in.Team == "") {
			return nil, registry.Bad("Choose exactly one user_id or team_id recipient.")
		}
		grant := roles.AssignmentInput{ID: in.ID, Role: in.Role, User: in.User, Group: in.Team}
		call = func(ctx context.Context, a op.Actor, scope authz.Scope, sr access.Resource) (any, error) {
			return s.Authority.Grant(ctx, a, scope, sr, grant)
		}
	case "revoke":
		if in.Revision < 1 {
			return nil, registry.Problem(428, "revision_required", "Revocation requires expected_revision.")
		}
		call = func(ctx context.Context, a op.Actor, scope authz.Scope, sr access.Resource) (any, error) {
			return s.Authority.Revoke(ctx, a, scope, sr, in.ID, in.Revision)
		}
	default:
		return nil, registry.Bad("Action must be grant or revoke.")
	}
	if e = registry.ID(in.ID); e != nil {
		return nil, e
	}
	permissions := []string{"access.manage"}
	if target.Kind == "platform" {
		permissions = append(permissions, "platform.manage")
	}
	ctx, e = s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": "role_assignment", "id": in.ID}, in, permissions...)
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(func() (any, error) {
		scope, sr, e := agentScope(res, target)
		if e != nil {
			return nil, e
		}
		return call(ctx, op.AgentActor(ctx, p), scope, sr)
	}())
	return agentWriteResult(ctx, data, e)
}

func (s *Server) toolAgentMembership(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in MembershipInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	res, target, e := s.agentRoot(p, in.Target)
	if e != nil {
		return nil, e
	}
	if target.Kind != "tenant" && target.Kind != "team" {
		return nil, registry.Bad("Membership target must be Tenant or team.")
	}
	if e = registry.ID(in.User); e != nil {
		return nil, e
	}
	var call func(context.Context, op.Actor) (any, error)
	switch in.Action {
	case "add":
		call = func(ctx context.Context, a op.Actor) (any, error) {
			if target.Kind == "team" {
				return s.Members.AddTeamMember(ctx, a, res.Tenant, target.ID, in.User, in.Revision)
			}
			return s.Members.AddMembership(ctx, a, res.Tenant, membership.AccountChoice{User: in.User})
		}
	case "set_state":
		if in.Revision < 1 {
			return nil, registry.Problem(428, "revision_required", "Membership/team revision is required.")
		}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			if target.Kind == "team" {
				return s.Members.SetTeamMemberState(ctx, a, res.Tenant, target.ID, in.User, in.State, in.Revision)
			}
			return s.Members.UpdateMembership(ctx, a, res.Tenant, in.User, in.State, in.Revision)
		}
	case "remove":
		if target.Kind != "team" {
			return nil, registry.Bad("Tenant membership is disable-only.")
		}
		if in.Revision < 1 {
			return nil, registry.Problem(428, "revision_required", "Team revision is required.")
		}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Members.RemoveTeamMember(ctx, a, res.Tenant, target.ID, in.User, in.Revision)
		}
	default:
		return nil, registry.Bad("Unsupported membership action.")
	}
	ctx, e = s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": "membership", "user_id": in.User, "target": target}, in, "access.manage")
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(call(ctx, op.AgentActor(ctx, p)))
	return agentWriteResult(ctx, data, e)
}

func (s *Server) toolAgentAppAccess(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in AppAccessInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	if e := registry.ID(in.App); e != nil {
		return nil, e
	}
	if in.Revision < 1 {
		return nil, registry.Problem(428, "revision_required", "App revision is required.")
	}
	ctx, e := s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": "app_access", "id": in.App}, in, "access.manage")
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(s.Authoring.SaveAppAccess(ctx, op.AgentActor(ctx, p), in.App, in.Access, in.Revision))
	return agentWriteResult(ctx, data, e)
}
