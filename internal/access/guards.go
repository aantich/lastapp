package access

import (
	"context"
	"database/sql"
	"errors"

	"github.com/aantich/lastapp/internal/registry"
)

type WorkspaceOperationKey struct{}

func (s *Checker) WorkspaceOwner(ctx context.Context, q registry.Queryer, p Principal, v registry.Workspace) error {
	if v.Owner != p.ID {
		return registry.Missing()
	}
	if p.Agent != nil && Deref(v.OwnerGrant) != p.Agent.ID {
		return registry.Missing()
	}
	var active string
	if e := q.QueryRowContext(ctx, "SELECT state FROM spaces WHERE id=?", v.ProductionSpace).Scan(&active); e != nil {
		return e
	}
	if active != registry.StateActive {
		return registry.Conflict("The production Space is inactive.")
	}
	// Losing authority on the source also stops access to its development copies.
	if p.Agent != nil {
		original, e := s.ReadAgentGrant(ctx, q, p.Agent.ID)
		if e != nil {
			return e
		}
		if original.Revision != p.Agent.Revision {
			return registry.Missing()
		}
		p.Agent = original
	}
	return s.Authorize(ctx, q, p, "workspace.manage", Resource{Tenant: v.TenantID, Space: v.ProductionSpace}, true)
}

func (s *Checker) WorkspaceGuard(ctx context.Context, q registry.Queryer, p Principal, space string) error {
	if space == "" {
		return nil
	}
	v, e := registry.ScanOne[registry.Workspace](ctx, q, "SELECT "+registry.WorkspaceColumns+" FROM development_workspaces w WHERE w.backing_space_id=?", space)
	if e != nil {
		if ae, ok := e.(*registry.APIError); ok && ae.Status == 404 {
			return nil
		}
		return e
	}
	if e = s.WorkspaceOwner(ctx, q, p, v); e != nil {
		return e
	}
	if v.State != "ready" && ctx.Value(WorkspaceOperationKey{}) != v.ID {
		return registry.Conflict("Workspace is not ready. Inspect its status or retry preparation.")
	}
	return nil
}

func (s *Checker) ProductionMutationGuard(ctx context.Context, q registry.Queryer, p Principal, action string, res Resource) error {
	if res.App != "" {
		if e := q.QueryRowContext(ctx, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", res.App, res.Tenant).Scan(&res.Space); e != nil {
			return e
		}
	}
	if res.Space != "" && action != "workspace.manage" {
		var job string
		err := q.QueryRowContext(ctx, "SELECT id FROM release_deployments WHERE space_id=? AND state IN ('queued','preparing','publishing') LIMIT 1", res.Space).Scan(&job)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && ctx.Value(DeploymentOperationKey{}) != job {
			return registry.Conflict("A deployment is preparing this Space. Wait for its result before changing configuration.")
		}
	}
	if p.Agent == nil || res.Space == "" || action == "workspace.manage" {
		return nil
	}
	var workspace sql.NullString
	if e := q.QueryRowContext(ctx, "SELECT workspace_id FROM spaces WHERE id=?", res.Space).Scan(&workspace); e != nil {
		return e
	}
	if !workspace.Valid {
		return RequireAgent(p.Agent, "production.deploy")
	}
	return nil
}

type DeploymentOperationKey struct{}
