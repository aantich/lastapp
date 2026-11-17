package agent

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/inventory"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/roles"
)

// RoleInput is a custom role in a lastapp_create argument.
type RoleInput struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"display_name"`
}

// TenantInput is a Tenant in a lastapp_create argument.
type TenantInput struct {
	op.NamedInput
	Plan   string `json:"plan_id"`
	Layout string `json:"default_layout"`
	Admin  string `json:"initial_admin_user_id"`
}

// CreateInput is the lastapp_create argument.
type CreateInput struct {
	Target     *Target                        `json:"target,omitempty"`
	RequestKey string                         `json:"request_key" jsonschema:"Fresh canonical UUID for this logical mutation. Reuse unchanged arguments when retrying."`
	Space      *inventory.SpaceCreateInput    `json:"space,omitempty"`
	App        *inventory.AppCreateInput      `json:"app,omitempty"`
	Microapp   *inventory.MicroappCreateInput `json:"microapp,omitempty"`
	Team       *op.NamedInput                 `json:"team,omitempty"`
	Role       *RoleInput                     `json:"role,omitempty"`
	Tenant     *TenantInput                   `json:"tenant,omitempty"`
	Plan       *inventory.PlanInput           `json:"plan,omitempty"`
}

// UpdateInput is the lastapp_update argument.
type UpdateInput struct {
	Resource   string               `json:"resource"`
	ID         string               `json:"id"`
	Target     *Target              `json:"target,omitempty"`
	Revision   int64                `json:"expected_revision"`
	RequestKey string               `json:"request_key"`
	Name       *string              `json:"display_name,omitempty"`
	Compute    *string              `json:"compute_mode,omitempty"`
	Artifact   *string              `json:"artifact_ref,omitempty"`
	Order      *int                 `json:"navigation_order,omitempty"`
	Plan       *inventory.PlanInput `json:"plan,omitempty"`
	PlanID     *string              `json:"plan_id,omitempty"`
	Layout     *string              `json:"default_layout,omitempty"`
}

// LifecycleInput is the lastapp_lifecycle argument.
type LifecycleInput struct {
	Resource   string  `json:"resource"`
	ID         string  `json:"id"`
	Target     *Target `json:"target,omitempty"`
	State      string  `json:"state" jsonschema:"Supported stored state: active, disabled, archived, suspended or retired, according to resource type. No hard deletion."`
	Revision   int64   `json:"expected_revision"`
	RequestKey string  `json:"request_key"`
}

// inventoryUpdate selects the Space, App or micro-app metadata operation.
func (s *Server) inventoryUpdate(resource string) func(context.Context, op.Actor, string, string, inventory.MetadataPatch, int64) (op.Ref, error) {
	switch resource {
	case "space":
		return s.Inventory.UpdateSpace
	case "app":
		return s.Inventory.UpdateApp
	}
	return s.Inventory.UpdateMicroapp
}

func (s *Server) toolAgentCreate(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in CreateInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	res, target, e := s.agentRoot(p, in.Target)
	if e != nil {
		return nil, e
	}
	count := 0
	for _, v := range []any{in.Space, in.App, in.Microapp, in.Team, in.Role, in.Tenant, in.Plan} {
		if !reflect.ValueOf(v).IsNil() {
			count++
		}
	}
	if count != 1 {
		return nil, registry.Bad("Supply exactly one resource object.")
	}
	kind, key := "", ""
	var call func(context.Context, op.Actor) (any, error)
	permissions := []string{"resources.write"}
	switch {
	case in.Space != nil:
		if target.Kind != "tenant" {
			return nil, registry.Bad("Space creation requires a Tenant target.")
		}
		kind = "space"
		key = in.Space.ID
		if in.Space.Manager == "" {
			in.Space.Manager = p.ID
		}
		if in.Space.Manager != p.ID {
			permissions = append(permissions, "access.manage")
		}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Inventory.CreateSpace(ctx, a, res.Tenant, *in.Space)
		}
	case in.App != nil:
		if target.Kind != "space" {
			return nil, registry.Bad("App creation requires a Space target.")
		}
		kind = "app"
		key = in.App.ID
		if in.App.Manager == "" {
			in.App.Manager = p.ID
		}
		if in.App.Manager != p.ID {
			permissions = append(permissions, "access.manage")
		}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Inventory.CreateApp(ctx, a, res.Tenant, target.ID, *in.App)
		}
	case in.Microapp != nil:
		if target.Kind != "app" {
			return nil, registry.Bad("Micro-app creation requires an App target.")
		}
		kind = "microapp"
		key = in.Microapp.ID
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Inventory.CreateMicroapp(ctx, a, res.Tenant, target.ID, *in.Microapp)
		}
	case in.Team != nil:
		if target.Kind != "tenant" {
			return nil, registry.Bad("Team creation requires a Tenant target.")
		}
		kind = "team"
		key = in.Team.ID
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Members.CreateTeam(ctx, a, res.Tenant, *in.Team)
		}
	case in.Role != nil:
		if target.Kind != "space" && target.Kind != "app" {
			return nil, registry.Bad("Custom roles belong to a Space or App; Tenant/team roles are fixed.")
		}
		kind = "role"
		key = in.Role.ID
		role := roles.RoleInput{ID: in.Role.ID, Code: in.Role.Code, Name: in.Role.Name}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			scope, sr, e := agentScope(res, target)
			if e != nil {
				return nil, e
			}
			return s.Authority.CreateRole(ctx, a, scope, sr, role)
		}
		permissions = []string{"access.manage"}
	case in.Tenant != nil:
		if target.Kind != "platform" {
			return nil, registry.Missing()
		}
		kind = "tenant"
		key = in.Tenant.ID
		tenant := inventory.TenantCreateInput{ID: in.Tenant.ID, Slug: in.Tenant.Slug, Name: in.Tenant.Name, Plan: in.Tenant.Plan, Admin: in.Tenant.Admin, Layout: in.Tenant.Layout}
		call = func(ctx context.Context, a op.Actor) (any, error) { return s.Inventory.CreateTenant(ctx, a, tenant) }
		permissions = []string{"platform.manage"}
	case in.Plan != nil:
		if target.Kind != "platform" {
			return nil, registry.Missing()
		}
		kind = "plan"
		key = in.Plan.ID
		call = func(ctx context.Context, a op.Actor) (any, error) { return s.Inventory.CreatePlan(ctx, a, *in.Plan) }
		permissions = []string{"platform.manage"}
	}
	if e = registry.ID(key); e != nil {
		return nil, e
	}
	ctx, e = s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": kind, "id": key}, in, permissions...)
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(call(ctx, op.AgentActor(ctx, p)))
	return agentWriteResult(ctx, data, e)
}

func (s *Server) toolAgentUpdate(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in UpdateInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	res, target, e := s.agentRoot(p, in.Target)
	if e != nil {
		return nil, e
	}
	if e = registry.ID(in.ID); e != nil {
		return nil, e
	}
	permissions := []string{"resources.write"}
	var call func(context.Context, op.Actor) (any, error)
	switch in.Resource {
	case "space", "app", "microapp":
		update := s.inventoryUpdate(in.Resource)
		patch := inventory.MetadataPatch{Name: in.Name, Compute: in.Compute, Artifact: in.Artifact, Order: in.Order}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			if in.PlanID != nil || in.Layout != nil {
				return nil, registry.Bad("Invalid or unsupported JSON request fields.")
			}
			return update(ctx, a, res.Tenant, in.ID, patch, in.Revision)
		}
	case "team":
		patch := membership.TeamPatch{Name: in.Name}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			if in.Compute != nil || in.Artifact != nil || in.Order != nil || in.PlanID != nil || in.Layout != nil {
				return nil, registry.Bad("Invalid or unsupported JSON request fields.")
			}
			return s.Members.UpdateTeam(ctx, a, res.Tenant, in.ID, patch, in.Revision)
		}
	case "role":
		patch := roles.RolePatch{Name: in.Name}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			if in.Compute != nil || in.Artifact != nil || in.Order != nil || in.PlanID != nil || in.Layout != nil {
				return nil, registry.Bad("Invalid or unsupported JSON request fields.")
			}
			scope, sr, e := agentScope(res, target)
			if e != nil {
				return nil, e
			}
			return s.Authority.UpdateRole(ctx, a, scope, sr, in.ID, patch, in.Revision)
		}
		permissions = []string{"access.manage"}
	case "tenant":
		if p.Agent.Kind == "platform" {
			patch := inventory.TenantPatch{Name: in.Name, Plan: in.PlanID, Layout: in.Layout}
			call = func(ctx context.Context, a op.Actor) (any, error) {
				if in.Compute != nil || in.Artifact != nil || in.Order != nil {
					return nil, registry.Bad("Invalid or unsupported JSON request fields.")
				}
				return s.Inventory.UpdateTenant(ctx, a, in.ID, patch, in.Revision)
			}
			permissions = []string{"platform.manage"}
		} else {
			if in.ID != res.Tenant {
				return nil, registry.Missing()
			}
			call = func(ctx context.Context, a op.Actor) (any, error) {
				if in.Compute != nil || in.Artifact != nil || in.Order != nil || in.PlanID != nil || in.Layout != nil {
					return nil, registry.Bad("Invalid or unsupported JSON request fields.")
				}
				name := ""
				if in.Name != nil {
					name = *in.Name
				}
				return s.Inventory.UpdateTenantProfile(ctx, a, res.Tenant, name, in.Revision)
			}
		}
	case "plan":
		if target.Kind != "platform" || in.Plan == nil {
			return nil, registry.Bad("Plan update requires platform scope and the complete plan object.")
		}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Inventory.UpdatePlan(ctx, a, in.ID, *in.Plan, in.Revision)
		}
		permissions = []string{"platform.manage"}
	default:
		return nil, registry.Bad("Unsupported metadata resource.")
	}
	if in.Plan != nil && in.Resource != "plan" {
		return nil, registry.Bad("Plan object is only valid for plan updates.")
	}
	if in.Revision < 1 {
		return nil, registry.Problem(428, "revision_required", "expected_revision is required.")
	}
	ctx, e = s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": in.Resource, "id": in.ID}, in, permissions...)
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(call(ctx, op.AgentActor(ctx, p)))
	return agentWriteResult(ctx, data, e)
}

func (s *Server) toolAgentLifecycle(ctx context.Context, p access.Principal, raw json.RawMessage) (any, error) {
	var in LifecycleInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	res, target, e := s.agentRoot(p, in.Target)
	if e != nil {
		return nil, e
	}
	if e = registry.ID(in.ID); e != nil {
		return nil, e
	}
	permissions := []string{"lifecycle.manage", "access.manage"}
	var call func(context.Context, op.Actor) (any, error)
	switch in.Resource {
	case "space", "app", "microapp":
		update := s.inventoryUpdate(in.Resource)
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return update(ctx, a, res.Tenant, in.ID, inventory.MetadataPatch{State: &in.State}, in.Revision)
		}
	case "team":
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Members.UpdateTeam(ctx, a, res.Tenant, in.ID, membership.TeamPatch{State: &in.State}, in.Revision)
		}
	case "role":
		call = func(ctx context.Context, a op.Actor) (any, error) {
			scope, sr, e := agentScope(res, target)
			if e != nil {
				return nil, e
			}
			return s.Authority.UpdateRole(ctx, a, scope, sr, in.ID, roles.RolePatch{State: &in.State}, in.Revision)
		}
	case "database":
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Storage.UpdateDatabase(ctx, a, res.Tenant, in.ID, databases.DatabaseChange{State: in.State}, in.Revision)
		}
	case "tenant":
		if target.Kind != "platform" {
			return nil, registry.Bad("Tenant lifecycle requires platform scope.")
		}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			return s.Inventory.UpdateTenant(ctx, a, in.ID, inventory.TenantPatch{State: &in.State}, in.Revision)
		}
		permissions = append(permissions, "platform.manage")
	case "entity":
		v, e := s.Authoring.EntityRow(ctx, s.DB, op.AgentActor(ctx, p), in.ID, "entity.manage")
		if e != nil {
			return nil, e
		}
		call = func(ctx context.Context, a op.Actor) (any, error) {
			d, e := v.DraftDefinition()
			if e != nil {
				return nil, e
			}
			return s.Authoring.SaveEntity(ctx, a, in.ID, entities.EntitySaveInput{Definition: d, State: in.State}, in.Revision)
		}
		permissions = append(permissions, "entities.publish")
	default:
		return nil, registry.Bad("Unsupported lifecycle resource.")
	}
	if in.Revision < 1 {
		return nil, registry.Problem(428, "revision_required", "expected_revision is required.")
	}
	ctx, e = s.beginAgentWrite(ctx, p, in.RequestKey, registry.Row{"type": in.Resource, "id": in.ID}, in, permissions...)
	if e != nil {
		return agentWriteResult(ctx, nil, e)
	}
	data, e := agentSized(call(ctx, op.AgentActor(ctx, p)))
	return agentWriteResult(ctx, data, e)
}
