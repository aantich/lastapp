package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

const (
	maxOwnedWorkspaces = 5
	maxSpaceWorkspaces = 20
)

type (
	workspaceOperationKey struct{}
	workspaceCreateInput  struct {
		ID   string `json:"id"`
		Name string `json:"display_name"`
	}
)

// A snapshot contains configuration only. Never add records, credentials,
// assignments or approved origins here: it is also the release review boundary.
type spaceSnapshot struct {
	Revision  int64              `json:"revision"`
	Entities  []snapshotEntity   `json:"entities"`
	Roles     []snapshotRole     `json:"roles"`
	Apps      []snapshotApp      `json:"apps"`
	Microapps []snapshotMicroapp `json:"microapps"`
}
type snapshotEntity struct {
	Revision   int64             `json:"revision"`
	ID         string            `json:"id"`
	State      string            `json:"state"`
	Definition entity.Definition `json:"definition"`
}
type snapshotRole struct {
	Revision int64  `json:"revision"`
	ID       string `json:"id"`
	App      string `json:"app_id,omitempty"`
	Code     string `json:"code"`
	Name     string `json:"display_name"`
	State    string `json:"state"`
}
type snapshotApp struct {
	Revision int64            `json:"revision"`
	ID       string           `json:"id"`
	Slug     string           `json:"slug"`
	Name     string           `json:"display_name"`
	State    string           `json:"state"`
	Access   entity.AppAccess `json:"access"`
}
type snapshotMicroapp struct {
	Revision int64  `json:"revision"`
	ID       string `json:"id"`
	App      string `json:"app_id"`
	Slug     string `json:"slug"`
	Name     string `json:"display_name"`
	Artifact string `json:"artifact_ref"`
	Order    int64  `json:"navigation_order"`
	State    string `json:"state"`
}

func (v *snapshotRole) ScanFields() []any {
	return []any{&v.Revision, &v.ID, &v.App, &v.Code, &v.Name, &v.State}
}

func (v *snapshotMicroapp) ScanFields() []any {
	return []any{&v.Revision, &v.ID, &v.App, &v.Slug, &v.Name, &v.Artifact, &v.Order, &v.State}
}

func (s *Server) snapshot(ctx context.Context, q registry.Queryer, space string) (spaceSnapshot, error) {
	var definitionBytes int64
	if e := q.QueryRowContext(ctx, "SELECT COALESCE(SUM(OCTET_LENGTH(v.definition_json)),0) FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=?", space).Scan(&definitionBytes); e != nil {
		return spaceSnapshot{}, e
	}
	if definitionBytes > maxSpaceSnapshotBytes {
		return spaceSnapshot{}, registry.Conflict("Space configuration exceeds the workspace/release snapshot limit.")
	}
	out := spaceSnapshot{Entities: []snapshotEntity{}, Roles: []snapshotRole{}, Apps: []snapshotApp{}, Microapps: []snapshotMicroapp{}}
	if e := q.QueryRowContext(ctx, "SELECT revision FROM spaces WHERE id=?", space).Scan(&out.Revision); e != nil {
		return out, e
	}
	var pending *string
	var definition registry.JSONValue
	var captured snapshotEntity
	r, e := q.QueryContext(ctx, "SELECT e.id,e.revision,e.state,e.pending_version_id,v.definition_json FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? ORDER BY e.id", space)
	if e != nil {
		return out, e
	}
	defer r.Close()
	for r.Next() {
		if e = r.Scan(&captured.ID, &captured.Revision, &captured.State, &pending, &definition); e != nil {
			return out, e
		}
		if pending != nil {
			return out, registry.Conflict("Wait for entity publications to finish before capturing a workspace or release.")
		}
		if captured.Definition, e = defOf(definition.Value()); e != nil {
			return out, e
		}
		out.Entities = append(out.Entities, captured)
	}
	if e = r.Err(); e != nil {
		return out, e
	}
	if out.Roles, e = registry.ScanAll[snapshotRole](ctx, q, "SELECT revision,id,COALESCE(app_id,''),code,display_name,state FROM roles WHERE space_id=? AND domain='data' ORDER BY id", space); e != nil {
		return out, e
	}
	apps, e := registry.ScanAll[appRecord](ctx, q, "SELECT "+appRecordColumns+" FROM apps WHERE space_id=? ORDER BY id", space)
	if e != nil {
		return out, e
	}
	for _, v := range apps {
		a, e := accessOf(v.Access)
		if e != nil {
			return out, e
		}
		a.Origin = ""
		out.Apps = append(out.Apps, snapshotApp{v.Revision, v.ID, v.Slug, v.Name, v.State, a})
	}
	if out.Microapps, e = registry.ScanAll[snapshotMicroapp](ctx, q, "SELECT m.revision,m.id,m.app_id,m.slug,m.display_name,COALESCE(m.artifact_ref,''),m.navigation_order,m.state FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE a.space_id=? ORDER BY m.id", space); e != nil {
		return out, e
	}
	encoded, _ := json.Marshal(out)
	if len(encoded) > maxSpaceSnapshotBytes {
		return out, registry.Conflict("Space configuration exceeds the workspace/release snapshot limit.")
	}
	return out, nil
}

func (s *Server) workspaceRoutes() {
	s.api("GET /v1/spaces/{space}/workspaces", s.workspaceList)
	s.api("POST /v1/spaces/{space}/workspaces", s.workspaceCreate)
	s.api("GET /v1/workspaces/{workspace}", s.workspaceGet)
	s.api("POST /v1/workspaces/{workspace}/archive", s.workspaceArchive)
	s.api("POST /v1/workspaces/{workspace}/retry", s.workspaceRetry)
	s.api("POST /v1/apps/{app}/development-launch", s.developmentLaunch)
}

// Workspace is a development workspace. Read with its backing storage
// (workspaceStorage), it also carries the database resource and its state;
// the workspace view adds its item mappings.
type Workspace struct {
	BackingSpace     string              `json:"backing_space_id"`
	BaseHash         string              `json:"base_hash"`
	CreatedAt        time.Time           `json:"created_at"`
	DatabaseResource *string             `json:"database_resource_id,omitempty"`
	DatabaseState    *string             `json:"database_state,omitempty"`
	Name             string              `json:"display_name"`
	Error            *string             `json:"error_message"`
	ID               string              `json:"id"`
	Mappings         *[]workspaceMapping `json:"mappings,omitempty"`
	OwnerGrant       *string             `json:"owner_agent_grant_id"`
	Owner            string              `json:"owner_user_id"`
	ProductionSpace  string              `json:"production_space_id"`
	Revision         int64               `json:"revision"`
	State            string              `json:"state"`
	TenantID         string              `json:"tenant_id"`
}

// workspaceColumns selects a Workspace from development_workspaces w.
const workspaceColumns = "w.backing_space_id,w.base_hash,w.created_at,w.display_name,w.error_message,w.id,w.owner_agent_grant_id,w.owner_user_id,w.production_space_id,w.revision,w.state,w.tenant_id"

func (v *Workspace) ScanFields() []any {
	return []any{&v.BackingSpace, &v.BaseHash, &v.CreatedAt, &v.Name, &v.Error, &v.ID, &v.OwnerGrant, &v.Owner, &v.ProductionSpace, &v.Revision, &v.State, &v.TenantID}
}

// workspaceStorage selects a Workspace with its backing database resource
// and that resource's state; the query continues with a WHERE clause.
const workspaceStorage = "SELECT " + workspaceColumns + ",sp.database_resource_id,d.state FROM development_workspaces w JOIN spaces sp ON sp.id=w.backing_space_id JOIN database_resources d ON d.id=sp.database_resource_id"

func scanWorkspaceStorage(ctx context.Context, q registry.Queryer, where string, args ...any) (Workspace, error) {
	var v Workspace
	return v, registry.Lookup(ctx, q, workspaceStorage+" "+where, args, append(v.ScanFields(), &v.DatabaseResource, &v.DatabaseState)...)
}

// WorkspaceItem is a workspace in a Space's collection.
type WorkspaceItem struct {
	BackingSpace    string    `json:"backing_space_id"`
	CreatedAt       time.Time `json:"created_at"`
	Name            string    `json:"display_name"`
	Error           *string   `json:"error_message"`
	ID              string    `json:"id"`
	OwnerGrant      *string   `json:"owner_agent_grant_id"`
	ProductionSpace string    `json:"production_space_id"`
	Revision        int64     `json:"revision"`
	State           string    `json:"state"`
}

func (v *WorkspaceItem) ScanFields() []any {
	return []any{&v.BackingSpace, &v.CreatedAt, &v.Name, &v.Error, &v.ID, &v.OwnerGrant, &v.ProductionSpace, &v.Revision, &v.State}
}

// workspaceMapping maps a development record to its production counterpart.
type workspaceMapping struct {
	Development string `json:"development_id"`
	Kind        string `json:"kind"`
	Production  string `json:"production_id"`
}

func (v *workspaceMapping) ScanFields() []any { return []any{&v.Development, &v.Kind, &v.Production} }

// workspaceIDs maps a workspace's development ids to production ids.
func workspaceIDs(ctx context.Context, q registry.Queryer, workspace string) (map[string]string, error) {
	mappings, e := registry.ScanAll[workspaceMapping](ctx, q, "SELECT development_id,kind,production_id FROM workspace_items WHERE workspace_id=?", workspace)
	ids := map[string]string{}
	for _, m := range mappings {
		ids[m.Development] = m.Production
	}
	return ids, e
}

func (s *Server) workspaceRow(ctx context.Context, q registry.Queryer, p Principal, id string) (Workspace, error) {
	v, e := scanWorkspaceStorage(ctx, q, "WHERE w.id=?", id)
	if e != nil {
		return v, e
	}
	return v, s.workspaceOwner(ctx, q, p, v)
}

func (s *Server) workspaceOwner(ctx context.Context, q registry.Queryer, p Principal, v Workspace) error {
	if v.Owner != p.ID {
		return registry.Missing()
	}
	if p.Agent != nil && deref(v.OwnerGrant) != p.Agent.ID {
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
		original, e := s.readAgentGrant(ctx, q, p.Agent.ID)
		if e != nil {
			return e
		}
		if original.Revision != p.Agent.Revision {
			return registry.Missing()
		}
		p.Agent = original
	}
	return s.authorize(ctx, q, p, "workspace.manage", Resource{Tenant: v.TenantID, Space: v.ProductionSpace}, true)
}

func (s *Server) workspaceGuard(ctx context.Context, q registry.Queryer, p Principal, space string) error {
	if space == "" {
		return nil
	}
	v, e := registry.ScanOne[Workspace](ctx, q, "SELECT "+workspaceColumns+" FROM development_workspaces w WHERE w.backing_space_id=?", space)
	if e != nil {
		if ae, ok := e.(*registry.APIError); ok && ae.Status == 404 {
			return nil
		}
		return e
	}
	if e = s.workspaceOwner(ctx, q, p, v); e != nil {
		return e
	}
	if v.State != "ready" && ctx.Value(workspaceOperationKey{}) != v.ID {
		return registry.Conflict("Workspace is not ready. Inspect its status or retry preparation.")
	}
	return nil
}

func (s *Server) Workspaces(ctx context.Context, actor Actor, space string, page PageRequest) (Page[WorkspaceItem], error) {
	p := actor.Principal
	var tenant string
	var parent *string
	if e := registry.Lookup(ctx, s.DB, "SELECT tenant_id,workspace_id FROM spaces WHERE id=?", []any{space}, &tenant, &parent); e != nil {
		return Page[WorkspaceItem]{}, e
	}
	if parent != nil {
		return Page[WorkspaceItem]{}, registry.Bad("Select the production Space.")
	}
	if e := s.authorize(ctx, s.DB, p, "workspace.manage", Resource{Tenant: tenant, Space: space}, true); e != nil {
		return Page[WorkspaceItem]{}, e
	}
	query := "SELECT backing_space_id,created_at,display_name,error_message,id,owner_agent_grant_id,production_space_id,revision,state FROM development_workspaces WHERE production_space_id=? AND owner_user_id=?"
	args := []any{space, p.ID}
	if p.Agent != nil {
		query += " AND owner_agent_grant_id=?"
		args = append(args, p.Agent.ID)
	}
	return pageScan[WorkspaceItem](ctx, s.DB, page, collectionFor("workspaces"), query+" ORDER BY created_at DESC,id", args...)
}

func (s *Server) Workspace(ctx context.Context, actor Actor, workspace string) (Workspace, error) {
	p := actor.Principal
	v, e := s.workspaceRow(ctx, s.DB, p, workspace)
	if e != nil {
		return v, e
	}
	mappings, e := registry.ScanAll[workspaceMapping](ctx, s.DB, "SELECT development_id,kind,production_id FROM workspace_items WHERE workspace_id=? ORDER BY kind,development_id", v.ID)
	v.Mappings = &mappings
	return v, e
}

func (s *Server) CreateWorkspace(ctx context.Context, actor Actor, space string, in workspaceCreateInput) (Workspace, error) {
	p := actor.Principal
	if in.ID == "" {
		in.ID = actor.operationID()
	}
	if e := registry.ID(in.ID); e != nil {
		return Workspace{}, e
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 160 {
		return Workspace{}, registry.Bad("Enter a workspace name (maximum 160 bytes).")
	}
	res := Resource{Space: space}
	var parent *string
	if e := registry.Lookup(ctx, s.DB, "SELECT tenant_id,workspace_id FROM spaces WHERE id=? AND state='active'", []any{space}, &res.Tenant, &parent); e != nil {
		return Workspace{}, e
	}
	if parent != nil {
		return Workspace{}, registry.Bad("Workspaces cannot contain workspaces.")
	}
	e := s.run.mutate(ctx, actor, "workspace.manage", res, func(tx *sql.Tx) error {
		var owned, total int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(owner_user_id=?),0) FROM development_workspaces WHERE production_space_id=? AND state<>'archived'", p.ID, res.Space).Scan(&total, &owned); e != nil {
			return e
		}
		if owned >= maxOwnedWorkspaces || total >= maxSpaceWorkspaces {
			return registry.Conflict("Archive an unused workspace before creating another.")
		}
		if e := s.ensureSnapshotReady(ctx, tx, res.Space); e != nil {
			return e
		}
		base, e := s.snapshot(ctx, tx, res.Space)
		if e != nil {
			return e
		}
		b, _ := json.Marshal(base)
		var layout, target string
		if e = registry.Lookup(ctx, tx, "SELECT sp.database_layout,d.target_id FROM spaces sp JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.id=?", []any{res.Space}, &layout, &target); e != nil {
			return e
		}
		if s.Config.WorkspaceDatabaseTarget != "" {
			target = s.Config.WorkspaceDatabaseTarget
		}
		policy, e := policyFor(ctx, tx, res.Tenant)
		if e != nil {
			return e
		}
		backing, resource := registry.NewID(), registry.NewID()
		if e = s.storage.allocate(ctx, tx, actor, res.Tenant, databaseInput{ID: resource, Mode: "managed", Layout: layout, Target: target}, policy, &res); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO spaces(id,tenant_id,slug,display_name,database_resource_id,database_layout) VALUES(?,?,?,?,?,?)", backing, res.Tenant, "dev-"+strings.ReplaceAll(in.ID, "-", ""), in.Name, resource, layout); e != nil {
			return e
		}
		var grant any
		if p.Agent != nil {
			grant = p.Agent.ID
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO development_workspaces(id,tenant_id,production_space_id,backing_space_id,owner_user_id,owner_agent_grant_id,display_name,base_hash,base_json) VALUES(?,?,?,?,?,?,?,?,?)", in.ID, res.Tenant, res.Space, backing, p.ID, grant, in.Name, entity.Hash(base), b); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE spaces SET workspace_id=? WHERE id=?", in.ID, backing); e != nil {
			return e
		}
		if e = assignInitialManager(ctx, tx, actor, res.Tenant, backing, "", p.ID); e != nil {
			return e
		}
		if e = s.installSnapshot(ctx, tx, base, res.Tenant, backing, in.ID, p.ID); e != nil {
			return e
		}
		return auditAs(ctx, tx, actor, res.Tenant, "workspace.created", "workspace", in.ID, registry.Row{"production_space_id": res.Space, "backing_space_id": backing})
	})
	if e != nil {
		return Workspace{}, e
	}
	return s.Workspace(ctx, actor, in.ID)
}

// entityVersionRef names an entity and one of its versions.
type entityVersionRef struct{ Entity, Version string }

func (v *entityVersionRef) ScanFields() []any { return []any{&v.Entity, &v.Version} }

func (s *Server) changeWorkspace(ctx context.Context, actor Actor, workspace string, retry bool, revision int64) (Workspace, error) {
	p := actor.Principal
	v, e := s.workspaceRow(ctx, s.DB, p, workspace)
	if e != nil {
		return Workspace{}, e
	}
	rev := revision
	if rev < 1 {
		return Workspace{}, revisionRequired()
	}
	e = s.run.mutate(ctx, actor, "workspace.manage", Resource{Tenant: v.TenantID, Space: v.ProductionSpace}, func(tx *sql.Tx) error {
		v, e = s.workspaceRow(ctx, tx, p, workspace)
		if e != nil {
			return e
		}
		if e = checkRevision(v.Revision, rev); e != nil {
			return e
		}
		state := registry.StateArchived
		if retry {
			if v.State != "failed" {
				return registry.Conflict("Only failed preparation can be retried.")
			}
			state = "preparing"
			failed, e := registry.ScanAll[entityVersionRef](ctx, tx, "SELECT e.id,v.id FROM entity_definitions e JOIN entity_versions v ON v.entity_id=e.id AND v.state='failed' WHERE e.space_id=? AND e.active_version_id IS NULL AND e.pending_version_id IS NULL", v.BackingSpace)
			if e != nil {
				return e
			}
			for _, def := range failed {
				if _, e = tx.ExecContext(ctx, "UPDATE entity_versions SET state='queued',attempt_id=NULL,error_message=NULL,finished_at=NULL WHERE id=?", def.Version); e != nil {
					return e
				}
				if _, e = tx.ExecContext(ctx, "UPDATE entity_definitions SET pending_version_id=?,revision=revision+1 WHERE id=?", def.Version, def.Entity); e != nil {
					return e
				}
			}

			if _, e = tx.ExecContext(ctx, "UPDATE database_resources SET state='pending',revision=revision+1 WHERE id=? AND state='failed'", v.DatabaseResource); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(ctx, "UPDATE development_workspaces SET state=?,error_message=NULL,revision=revision+1 WHERE id=?", state, v.ID); e != nil {
			return e
		}
		if !retry {
			if _, e = tx.ExecContext(ctx, "UPDATE spaces SET state='archived',revision=revision+1 WHERE id=?", v.BackingSpace); e != nil {
				return e
			}
		}
		return auditAs(ctx, tx, actor, v.TenantID, "workspace."+state, "workspace", v.ID, registry.Row{})
	})
	if e != nil {
		return Workspace{}, e
	}
	return s.Workspace(ctx, actor, workspace)
}

func lockTenant(ctx context.Context, tx *sql.Tx, tenant string) error {
	var v string
	return tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE id=? FOR UPDATE", tenant).Scan(&v)
}

func (s *Server) productionMutationGuard(ctx context.Context, q registry.Queryer, p Principal, action string, res Resource) error {
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
		if err == nil && ctx.Value(deploymentOperationKey{}) != job {
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
		return requireAgent(p.Agent, "production.deploy")
	}
	return nil
}

func (s *Server) workspaceList(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.Workspaces(r.Context(), httpActor(r, p), r.PathValue("space"), pageRequest(r.URL.Query()))
	return reply(w, 200, v, e)
}

func (s *Server) workspaceGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.Workspace(r.Context(), httpActor(r, p), r.PathValue("workspace"))
	return reply(w, 200, v, e)
}

func (s *Server) workspaceCreate(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in workspaceCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.CreateWorkspace(r.Context(), httpActor(r, p), r.PathValue("space"), in)
	return reply(w, 200, v, e)
}

func (s *Server) workspaceArchive(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.changeWorkspace(r.Context(), httpActor(r, p), r.PathValue("workspace"), false, ifMatch(r))
	return reply(w, 200, v, e)
}

func (s *Server) workspaceRetry(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.changeWorkspace(r.Context(), httpActor(r, p), r.PathValue("workspace"), true, ifMatch(r))
	return reply(w, 200, v, e)
}
