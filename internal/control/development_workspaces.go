package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aantich/lastapp/internal/entity"
)

const (
	maxOwnedWorkspaces = 5
	maxSpaceWorkspaces = 20
)

type (
	workspaceAllocationKey struct{}
	workspaceOperationKey  struct{}
	workspaceCreateInput   struct {
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

func (s *Server) snapshot(ctx context.Context, q Queryer, space string) (spaceSnapshot, error) {
	var definitionBytes int64
	if e := q.QueryRowContext(ctx, "SELECT COALESCE(SUM(OCTET_LENGTH(v.definition_json)),0) FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=?", space).Scan(&definitionBytes); e != nil {
		return spaceSnapshot{}, e
	}
	if definitionBytes > maxSpaceSnapshotBytes {
		return spaceSnapshot{}, conflict("Space configuration exceeds the workspace/release snapshot limit.")
	}
	out := spaceSnapshot{Entities: []snapshotEntity{}, Roles: []snapshotRole{}, Apps: []snapshotApp{}, Microapps: []snapshotMicroapp{}}
	if e := q.QueryRowContext(ctx, "SELECT revision FROM spaces WHERE id=?", space).Scan(&out.Revision); e != nil {
		return out, e
	}
	vs, e := rows(ctx, q, "SELECT e.id,e.revision,e.state,e.pending_version_id,v.definition_json FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? ORDER BY e.id", space)
	if e != nil {
		return out, e
	}
	for _, v := range vs {
		if v["pending_version_id"] != nil {
			return out, conflict("Wait for entity publications to finish before capturing a workspace or release.")
		}
		d, e := defOf(v["definition_json"])
		if e != nil {
			return out, e
		}
		out.Entities = append(out.Entities, snapshotEntity{integer(v["revision"]), str(v["id"]), str(v["state"]), d})
	}
	vs, e = rows(ctx, q, "SELECT id,revision,app_id,code,display_name,state FROM roles WHERE space_id=? AND domain='data' ORDER BY id", space)
	if e != nil {
		return out, e
	}
	for _, v := range vs {
		out.Roles = append(out.Roles, snapshotRole{integer(v["revision"]), str(v["id"]), str(v["app_id"]), str(v["code"]), str(v["display_name"]), str(v["state"])})
	}
	vs, e = rows(ctx, q, "SELECT * FROM apps WHERE space_id=? ORDER BY id", space)
	if e != nil {
		return out, e
	}
	for _, v := range vs {
		a, e := accessOf(v)
		if e != nil {
			return out, e
		}
		a.Origin = ""
		out.Apps = append(out.Apps, snapshotApp{integer(v["revision"]), str(v["id"]), str(v["slug"]), str(v["display_name"]), str(v["state"]), a})
	}
	vs, e = rows(ctx, q, "SELECT m.* FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE a.space_id=? ORDER BY m.id", space)
	if e != nil {
		return out, e
	}
	for _, v := range vs {
		out.Microapps = append(out.Microapps, snapshotMicroapp{integer(v["revision"]), str(v["id"]), str(v["app_id"]), str(v["slug"]), str(v["display_name"]), str(v["artifact_ref"]), integer(v["navigation_order"]), str(v["state"])})
	}
	encoded, _ := json.Marshal(out)
	if len(encoded) > maxSpaceSnapshotBytes {
		return out, conflict("Space configuration exceeds the workspace/release snapshot limit.")
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

func (s *Server) workspaceRow(ctx context.Context, q Queryer, p Principal, id string) (Row, error) {
	v, e := row(ctx, q, "SELECT w.*,sp.database_resource_id,d.state AS database_state FROM development_workspaces w JOIN spaces sp ON sp.id=w.backing_space_id JOIN database_resources d ON d.id=sp.database_resource_id WHERE w.id=?", id)
	if e != nil {
		return nil, e
	}
	if e = s.workspaceOwner(ctx, q, p, v); e != nil {
		return nil, e
	}
	return v, nil
}

func (s *Server) workspaceOwner(ctx context.Context, q Queryer, p Principal, v Row) error {
	if str(v["owner_user_id"]) != p.ID {
		return missing()
	}
	if p.Agent != nil && str(v["owner_agent_grant_id"]) != p.Agent.ID {
		return missing()
	}
	var active string
	if e := q.QueryRowContext(ctx, "SELECT state FROM spaces WHERE id=?", v["production_space_id"]).Scan(&active); e != nil {
		return e
	}
	if active != "active" {
		return conflict("The production Space is inactive.")
	}
	// Losing authority on the source also stops access to its development copies.
	if p.Agent != nil {
		original, e := s.readAgentGrant(ctx, q, p.Agent.ID)
		if e != nil {
			return e
		}
		if original.Revision != p.Agent.Revision {
			return missing()
		}
		p.Agent = original
	}
	return s.authorize(ctx, q, p, "workspace.manage", Resource{Tenant: str(v["tenant_id"]), Space: str(v["production_space_id"])}, true)
}

func (s *Server) workspaceGuard(ctx context.Context, q Queryer, p Principal, space string) error {
	if space == "" {
		return nil
	}
	v, e := row(ctx, q, "SELECT w.* FROM development_workspaces w WHERE w.backing_space_id=?", space)
	if e != nil {
		if ae, ok := e.(*APIError); ok && ae.Status == 404 {
			return nil
		}
		return e
	}
	if e = s.workspaceOwner(ctx, q, p, v); e != nil {
		return e
	}
	if str(v["state"]) != "ready" && ctx.Value(workspaceOperationKey{}) != str(v["id"]) {
		return conflict("Workspace is not ready. Inspect its status or retry preparation.")
	}
	return nil
}

func (s *Server) workspaceList(w http.ResponseWriter, r *http.Request, p Principal) error {
	sp, e := row(r.Context(), s.DB, "SELECT tenant_id,workspace_id FROM spaces WHERE id=?", r.PathValue("space"))
	if e != nil {
		return e
	}
	if sp["workspace_id"] != nil {
		return bad("Select the production Space.")
	}
	if e = s.authorize(r.Context(), s.DB, p, "workspace.manage", Resource{Tenant: str(sp["tenant_id"]), Space: r.PathValue("space")}, true); e != nil {
		return e
	}
	query := "SELECT id,display_name,production_space_id,backing_space_id,state,error_message,revision,created_at,owner_agent_grant_id FROM development_workspaces WHERE production_space_id=? AND owner_user_id=?"
	args := []any{r.PathValue("space"), p.ID}
	if p.Agent != nil {
		query += " AND owner_agent_grant_id=?"
		args = append(args, p.Agent.ID)
	}
	vs, e := pageRows(r.Context(), s.DB, r, query+" ORDER BY created_at DESC,id", args...)
	if e == nil {
		sendPage(w, r, vs)
	}
	return e
}

func (s *Server) workspaceGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.workspaceRow(r.Context(), s.DB, p, r.PathValue("workspace"))
	if e != nil {
		return e
	}
	delete(v, "base_json")
	mappings, e := rows(r.Context(), s.DB, "SELECT kind,development_id,production_id FROM workspace_items WHERE workspace_id=? ORDER BY kind,development_id", v["id"])
	if e != nil {
		return e
	}
	v["mappings"] = mappings
	send(w, 200, v)
	return nil
}

func (s *Server) workspaceCreate(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in workspaceCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if in.ID == "" {
		in.ID = operationID(r.Context())
	}
	if e := id(in.ID); e != nil {
		return e
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 160 {
		return bad("Enter a workspace name (maximum 160 bytes).")
	}
	sp, e := row(r.Context(), s.DB, "SELECT tenant_id,workspace_id FROM spaces WHERE id=? AND state='active'", r.PathValue("space"))
	if e != nil {
		return e
	}
	if sp["workspace_id"] != nil {
		return bad("Workspaces cannot contain workspaces.")
	}
	res := Resource{Tenant: str(sp["tenant_id"]), Space: r.PathValue("space")}
	e = s.mutate(r.Context(), p, "workspace.manage", res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var owned, total int
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*),COALESCE(SUM(owner_user_id=?),0) FROM development_workspaces WHERE production_space_id=? AND state<>'archived'", p.ID, res.Space).Scan(&total, &owned); e != nil {
			return e
		}
		if owned >= maxOwnedWorkspaces || total >= maxSpaceWorkspaces {
			return conflict("Archive an unused workspace before creating another.")
		}
		if e := s.ensureSnapshotReady(r.Context(), tx, res.Space); e != nil {
			return e
		}
		base, e := s.snapshot(r.Context(), tx, res.Space)
		if e != nil {
			return e
		}
		b, _ := json.Marshal(base)
		source, e := row(r.Context(), tx, "SELECT sp.database_layout,d.target_id FROM spaces sp JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.id=?", res.Space)
		if e != nil {
			return e
		}
		if s.Config.WorkspaceDatabaseTarget != "" {
			source["target_id"] = s.Config.WorkspaceDatabaseTarget
		}
		policy, e := policyFor(r.Context(), tx, res.Tenant)
		if e != nil {
			return e
		}
		backing, resource := newID(), newID()
		ctx := context.WithValue(r.Context(), workspaceAllocationKey{}, res)
		if e = s.allocateDatabase(ctx, tx, p, res.Tenant, databaseInput{ID: resource, Mode: "managed", Layout: str(source["database_layout"]), Target: str(source["target_id"])}, policy, r.Header.Get("X-Request-ID")); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO spaces(id,tenant_id,slug,display_name,database_resource_id,database_layout) VALUES(?,?,?,?,?,?)", backing, res.Tenant, "dev-"+strings.ReplaceAll(in.ID, "-", ""), in.Name, resource, source["database_layout"]); e != nil {
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
		if e = assignInitialManager(ctx, tx, p, res.Tenant, backing, "", p.ID, r.Header.Get("X-Request-ID")); e != nil {
			return e
		}
		if e = s.installSnapshot(ctx, tx, base, res.Tenant, backing, in.ID, p.ID); e != nil {
			return e
		}
		return audit(ctx, tx, res.Tenant, p.ID, "workspace.created", "workspace", in.ID, r.Header.Get("X-Request-ID"), Row{"production_space_id": res.Space, "backing_space_id": backing})
	})
	if e != nil {
		return e
	}
	r.SetPathValue("workspace", in.ID)
	return s.workspaceGet(w, r, p)
}

func (s *Server) workspaceArchive(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.workspaceState(w, r, p, false)
}

func (s *Server) workspaceRetry(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.workspaceState(w, r, p, true)
}

func (s *Server) workspaceState(w http.ResponseWriter, r *http.Request, p Principal, retry bool) error {
	v, e := s.workspaceRow(r.Context(), s.DB, p, r.PathValue("workspace"))
	if e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	e = s.mutate(r.Context(), p, "workspace.manage", Resource{Tenant: str(v["tenant_id"]), Space: str(v["production_space_id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		v, e = s.workspaceRow(r.Context(), tx, p, r.PathValue("workspace"))
		if e != nil {
			return e
		}
		if e = checkRevision(integer(v["revision"]), rev); e != nil {
			return e
		}
		state := "archived"
		if retry {
			if v["state"] != "failed" {
				return conflict("Only failed preparation can be retried.")
			}
			state = "preparing"
			failed, e := rows(r.Context(), tx, "SELECT e.id,v.id AS version_id FROM entity_definitions e JOIN entity_versions v ON v.entity_id=e.id AND v.state='failed' WHERE e.space_id=? AND e.active_version_id IS NULL AND e.pending_version_id IS NULL", v["backing_space_id"])
			if e != nil {
				return e
			}
			for _, def := range failed {
				if _, e = tx.ExecContext(r.Context(), "UPDATE entity_versions SET state='queued',attempt_id=NULL,error_message=NULL,finished_at=NULL WHERE id=?", def["version_id"]); e != nil {
					return e
				}
				if _, e = tx.ExecContext(r.Context(), "UPDATE entity_definitions SET pending_version_id=?,revision=revision+1 WHERE id=?", def["version_id"], def["id"]); e != nil {
					return e
				}
			}

			if _, e = tx.ExecContext(r.Context(), "UPDATE database_resources SET state='pending',revision=revision+1 WHERE id=? AND state='failed'", v["database_resource_id"]); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(r.Context(), "UPDATE development_workspaces SET state=?,error_message=NULL,revision=revision+1 WHERE id=?", state, v["id"]); e != nil {
			return e
		}
		if !retry {
			if _, e = tx.ExecContext(r.Context(), "UPDATE spaces SET state='archived',revision=revision+1 WHERE id=?", v["backing_space_id"]); e != nil {
				return e
			}
		}
		return audit(r.Context(), tx, str(v["tenant_id"]), p.ID, "workspace."+state, "workspace", str(v["id"]), r.Header.Get("X-Request-ID"), Row{})
	})
	if e != nil {
		return e
	}
	return s.workspaceGet(w, r, p)
}

func lockTenant(ctx context.Context, tx *sql.Tx, tenant string) error {
	var v string
	return tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE id=? FOR UPDATE", tenant).Scan(&v)
}

func (s *Server) productionMutationGuard(ctx context.Context, q Queryer, p Principal, action string, res Resource) error {
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
			return conflict("A deployment is preparing this Space. Wait for its result before changing configuration.")
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
