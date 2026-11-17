package delivery

import (
	"context"

	"github.com/aantich/lastapp/internal/artifact"
	"github.com/aantich/lastapp/internal/registry"
)

func Policy() registry.Row {
	return registry.Row{"max_workspace_apps": registry.MaxWorkspaceApps, "max_release_microapps": registry.MaxReleaseMicroapps, "max_release_dependencies": registry.MaxReleaseDependencies, "max_snapshot_bytes": registry.MaxSpaceSnapshotBytes, "max_app_builds": registry.MaxAppBuilds, "max_app_releases": registry.MaxAppReleases, "max_owned_workspaces": maxOwnedWorkspaces, "max_space_workspaces": maxSpaceWorkspaces, "upload": artifact.Limits}
}

func (s *Service) snapshotAppQuota(ctx context.Context, q registry.Queryer, tenant string, snap spaceSnapshot) error {
	policy, e := registry.PolicyFor(ctx, q, tenant)
	if e != nil {
		return e
	}
	if policy.MaxApps == nil {
		return nil
	}
	var count int
	if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND sp.workspace_id IS NULL AND a.state<>'archived'", tenant).Scan(&count); e != nil {
		return e
	}
	for _, app := range snap.Apps {
		var n int
		if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM apps WHERE id=? AND state<>'archived'", app.ID).Scan(&n); e != nil {
			return e
		}
		if app.State != registry.StateArchived && n == 0 {
			count++
		}
		if app.State == registry.StateArchived && n == 1 {
			count--
		}
	}
	if int64(count) > *policy.MaxApps {
		return registry.Conflict("This release exceeds the Tenant's production App quota.")
	}
	return nil
}
