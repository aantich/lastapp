package control

import (
	"context"
	"database/sql"
	"time"

	"github.com/aantich/lastapp/internal/artifact"
)

const (
	maxWorkspaceApps       = 100
	maxReleaseMicroapps    = 100
	maxReleaseDependencies = 100
	maxSpaceSnapshotBytes  = 1 << 20
	maxAppBuilds           = 100
	maxAppReleases         = 100
	maxOutstandingUploads  = 10
	uploadTicketLifetime   = 10 * time.Minute
)

func developmentPolicy() Row {
	return Row{"max_workspace_apps": maxWorkspaceApps, "max_release_microapps": maxReleaseMicroapps, "max_release_dependencies": maxReleaseDependencies, "max_snapshot_bytes": maxSpaceSnapshotBytes, "max_app_builds": maxAppBuilds, "max_app_releases": maxAppReleases, "max_owned_workspaces": maxOwnedWorkspaces, "max_space_workspaces": maxSpaceWorkspaces, "upload": artifact.Limits}
}

// Workspace Apps consume only the workspace's metadata capacity. Production
// subscription limits apply to production creation/restoration and deployment.
func appQuota(ctx context.Context, q Queryer, tenant, space string, policy Policy, creating bool) error {
	var workspace sql.NullString
	if e := q.QueryRowContext(ctx, "SELECT workspace_id FROM spaces WHERE tenant_id=? AND id=?", tenant, space).Scan(&workspace); e != nil {
		return e
	}
	if !workspace.Valid {
		return quota(ctx, q, tenant, "apps", policy.MaxApps)
	}
	if !creating {
		return nil // Restoring an existing App does not allocate another slot.
	}
	var count int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM apps WHERE space_id=?", space).Scan(&count); e != nil {
		return e
	}
	if count >= maxWorkspaceApps {
		return conflict("Workspace App limit reached.")
	}
	return nil
}

func (s *Server) snapshotAppQuota(ctx context.Context, q Queryer, tenant string, snap spaceSnapshot) error {
	policy, e := policyFor(ctx, q, tenant)
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
		if app.State != stateArchived && n == 0 {
			count++
		}
		if app.State == stateArchived && n == 1 {
			count--
		}
	}
	if int64(count) > *policy.MaxApps {
		return conflict("This release exceeds the Tenant's production App quota.")
	}
	return nil
}
