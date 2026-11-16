package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/aantich/lastapp/internal/entity"
)

// deploymentTask is a production deployment as its worker applies it, with
// the database resource of its Space.
type deploymentTask struct {
	ID, TenantID, SpaceID, ReleaseID, TargetApp, CreatedBy, State, Resource string
	AgentGrant                                                              string // empty for a person
	AgentRevision                                                           int64
	Plan                                                                    jsonValue
}

// deploymentTaskSQL selects a deploymentTask from release_deployments d; the
// query continues with a WHERE clause.
const deploymentTaskSQL = "SELECT d.id,d.tenant_id,d.space_id,d.release_id,d.target_app_id,d.created_by,d.state,sp.database_resource_id,COALESCE(d.agent_grant_id,''),COALESCE(d.agent_grant_revision,0),d.plan_json FROM release_deployments d JOIN spaces sp ON sp.id=d.space_id"

func (v *deploymentTask) fields() []any {
	return []any{&v.ID, &v.TenantID, &v.SpaceID, &v.ReleaseID, &v.TargetApp, &v.CreatedBy, &v.State, &v.Resource, &v.AgentGrant, &v.AgentRevision, &v.Plan}
}

func decodeDeployment(v jsonValue) (deploymentPlan, error) {
	var plan deploymentPlan
	b, e := json.Marshal(v)
	if e == nil {
		e = json.Unmarshal(b, &plan)
	}
	return plan, e
}

func (s *Server) deploymentPrincipal(ctx context.Context, q Queryer, v deploymentTask) (Principal, error) {
	p := Principal{ID: v.CreatedBy}
	if v.AgentGrant != "" {
		g, e := s.readAgentGrant(ctx, q, v.AgentGrant)
		if e != nil {
			return p, e
		}
		if g.Revision != v.AgentRevision {
			return p, problem(403, "agent_access_revoked", "Deployment delegation changed before activation.")
		}
		p.Agent = g
	}
	return p, s.deploymentAuthority(ctx, q, p, Resource{Tenant: v.TenantID, Space: v.SpaceID})
}

func (s *Server) deploymentJob(ctx context.Context, q Queryer) error {
	jobs, e := scanAll[deploymentTask](ctx, q, deploymentTaskSQL+" WHERE d.state IN ('queued','preparing','publishing') ORDER BY d.created_at LIMIT 1")
	if e != nil || len(jobs) == 0 {
		return e
	}
	v := jobs[0]
	ctx = context.WithValue(ctx, deploymentOperationKey{}, v.ID)
	ctx = agentAuditContext(ctx, v.AgentGrant, "")
	attempt, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	run := func() error {
		p, e := s.deploymentPrincipal(attempt, q, v)
		if e != nil {
			return e
		}
		plan, e := decodeDeployment(v.Plan)
		if e != nil {
			return e
		}
		if v.State == "queued" {
			if e = s.ensureReleaseBuild(attempt, q, v.ReleaseID); e != nil {
				return e
			}
		}
		if v.State == "queued" {
			tx, e := s.DB.BeginTx(attempt, nil)
			if e != nil {
				return e
			}
			defer tx.Rollback()
			if e = lockTenant(attempt, tx, v.TenantID); e != nil {
				return e
			}
			if _, e = s.deploymentPrincipal(attempt, tx, v); e != nil {
				return e
			}
			if e = s.applySnapshotMetadata(attempt, tx, v.TenantID, v.SpaceID, plan.Snapshot, false); e != nil {
				return e
			}
			if _, e = tx.ExecContext(attempt, "UPDATE release_deployments SET state='preparing',revision=revision+1 WHERE id=?", v.ID); e != nil {
				return e
			}
			return tx.Commit()
		}
		if v.State == "preparing" {
			defs, e := scanAll[EntityDefinition](attempt, q, "SELECT "+entityColumns+" FROM entity_definitions WHERE space_id=? ORDER BY id", v.SpaceID)
			if e != nil {
				return e
			}
			if e = s.prepareSnapshotTables(attempt, v.Resource, defs); e != nil {
				return e
			}
			tx, e := s.DB.BeginTx(attempt, nil)
			if e != nil {
				return e
			}
			defer tx.Rollback()
			if e = lockTenant(attempt, tx, v.TenantID); e != nil {
				return e
			}
			if _, e = s.deploymentPrincipal(attempt, tx, v); e != nil {
				return e
			}
			candidates := map[string]bool{}
			for _, d := range plan.Snapshot.Entities {
				if d.State == stateActive {
					candidates[d.ID] = true
				}
			}
			for _, d := range plan.Snapshot.Entities {
				var current EntityDefinition
				var activeHash string
				e := lookup(attempt, tx, "SELECT "+prefixed("e", entityColumns)+",COALESCE(ev.definition_hash,'') FROM entity_definitions e LEFT JOIN entity_versions ev ON ev.id=e.active_version_id WHERE e.id=?", []any{d.ID}, append(current.fields(), &activeHash)...)
				if e != nil {
					return e
				}
				if activeHash == entity.Hash(d.Definition) {
					continue
				}
				if e = s.queueDefinition(attempt, tx, p, current, plan.Versions[d.ID], candidates); e != nil {
					return e
				}
				if _, e = tx.ExecContext(attempt, "UPDATE entity_versions SET deployment_id=? WHERE id=?", v.ID, plan.Versions[d.ID]); e != nil {
					return e
				}
			}
			if _, e = tx.ExecContext(attempt, "UPDATE release_deployments SET state='publishing',revision=revision+1 WHERE id=?", v.ID); e != nil {
				return e
			}
			return tx.Commit()
		}
		for _, d := range plan.Snapshot.Entities {
			current, e := row(attempt, q, "SELECT ev.definition_hash,ev.state,e.pending_version_id FROM entity_definitions e LEFT JOIN entity_versions ev ON ev.id=e.active_version_id WHERE e.id=?", d.ID)
			if e != nil {
				return e
			}
			if str(current["definition_hash"]) == entity.Hash(d.Definition) && current["state"] == "published" && current["pending_version_id"] == nil {
				continue
			}
			var state string
			if e = q.QueryRowContext(attempt, "SELECT state FROM entity_versions WHERE id=?", plan.Versions[d.ID]).Scan(&state); e != nil {
				return e
			}
			if state == "failed" {
				return conflict("A backend publication failed. Live security policies and the previous client release remain selected. Inspect the entity publication before deploying again.")
			}
			if state != "prepared" {
				return nil
			}
		}
		if e = s.ensureReleaseBuild(attempt, q, v.ReleaseID); e != nil {
			return e
		}
		tx, e := s.DB.BeginTx(attempt, nil)
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if e = lockTenant(attempt, tx, v.TenantID); e != nil {
			return e
		}
		if _, e = s.deploymentPrincipal(attempt, tx, v); e != nil {
			return e
		}
		if e = s.applySnapshotMetadata(attempt, tx, v.TenantID, v.SpaceID, plan.Snapshot, true); e != nil {
			return e
		}
		var storage *sql.DB
		for _, captured := range plan.Snapshot.Entities {
			version, err := scanOne[entityVersion](attempt, tx, "SELECT "+versionColumns+" FROM entity_versions v JOIN spaces sp ON sp.id=v.space_id WHERE v.id=? AND v.deployment_id=? AND v.state='prepared'", plan.Versions[captured.ID], v.ID)
			if err != nil {
				if ae, ok := err.(*APIError); ok && ae.Status == 404 {
					continue
				} // Unchanged entity.
				return err
			}
			if e = s.activateEntityVersion(attempt, tx, version, captured.Definition); e != nil {
				return e
			}
			// Open the new physical version before committing its registry version.
			// Old runtime snapshots see a mismatch and fail closed until commit.
			if storage == nil {
				storage, e = s.businessDB(attempt, v.Resource, true)
				if e != nil {
					return e
				}
				defer storage.Close()
			}
			if e = openPublicationGate(attempt, storage, version, entity.Hash(captured.Definition.Fields)); e != nil {
				return e
			}
			if _, e = tx.ExecContext(attempt, "UPDATE entity_versions SET state='published',finished_at=UTC_TIMESTAMP(6) WHERE id=?", version.ID); e != nil {
				return e
			}
			if _, e = tx.ExecContext(attempt, "UPDATE entity_definitions SET pending_version_id=NULL,revision=revision+1 WHERE id=? AND pending_version_id=?", captured.ID, version.ID); e != nil {
				return e
			}
		}
		for _, app := range plan.Snapshot.Apps {
			// Preserve external origins for sibling Apps. Only this deployment's App
			// switches to the platform's dedicated static host.
			current, e := s.appRow(attempt, tx, app.ID)
			if e != nil {
				return e
			}
			old, e := accessOf(current.Access)
			if e != nil {
				return e
			}
			access := app.Access
			access.Origin = old.Origin
			if app.ID == v.TargetApp {
				access.Origin = s.appHostedOrigin(app.ID)
			}
			b, _ := json.Marshal(access)
			host := ""
			if access.Origin != "" {
				u, _ := url.Parse(access.Origin)
				host = u.Hostname()
			}
			if _, e = tx.ExecContext(attempt, "UPDATE apps SET data_access_json=?,approved_origin=?,origin_host=?,origin_generation=origin_generation+IF(?<>?,1,0),active_release_id=IF(id=?, ?,active_release_id),revision=revision+1 WHERE id=?", b, nullable(access.Origin), nullable(host), old.Origin, access.Origin, v.TargetApp, v.ReleaseID, app.ID); e != nil {
				return e
			}
			if e = projectAppPermissions(attempt, tx, app.ID, access); e != nil {
				return e
			}
		}
		// Only future releases use the updated base. Every existing release keeps
		// its original immutable base hash, preventing accidental stale rollback.
		currentBase, err := s.snapshot(attempt, tx, v.SpaceID)
		if err != nil {
			return err
		}
		baseJSON, _ := json.Marshal(currentBase)
		if _, err = tx.ExecContext(attempt, "UPDATE development_workspaces ws JOIN app_releases rel ON rel.workspace_id=ws.id SET ws.base_hash=?,ws.base_json=?,ws.revision=ws.revision+1 WHERE rel.id=?", entity.Hash(currentBase), baseJSON, v.ReleaseID); err != nil {
			return err
		}
		if _, e = tx.ExecContext(attempt, "UPDATE release_deployments SET state='active',finished_at=UTC_TIMESTAMP(6),revision=revision+1 WHERE id=?", v.ID); e != nil {
			return e
		}
		if e = audit(attempt, tx, v.TenantID, p.ID, "release.deployed", "deployment", v.ID, newID(), Row{"release_id": v.ReleaseID, "app_id": v.TargetApp}); e != nil {
			return e
		}
		return tx.Commit()
	}
	if e = run(); e == nil {
		return nil
	}
	// A lost commit acknowledgement must not undo a completed activation.
	var currentState string
	if err := q.QueryRowContext(ctx, "SELECT state FROM release_deployments WHERE id=?", v.ID).Scan(&currentState); err != nil {
		return err
	}
	if currentState == stateActive {
		return nil
	}
	message := "Deployment preparation failed. The previous client release remains selected. Review backend publications before retrying from a fresh workspace."
	if ae, ok := e.(*APIError); ok {
		message = ae.Message
	} else {
		s.logger(ctx).Warn("release deployment failed", diagnostic(e)...)
	}
	return s.failDeployment(ctx, v, message)
}

func (s *Server) failDeployment(ctx context.Context, job deploymentTask, message string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockTenant(ctx, tx, job.TenantID); err != nil {
		return err
	}
	versions, err := scanAll[entityVersion](ctx, tx, "SELECT "+versionColumns+" FROM entity_versions v JOIN spaces sp ON sp.id=v.space_id WHERE v.deployment_id=? AND v.state IN ('queued','publishing','prepared')", job.ID)
	if err != nil {
		return err
	}
	for _, version := range versions {
		if _, err = tx.ExecContext(ctx, "UPDATE entity_versions SET state='failed',error_message=?,finished_at=UTC_TIMESTAMP(6) WHERE id=?", message, version.ID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE entity_definitions SET pending_version_id=NULL,revision=revision+1 WHERE id=? AND pending_version_id=?", version.EntityID, version.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE release_deployments SET state='failed',error_message=?,finished_at=UTC_TIMESTAMP(6),revision=revision+1 WHERE id=? AND state IN ('queued','preparing','publishing')", message, job.ID); err != nil {
		return err
	}
	if err = audit(ctx, tx, job.TenantID, job.CreatedBy, "release.deployment_failed", "deployment", job.ID, newID(), Row{"error_message": message}); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, version := range versions {
		s.restorePublicationBase(ctx, version)
	}
	return nil
}

func (s *Server) applySnapshotMetadata(ctx context.Context, tx *sql.Tx, tenant, space string, snapshot spaceSnapshot, activate bool) error {
	if e := s.snapshotAppQuota(ctx, tx, tenant, snapshot); e != nil {
		return e
	}
	exists := func(table, id string) (bool, error) {
		var parent string
		query := "SELECT space_id FROM " + table + " WHERE id=?"
		e := tx.QueryRowContext(ctx, query, id).Scan(&parent)
		if errors.Is(e, sql.ErrNoRows) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		if parent != space {
			return false, missing()
		}
		return true, nil
	}
	for _, v := range snapshot.Apps {
		ok, e := exists("apps", v.ID)
		if e != nil {
			return e
		}
		if ok {
			if activate {
				_, e = tx.ExecContext(ctx, "UPDATE apps SET display_name=?,state=?,revision=revision+1 WHERE id=?", v.Name, v.State, v.ID)
			}
		} else {
			_, e = tx.ExecContext(ctx, "INSERT INTO apps(id,tenant_id,space_id,slug,display_name,state,compute_mode) VALUES(?,?,?,?,?,?,'shared')", v.ID, tenant, space, v.Slug, v.Name, v.State)
		}
		if e != nil {
			return e
		}
	}
	for _, v := range snapshot.Roles {
		ok, e := exists("roles", v.ID)
		if e != nil {
			return e
		}
		if ok {
			if activate {
				_, e = tx.ExecContext(ctx, "UPDATE roles SET display_name=?,state=?,revision=revision+1 WHERE id=? AND domain='data'", v.Name, v.State, v.ID)
			}
		} else {
			if e = metadataQuota(ctx, tx, tenant, "roles"); e != nil {
				return e
			}
			kind, owner := "space", "s:"+space
			if v.App != "" {
				kind, owner = "app", "a:"+v.App
			}
			_, e = tx.ExecContext(ctx, "INSERT INTO roles(id,tenant_id,space_id,app_id,owner_key,scope_kind,domain,code,display_name,state) VALUES(?,?,?,?,?,?,'data',?,?,?)", v.ID, tenant, space, nullable(v.App), owner, kind, v.Code, v.Name, stagedState(activate, v.State, stateDisabled))
		}
		if e != nil {
			return e
		}
	}
	for _, v := range snapshot.Entities {
		ok, e := exists("entity_definitions", v.ID)
		if e != nil {
			return e
		}
		b, _ := json.Marshal(v.Definition)
		if ok {
			_, e = tx.ExecContext(ctx, "UPDATE entity_definitions SET draft_json=?,label=IF(?, ?,label),state=IF(?, ?,state),revision=revision+1 WHERE id=?", b, activate, v.Definition.Label, activate, v.State, v.ID)
		} else {
			_, e = tx.ExecContext(ctx, "INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,state,draft_json) VALUES(?,?,?,?,?,?,?)", v.ID, tenant, space, v.Definition.Key, v.Definition.Label, stagedState(activate, v.State, stateArchived), b)
		}
		if e != nil {
			return e
		}
	}
	if !activate {
		return nil
	}
	for _, v := range snapshot.Microapps {
		var parent string
		e := tx.QueryRowContext(ctx, "SELECT a.space_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.id=?", v.ID).Scan(&parent)
		if e == nil {
			if parent != space {
				return missing()
			}
			_, e = tx.ExecContext(ctx, "UPDATE microapp_installations SET display_name=?,artifact_ref=?,navigation_order=?,state=?,revision=revision+1 WHERE id=?", v.Name, nullable(v.Artifact), v.Order, v.State, v.ID)
		} else if errors.Is(e, sql.ErrNoRows) {
			if e = metadataQuota(ctx, tx, tenant, "microapp_installations"); e != nil {
				return e
			}
			_, e = tx.ExecContext(ctx, "INSERT INTO microapp_installations(id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order,state) VALUES(?,?,?,?,?,?,?,?)", v.ID, tenant, v.App, v.Slug, v.Name, nullable(v.Artifact), v.Order, v.State)
		}
		if e != nil {
			return e
		}
	}
	return nil
}

func stagedState(activate bool, captured, inactive string) string {
	if activate {
		return captured
	}
	return inactive
}
