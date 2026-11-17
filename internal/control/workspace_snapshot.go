package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

func remapSnapshot(base spaceSnapshot, ids map[string]string) spaceSnapshot {
	// Only typed identity references are rewritten. Labels, formulas and arbitrary
	// literal JSON are data; a UUID-looking string is not automatically a reference.
	b, _ := json.Marshal(base)
	var out spaceSnapshot
	_ = json.Unmarshal(b, &out)
	remap := func(v string) string {
		if n, ok := ids[v]; ok {
			return n
		}
		return v
	}
	permissions := map[string]string{}
	for i := range out.Entities {
		e := &out.Entities[i]
		old := e.ID
		e.ID = remap(e.ID)
		for j := range e.Definition.Fields {
			f := &e.Definition.Fields[j]
			if f.Type == "reference" {
				f.Target = remap(f.Target)
			}
		}
		for j := range e.Definition.Permissions {
			p := &e.Definition.Permissions[j]
			permissions[entity.PermissionID(old, p.Key)] = entity.PermissionID(e.ID, p.Key)
			for k := range p.Roles {
				p.Roles[k] = remap(p.Roles[k])
			}
		}
	}
	gates := func(gs []entity.Gate) {
		for i := range gs {
			g := &gs[i]
			if p, ok := permissions[g.Permission]; ok {
				g.Permission = p
			}
			for j := range g.Roles {
				g.Roles[j] = remap(g.Roles[j])
			}
		}
	}
	for i := range out.Roles {
		v := &out.Roles[i]
		v.ID = remap(v.ID)
		v.App = remap(v.App)
	}
	for i := range out.Apps {
		v := &out.Apps[i]
		v.ID = remap(v.ID)
		v.Access.Origin = ""
		gates(v.Access.Ceiling)
		gates(v.Access.Restrictions)
	}
	for i := range out.Microapps {
		v := &out.Microapps[i]
		v.ID = remap(v.ID)
		v.App = remap(v.App)
	}
	return out
}

func snapshotIDs(base spaceSnapshot) map[string]string {
	ids := map[string]string{}
	for _, v := range base.Entities {
		ids[v.ID] = "entity"
	}
	for _, v := range base.Roles {
		ids[v.ID] = "role"
	}
	for _, v := range base.Apps {
		ids[v.ID] = "app"
	}
	for _, v := range base.Microapps {
		ids[v.ID] = "microapp"
	}
	return ids
}

func (s *Server) installSnapshot(ctx context.Context, tx *sql.Tx, base spaceSnapshot, tenant, space, workspace, user string) error {
	if len(base.Apps) > registry.MaxWorkspaceApps {
		return registry.Conflict("Space exceeds the workspace App limit.")
	}
	if e := registry.MetadataQuota(ctx, tx, tenant, "roles", len(base.Roles)); e != nil {
		return e
	}
	if e := registry.MetadataQuota(ctx, tx, tenant, "microapp_installations", len(base.Microapps)); e != nil {
		return e
	}
	ids := map[string]string{}
	for old, kind := range snapshotIDs(base) {
		ids[old] = registry.NewID()
		if _, e := tx.ExecContext(ctx, "INSERT INTO workspace_items(workspace_id,kind,development_id,production_id) VALUES(?,?,?,?)", workspace, kind, ids[old], old); e != nil {
			return e
		}
	}
	copy := remapSnapshot(base, ids)
	for _, v := range copy.Apps {
		b, _ := json.Marshal(v.Access)
		if _, e := tx.ExecContext(ctx, "INSERT INTO apps(id,tenant_id,space_id,slug,display_name,state,compute_mode,data_access_json) VALUES(?,?,?,?,?,?,'shared',?)", v.ID, tenant, space, v.Slug, v.Name, v.State, b); e != nil {
			return e
		}
	}
	for _, v := range copy.Roles {
		kind, owner := "space", "s:"+space
		if v.App != "" {
			kind, owner = "app", "a:"+v.App
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO roles(id,tenant_id,space_id,app_id,owner_key,scope_kind,domain,code,display_name,state) VALUES(?,?,?,?,?,?,'data',?,?,?)", v.ID, tenant, space, registry.Nullable(v.App), owner, kind, v.Code, v.Name, v.State); e != nil {
			return e
		}
	}
	for _, v := range copy.Entities {
		b, _ := json.Marshal(v.Definition)
		if _, e := tx.ExecContext(ctx, "INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,state,draft_json) VALUES(?,?,?,?,?,?,?)", v.ID, tenant, space, v.Definition.Key, v.Definition.Label, v.State, b); e != nil {
			return e
		}
	}
	for _, v := range copy.Microapps {
		if _, e := tx.ExecContext(ctx, "INSERT INTO microapp_installations(id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order,state) VALUES(?,?,?,?,?,?,?,?)", v.ID, tenant, v.App, v.Slug, v.Name, registry.Nullable(v.Artifact), v.Order, v.State); e != nil {
			return e
		}
	}
	return nil
}

// The single publication capture routine is shared by the authoring endpoint and
// bounded snapshot jobs. Candidate references are accepted only for a captured
// batch whose tables are prepared before any version can activate.
func (s *Server) queueDefinition(ctx context.Context, tx *sql.Tx, p access.Principal, v entities.EntityDefinition, version string, candidates map[string]bool) error {
	d, e := v.DraftDefinition()
	if e != nil {
		return e
	}
	if ds := d.Validate(); len(ds) > 0 {
		return registry.Bad(ds[0].Path + ": " + ds[0].Message)
	}
	if e = entities.ValidateEntityRefsCandidates(ctx, tx, d, v.TenantID, v.SpaceID, v.ID, candidates); e != nil {
		return e
	}
	if v.PendingVersion != nil {
		return registry.Conflict("A publication is already pending.")
	}
	b, _ := json.Marshal(d)
	plan := data.Plan(v.ID, "lastapp:"+entity.Hash(s.registryName)[:16]+":"+v.ID, d)
	pb, _ := json.Marshal(plan)
	if _, e = tx.ExecContext(ctx, "INSERT INTO entity_versions(id,tenant_id,space_id,entity_id,base_version_id,definition_json,definition_hash,created_by,storage_plan_json,storage_plan_hash,agent_grant_id) VALUES(?,?,?,?,?,?,?,?,?,?,?)", version, v.TenantID, v.SpaceID, v.ID, v.ActiveVersion, b, entity.Hash(d), p.ID, pb, entity.Hash(plan), agentID(p)); e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "UPDATE entity_definitions SET pending_version_id=?,revision=revision+1 WHERE id=?", version, v.ID)
	return e
}

func agentID(p access.Principal) any {
	if p.Agent != nil {
		return p.Agent.ID
	}
	return nil
}

// Empty tables are prepared without foreign keys, then the ordinary publisher
// verifies/adds those keys before activation. This also supports cyclic references.
func (s *Server) prepareSnapshotTables(ctx context.Context, resource string, entities []entities.EntityDefinition) error {
	db, e := s.businessDB(ctx, resource, true)
	if e != nil {
		return e
	}
	defer db.Close()
	for _, v := range entities {
		if v.ActiveVersion != nil {
			continue
		}
		d, e := v.DraftDefinition()
		if e != nil {
			return e
		}
		if ds := d.Validate(); len(ds) > 0 {
			return registry.Bad(ds[0].Message)
		}
		marker := "lastapp:" + entity.Hash(s.registryName)[:16] + ":" + v.ID
		var comment string
		e = db.QueryRowContext(ctx, "SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", strings.Trim(data.Table(v.ID), "`")).Scan(&comment)
		if e == nil {
			if comment != marker {
				return databases.OwnershipConflict()
			}
			continue
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if _, e = db.ExecContext(ctx, data.SkeletonSQL(v.ID, marker, d)); e != nil {
			return e
		}
	}
	return nil
}

func (s *Server) workspaceJob(ctx context.Context, q registry.Queryer) error {
	v, e := scanWorkspaceStorage(ctx, q, "WHERE w.state='preparing' ORDER BY w.created_at LIMIT 1")
	if ae, ok := e.(*registry.APIError); ok && ae.Status == 404 {
		return nil
	}
	if e != nil {
		return e
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, access.WorkspaceOperationKey{}, v.ID)
	p := access.Principal{ID: v.Owner}
	if access.Deref(v.OwnerGrant) != "" {
		p.Agent, e = s.ReadAgentGrant(ctx, q, access.Deref(v.OwnerGrant))
		if e != nil {
			return e
		}
	}
	run := func() error {
		if p.Agent != nil {
			if e = access.RequireAgent(p.Agent, "workspaces.write"); e != nil {
				return e
			}
		}
		if e = s.WorkspaceOwner(ctx, q, p, v); e != nil {
			return e
		}
		if access.Deref(v.DatabaseState) == "pending" || access.Deref(v.DatabaseState) == "provisioning" {
			return nil
		}
		if access.Deref(v.DatabaseState) != "ready" {
			return registry.Conflict("Workspace storage failed. Inspect database setup and retry workspace preparation.")
		}
		defs, e := registry.ScanAll[entities.EntityDefinition](ctx, q, "SELECT "+entities.EntityColumns+" FROM entity_definitions WHERE space_id=? ORDER BY id", v.BackingSpace)
		if e != nil {
			return e
		}
		if e = s.prepareSnapshotTables(ctx, access.Deref(v.DatabaseResource), defs); e != nil {
			return e
		}
		candidates := map[string]bool{}
		for _, d := range defs {
			candidates[d.ID] = true
		}
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if e = lockTenant(ctx, tx, v.TenantID); e != nil {
			return e
		}
		if e = s.WorkspaceOwner(ctx, tx, p, v); e != nil {
			return e
		}
		ready := true
		for _, d := range defs {
			if d.ActiveVersion != nil {
				continue
			}
			ready = false
			if d.PendingVersion != nil {
				continue
			}
			// A previous failure is surfaced rather than silently retried forever.
			var failures int
			if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM entity_versions WHERE entity_id=? AND state='failed'", d.ID).Scan(&failures); e != nil {
				return e
			}
			if failures > 0 {
				return registry.Conflict("Workspace entity preparation failed. Review publications before retrying.")
			}
			// The workspace job owns baseline publication authority; ordinary later
			// agent publications keep their own grants and permission checks.
			if e = s.queueDefinition(ctx, tx, access.Principal{ID: p.ID}, d, registry.NewID(), candidates); e != nil {
				return e
			}
		}
		if ready {
			apps, err := registry.ScanAll[registry.AppRecord](ctx, tx, "SELECT "+registry.AppRecordColumns+" FROM apps WHERE space_id=?", v.BackingSpace)
			if err != nil {
				return err
			}
			for _, a := range apps {
				access, err := registry.AccessOf(a.Access)
				if err != nil {
					return err
				}
				if err = entities.ProjectAppPermissions(ctx, tx, a.ID, access); err != nil {
					return err
				}
			}
			if _, e = tx.ExecContext(ctx, "UPDATE development_workspaces SET state='ready',revision=revision+1 WHERE id=? AND state='preparing'", v.ID); e != nil {
				return e
			}
		}
		return tx.Commit()
	}
	if e = run(); e == nil {
		return nil
	}
	message := "Workspace preparation failed. Check storage and publications, then retry."
	if ae, ok := e.(*registry.APIError); ok {
		message = ae.Message
	} else {
		httpapi.Logger(ctx, s.Log).Warn("workspace preparation failed", httpapi.Diagnostic(e)...)
	}
	_, err := q.ExecContext(parent, "UPDATE development_workspaces SET state='failed',error_message=?,revision=revision+1 WHERE id=? AND state='preparing'", message, v.ID)
	return err
}

func ensureReferenceConstraints(ctx context.Context, q registry.Queryer, entityID string, d entity.Definition) error {
	for _, f := range d.Fields {
		if f.Type != "reference" {
			continue
		}
		target := strings.Trim(data.Table(f.Target), "`")
		var n int
		if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=? AND REFERENCED_TABLE_NAME=? AND REFERENCED_COLUMN_NAME='id'", strings.Trim(data.Table(entityID), "`"), "f_"+f.Key, target).Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			continue
		}
		name := "fk_" + entity.Hash([]string{entityID, f.Key, f.Target})[:24]
		_, e := q.ExecContext(ctx, "ALTER TABLE "+data.Table(entityID)+" ADD CONSTRAINT `"+name+"` FOREIGN KEY (`tenant_id`,`space_id`,"+data.FieldColumn(f.Key)+") REFERENCES "+data.Table(f.Target)+" (`tenant_id`,`space_id`,`id`)")
		if e != nil {
			return e
		}
	}
	return nil
}

func (s *Server) publicationCandidates(ctx context.Context, q registry.Queryer, v entityVersion) (map[string]bool, error) {
	if v.Deployment != nil {
		var stored registry.JSONValue
		if e := registry.Lookup(ctx, q, "SELECT plan_json FROM release_deployments WHERE id=? AND state IN ('preparing','publishing')", []any{*v.Deployment}, &stored); e != nil {
			return nil, e
		}
		plan, e := decodeDeployment(stored)
		if e != nil {
			return nil, e
		}
		ids := map[string]bool{}
		for _, d := range plan.Snapshot.Entities {
			if d.State == registry.StateActive {
				ids[d.ID] = true
			}
		}
		return ids, nil
	}
	var n int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM development_workspaces WHERE backing_space_id=? AND state='preparing'", v.SpaceID).Scan(&n); e != nil {
		return nil, e
	}
	if n == 0 {
		return nil, nil
	}
	vs, e := registry.ScanStrings(ctx, q, "SELECT id FROM entity_definitions WHERE space_id=?", v.SpaceID)
	if e != nil {
		return nil, e
	}
	ids := map[string]bool{}
	for _, id := range vs {
		ids[id] = true
	}
	return ids, nil
}

func snapshotContractHash(v spaceSnapshot) string {
	b, _ := json.Marshal(v)
	var copy spaceSnapshot
	_ = json.Unmarshal(b, &copy)
	copy.Revision = 0
	for i := range copy.Entities {
		copy.Entities[i].Revision = 0
	}
	for i := range copy.Roles {
		copy.Roles[i].Revision = 0
	}
	for i := range copy.Apps {
		copy.Apps[i].Revision = 0
	}
	for i := range copy.Microapps {
		copy.Microapps[i].Revision = 0
	}
	return entity.Hash(copy)
}

// publishedDraft is an entity draft with its pending publication and the
// hash of its active definition ("" before the first publication).
type publishedDraft struct {
	Draft      registry.JSONValue
	Pending    *string
	ActiveHash string
}

func (v *publishedDraft) ScanFields() []any { return []any{&v.Draft, &v.Pending, &v.ActiveHash} }

func (s *Server) ensureSnapshotReady(ctx context.Context, q registry.Queryer, space string) error {
	drafts, e := registry.ScanAll[publishedDraft](ctx, q, "SELECT e.draft_json,e.pending_version_id,COALESCE(ev.definition_hash,'') FROM entity_definitions e LEFT JOIN entity_versions ev ON ev.id=e.active_version_id WHERE e.space_id=? AND e.state='active'", space)
	if e != nil {
		return e
	}
	for _, draft := range drafts {
		d, e := entities.DefOf(draft.Draft.Value())
		if e != nil {
			return e
		}
		if draft.Pending != nil || entity.Hash(d) != draft.ActiveHash {
			return registry.Conflict("Publish every active entity draft before capturing a workspace or release.")
		}
	}
	return nil
}
