package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/aantich/lastapp/internal/entity"
)

type releaseManifest struct {
	Microapps    map[string]string `json:"microapps"`
	Dependencies map[string]string `json:"dependencies"`
}
type releaseCreateInput struct {
	Name     string          `json:"display_name"`
	Build    string          `json:"build_id"`
	Manifest releaseManifest `json:"manifest"`
}
type deploymentOperationKey struct{}

// One public status projection for discovery, polling and MCP. Never expose the
// persisted plan or delegation details through a status response.
const deploymentStatusColumns = "d.id,d.tenant_id,d.space_id,d.release_id,d.target_app_id,d.state,d.error_message,d.revision,d.created_at,d.finished_at"

type deploymentPlan struct {
	Snapshot spaceSnapshot     `json:"snapshot"`
	BaseHash string            `json:"base_hash"`
	Versions map[string]string `json:"versions"`
}

var (
	pinnedNameRE    = regexp.MustCompile(`^[a-zA-Z0-9@][a-zA-Z0-9._/@-]{0,159}$`)
	pinnedVersionRE = regexp.MustCompile(`^(?:[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?|sha256:[a-f0-9]{64})$`)
)

func (s *Server) releaseCreate(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in releaseCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 160 || id(in.Build) != nil {
		return bad("Provide a release name and uploaded build ID.")
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	a, ws, e := s.ownedDevelopmentApp(r.Context(), s.DB, p, r.PathValue("app"))
	if e != nil {
		return e
	}
	record := operationID(r.Context())
	e = s.mutate(r.Context(), p, "release.manage", Resource{Tenant: str(a["tenant_id"]), App: str(a["id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		a, e = s.appRow(r.Context(), tx, str(a["id"]))
		if e != nil {
			return e
		}
		if e = checkRevision(integer(a["revision"]), rev); e != nil {
			return e
		}
		if _, e = row(r.Context(), tx, "SELECT id FROM app_builds WHERE id=? AND app_id=?", in.Build, a["id"]); e != nil {
			return e
		}
		var count int
		if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM app_releases WHERE app_id=?", a["id"]).Scan(&count); e != nil {
			return e
		}
		if count >= maxAppReleases {
			return conflict("Release limit reached. Start a fresh workspace.")
		}
		if e = s.ensureSnapshotReady(r.Context(), tx, str(a["space_id"])); e != nil {
			return e
		}
		base, e := s.snapshot(r.Context(), tx, str(a["space_id"]))
		if e != nil {
			return e
		}
		if len(in.Manifest.Microapps) > maxReleaseMicroapps || len(in.Manifest.Dependencies) > maxReleaseDependencies {
			return bad("Release manifest is too large.")
		}
		needed := map[string]bool{}
		for _, v := range base.Microapps {
			if v.App == str(a["id"]) && v.State == stateActive {
				needed[v.Slug] = true
				if !pinnedVersionRE.MatchString(in.Manifest.Microapps[v.Slug]) {
					return bad("Pin a version for each active micro-app in this App.")
				}
			}
		}
		for key := range in.Manifest.Microapps {
			if !needed[key] {
				return bad("Manifest names a micro-app outside this App.")
			}
		}
		for name, version := range in.Manifest.Dependencies {
			if len(name) > 160 || !pinnedNameRE.MatchString(name) || !pinnedVersionRE.MatchString(version) {
				return bad("Dependencies require bounded package names and exact versions (no ranges or latest).")
			}
		}
		// Complete mappings are allocated once and survive later release captures.
		ids := map[string]string{}
		maps, e := rows(r.Context(), tx, "SELECT development_id,production_id FROM workspace_items WHERE workspace_id=?", ws["id"])
		if e != nil {
			return e
		}
		for _, m := range maps {
			ids[str(m["development_id"])] = str(m["production_id"])
		}
		for key, kind := range snapshotIDs(base) {
			if ids[key] == "" {
				ids[key] = newID()
				if _, e = tx.ExecContext(r.Context(), "INSERT INTO workspace_items(workspace_id,kind,development_id,production_id) VALUES(?,?,?,?)", ws["id"], kind, key, ids[key]); e != nil {
					return e
				}
			}
		}
		production := remapSnapshot(base, ids)
		snapshotJSON, _ := json.Marshal(production)
		manifestJSON, _ := json.Marshal(in.Manifest)
		if _, e = tx.ExecContext(r.Context(), "INSERT INTO app_releases(id,tenant_id,app_id,build_id,workspace_id,display_name,manifest_json,snapshot_json,snapshot_hash,created_by,base_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?)", record, a["tenant_id"], a["id"], in.Build, ws["id"], in.Name, manifestJSON, snapshotJSON, snapshotContractHash(production), p.ID, ws["base_hash"]); e != nil {
			return e
		}
		return audit(r.Context(), tx, str(a["tenant_id"]), p.ID, "release.created", "release", record, r.Header.Get("X-Request-ID"), Row{"build_id": in.Build, "snapshot_hash": snapshotContractHash(production)})
	})
	if e != nil {
		return e
	}
	r.SetPathValue("release", record)
	return s.releaseGet(w, r, p)
}

func (s *Server) releaseList(w http.ResponseWriter, r *http.Request, p Principal) error {
	a, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if e = s.authorize(r.Context(), s.DB, p, "release.manage", Resource{Tenant: str(a["tenant_id"]), App: str(a["id"])}, false); e != nil {
		return e
	}
	vs, e := pageRows(r.Context(), s.DB, r, "SELECT rel.id,rel.display_name,rel.build_id,rel.snapshot_hash,rel.created_at FROM app_releases rel WHERE rel.app_id=? OR EXISTS(SELECT 1 FROM release_deployments d WHERE d.release_id=rel.id AND d.target_app_id=? AND d.state='active') ORDER BY rel.created_at DESC,rel.id", a["id"], a["id"])
	if e == nil {
		sendPage(w, r, vs)
	}
	return e
}

func (s *Server) releaseRow(ctx context.Context, q Queryer, p Principal, id string) (Row, error) {
	v, e := row(ctx, q, "SELECT rel.*,ws.production_space_id,ws.backing_space_id,ws.owner_user_id,ws.owner_agent_grant_id,wi.production_id AS production_app_id FROM app_releases rel JOIN development_workspaces ws ON ws.id=rel.workspace_id JOIN workspace_items wi ON wi.workspace_id=ws.id AND wi.kind='app' AND wi.development_id=rel.app_id WHERE rel.id=?", id)
	if e != nil {
		return nil, e
	}
	if e = s.authorize(ctx, q, p, "release.manage", Resource{Tenant: str(v["tenant_id"]), Space: str(v["production_space_id"])}, false); e != nil {
		return nil, e
	}
	// Undeployed snapshots belong to the private workspace. Successful releases
	// remain inspectable as production history after that workspace is archived.
	var deployed int
	if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM release_deployments WHERE release_id=? AND tenant_id=? AND space_id=? AND target_app_id=? AND state='active'", id, v["tenant_id"], v["production_space_id"], v["production_app_id"]).Scan(&deployed); e != nil {
		return nil, e
	}
	if deployed == 0 {
		if e = s.workspaceGuard(ctx, q, p, str(v["backing_space_id"])); e != nil {
			return nil, e
		}
	}
	return v, nil
}

func (s *Server) releaseGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.releaseRow(r.Context(), s.DB, p, r.PathValue("release"))
	if e != nil {
		return e
	}
	v["preview_url"] = s.appHostedOrigin(str(v["app_id"])) + "/"
	v["production_url"] = s.appHostedOrigin(str(v["production_app_id"])) + "/"
	send(w, 200, v)
	return nil
}

func (s *Server) releasePreview(w http.ResponseWriter, r *http.Request, p Principal) error {
	rel, e := s.releaseRow(r.Context(), s.DB, p, r.PathValue("release"))
	if e != nil {
		return e
	}
	a, _, e := s.ownedDevelopmentApp(r.Context(), s.DB, p, str(rel["app_id"]))
	if e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	origin := s.appHostedOrigin(str(a["id"]))
	if origin == "" {
		return conflict("Configure the App hosting origin first.")
	}
	e = s.mutate(r.Context(), p, "release.manage", Resource{Tenant: str(a["tenant_id"]), App: str(a["id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		a, e = s.appRow(r.Context(), tx, str(a["id"]))
		if e != nil {
			return e
		}
		if e = checkRevision(integer(a["revision"]), rev); e != nil {
			return e
		}
		// The shared Tenant mutation lock also covers entity/role/App changes.
		// Validate the complete snapshot under that lock, not in an earlier read.
		if e = s.ensureSnapshotReady(r.Context(), tx, str(a["space_id"])); e != nil {
			return e
		}
		currentSnapshot, err := s.snapshot(r.Context(), tx, str(a["space_id"]))
		if err != nil {
			return err
		}
		ids := map[string]string{}
		mappings, err := rows(r.Context(), tx, "SELECT development_id,production_id FROM workspace_items WHERE workspace_id=?", rel["workspace_id"])
		if err != nil {
			return err
		}
		for _, m := range mappings {
			ids[str(m["development_id"])] = str(m["production_id"])
		}
		if snapshotContractHash(remapSnapshot(currentSnapshot, ids)) != str(rel["snapshot_hash"]) {
			return conflict("Workspace configuration changed after this release was captured. Capture and test a fresh release.")
		}
		access, e := accessOf(a)
		if e != nil {
			return e
		}
		old := access.Origin
		access.Origin = origin
		b, _ := json.Marshal(access)
		if _, e = tx.ExecContext(r.Context(), "UPDATE apps SET active_release_id=?,data_access_json=?,approved_origin=?,origin_host=NULL,origin_generation=origin_generation+IF(?<>?,1,0),revision=revision+1 WHERE id=?", rel["id"], b, origin, old, origin, a["id"]); e != nil {
			return e
		}
		return audit(r.Context(), tx, str(a["tenant_id"]), p.ID, "release.preview_activated", "app", str(a["id"]), r.Header.Get("X-Request-ID"), Row{"release_id": rel["id"]})
	})
	if e != nil {
		return e
	}
	send(w, 200, Row{"id": rel["id"], "url": origin + "/"})
	return nil
}

func (s *Server) deploymentAuthority(ctx context.Context, q Queryer, p Principal, res Resource) error {
	var active int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM spaces WHERE tenant_id=? AND id=? AND state='active' AND workspace_id IS NULL", res.Tenant, res.Space).Scan(&active); e != nil {
		return e
	}
	if active != 1 {
		return conflict("Select an active production Space for deployment.")
	}
	if p.Agent != nil {
		if e := requireAgent(p.Agent, "production.deploy"); e != nil {
			return e
		}
	}
	for _, action := range []string{"production.deploy", "entity.publish", "app.manage", "authorization.manage"} {
		if e := s.authorize(ctx, q, p, action, res, true); e != nil {
			return e
		}
	}
	return nil
}

func (s *Server) releaseDeploy(w http.ResponseWriter, r *http.Request, p Principal) error {
	rel, e := s.releaseRow(r.Context(), s.DB, p, r.PathValue("release"))
	if e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	res := Resource{Tenant: str(rel["tenant_id"]), Space: str(rel["production_space_id"])}
	if e = s.deploymentAuthority(r.Context(), s.DB, p, res); e != nil {
		return e
	}
	if e = s.ensureReleaseBuild(r.Context(), s.DB, str(rel["id"])); e != nil {
		return e
	}
	record := operationID(r.Context())
	ctx := context.WithValue(r.Context(), deploymentOperationKey{}, record)
	e = s.mutate(ctx, p, "production.deploy", res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		a, e := s.appRow(ctx, tx, str(rel["production_app_id"]))
		if e != nil {
			if ae, ok := e.(*APIError); !ok || ae.Status != 404 {
				return e
			}
			if rev != 1 {
				return conflict("A new production App starts at expected revision 1.")
			}
		} else {
			if e = checkRevision(integer(a["revision"]), rev); e != nil {
				return e
			}
			if str(a["active_release_id"]) == str(rel["id"]) {
				return conflict("This release is already active in production.")
			}
		}
		if e = s.ensureSnapshotReady(ctx, tx, res.Space); e != nil {
			return e
		}
		current, e := s.snapshot(ctx, tx, res.Space)
		if e != nil {
			return e
		}
		if entity.Hash(current) != str(rel["base_hash"]) {
			return conflict("Production changed since this workspace was created. Create a fresh workspace and reconcile its configuration before deploying.")
		}
		var snap spaceSnapshot
		b, _ := json.Marshal(rel["snapshot_json"])
		if json.Unmarshal(b, &snap) != nil || snapshotContractHash(snap) != str(rel["snapshot_hash"]) {
			return conflict("Release snapshot is invalid.")
		}
		if s.appHostedOrigin(str(rel["production_app_id"])) == "" {
			return conflict("Configure the App hosting origin before deployment.")
		}
		if e = s.snapshotAppQuota(ctx, tx, res.Tenant, snap); e != nil {
			return e
		}
		// The FK target App exists before the durable deployment record is inserted.
		for _, app := range snap.Apps {
			if app.ID == str(rel["production_app_id"]) {
				if _, e = tx.ExecContext(ctx, "INSERT IGNORE INTO apps(id,tenant_id,space_id,slug,display_name,compute_mode) VALUES(?,?,?,?,?,'shared')", app.ID, res.Tenant, res.Space, app.Slug, app.Name); e != nil {
					return e
				}
			}
		}
		plan := deploymentPlan{Snapshot: snap, BaseHash: str(rel["base_hash"]), Versions: map[string]string{}}
		for _, d := range snap.Entities {
			plan.Versions[d.ID] = newID()
		}
		pb, _ := json.Marshal(plan)
		var grantRev any
		if p.Agent != nil {
			grantRev = p.Agent.Revision
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO release_deployments(id,tenant_id,space_id,release_id,target_app_id,created_by,agent_grant_id,agent_grant_revision,plan_json) VALUES(?,?,?,?,?,?,?,?,?)", record, res.Tenant, res.Space, rel["id"], rel["production_app_id"], p.ID, agentID(p), grantRev, pb); e != nil {
			return e
		}
		return audit(ctx, tx, res.Tenant, p.ID, "release.deployment_queued", "deployment", record, r.Header.Get("X-Request-ID"), Row{"release_id": rel["id"], "snapshot_hash": rel["snapshot_hash"]})
	})
	if e != nil {
		return e
	}
	send(w, 202, Row{"id": record, "state": "queued", "instruction": "Poll deployment status. Storage preparation can pause changed entity APIs; registry security and client selection activate together. Never assume queued means deployed."})
	return nil
}

func (s *Server) deploymentGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := row(r.Context(), s.DB, "SELECT "+deploymentStatusColumns+" FROM release_deployments d WHERE d.id=?", r.PathValue("deployment"))
	if e != nil {
		return e
	}
	if e = s.authorize(r.Context(), s.DB, p, "release.manage", Resource{Tenant: str(v["tenant_id"]), Space: str(v["space_id"])}, false); e != nil {
		return e
	}
	v["url"] = s.appHostedOrigin(str(v["target_app_id"])) + "/"
	send(w, 200, v)
	return nil
}

// Recover the durable job after a browser reload or a lost deploy response.
// A pending sibling deployment also explains a shared Space's mutation fence.
func (s *Server) appDeployment(w http.ResponseWriter, r *http.Request, p Principal) error {
	a, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if e = s.authorize(r.Context(), s.DB, p, "release.manage", Resource{Tenant: str(a["tenant_id"]), App: str(a["id"])}, false); e != nil {
		return e
	}
	space := str(a["space_id"])
	var production string
	e = s.DB.QueryRowContext(r.Context(), "SELECT production_space_id FROM development_workspaces WHERE backing_space_id=?", space).Scan(&production)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if production != "" {
		space = production
	}
	vs, e := rows(r.Context(), s.DB, `SELECT `+deploymentStatusColumns+`
 FROM release_deployments d JOIN app_releases rel ON rel.id=d.release_id
 WHERE d.tenant_id=? AND d.space_id=? AND (d.state IN ('queued','preparing','publishing') OR d.target_app_id=? OR rel.app_id=?)
 ORDER BY (d.state IN ('queued','preparing','publishing')) DESC,d.created_at DESC,d.id DESC LIMIT 1`, a["tenant_id"], space, a["id"], a["id"])
	if e != nil {
		return e
	}
	var deployment any
	if len(vs) > 0 {
		vs[0]["url"] = s.appHostedOrigin(str(vs[0]["target_app_id"])) + "/"
		deployment = vs[0]
	}
	send(w, 200, Row{"deployment": deployment})
	return nil
}
