package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/entity"
)

func (s *Server) entityRoutes() {
	s.api("GET /v1/entity-capabilities", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		send(w, 200, entity.Capabilities())
		return nil
	})
	s.api("GET /v1/spaces/{space}/entities", s.entityList)
	s.api("POST /v1/spaces/{space}/entities", s.entityCreate)
	s.api("GET /v1/entities/{entity}", s.entityGet)
	s.api("PATCH /v1/entities/{entity}", s.entitySave)
	s.api("POST /v1/entities/{entity}/validate", s.entityValidate)
	s.api("POST /v1/entities/{entity}/test", s.entityTest)
	s.api("POST /v1/entities/{entity}/publications", s.entityPublish)
	s.api("GET /v1/entities/{entity}/publications/{version}", s.entityPublication)
	s.api("GET /v1/apps/{app}/data-access", s.appAccessGet)
	s.api("PATCH /v1/apps/{app}/data-access", s.appAccessSave)
	s.api("GET /v1/apps/{app}/entity-contracts", s.appContracts)
	s.api("POST /v1/tenants/{tenant}/database-resources/{id}/schema-credentials", s.schemaCredentials)
	s.api("GET /v1/apps/{app}/launch-info", s.appLaunchInfo)
	s.api("POST /v1/apps/{app}/launch", s.appLaunch)
	s.mux.HandleFunc("/data/v1/", s.dataHandler)
}

func (s *Server) entityRow(ctx context.Context, q Queryer, p Principal, key, action string) (Row, error) {
	v, e := row(ctx, q, "SELECT * FROM entity_definitions WHERE id=?", key)
	if e != nil {
		return nil, e
	}
	e = s.authorize(ctx, q, p, action, Resource{Tenant: str(v["tenant_id"]), Space: str(v["space_id"])}, true)
	return v, e
}

func (s *Server) spaceForEntity(ctx context.Context, p Principal, space string) (Row, error) {
	v, e := row(ctx, s.DB, "SELECT * FROM spaces WHERE id=? AND state='active'", space)
	if e != nil {
		return nil, e
	}
	e = s.authorize(ctx, s.DB, p, "entity.manage", Resource{Tenant: str(v["tenant_id"]), Space: space}, true)
	return v, e
}

func defOf(v any) (entity.Definition, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return entity.Definition{}, e
	}
	return entity.Decode(b)
}

func (s *Server) entityList(w http.ResponseWriter, r *http.Request, p Principal) error {
	sp, e := s.spaceForEntity(r.Context(), p, r.PathValue("space"))
	if e != nil {
		return e
	}
	v, e := rows(r.Context(), s.DB, "SELECT id,entity_key,label,state,revision,active_version_id,pending_version_id FROM entity_definitions WHERE space_id=? AND tenant_id=? ORDER BY label,id", sp["id"], sp["tenant_id"])
	if e == nil {
		send(w, 200, Row{"items": v})
	}
	return e
}

func (s *Server) entityCreate(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		ID         string            `json:"id"`
		Definition entity.Definition `json:"definition"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if in.ID == "" {
		in.ID = newID()
	}
	if e := id(in.ID); e != nil {
		return e
	}
	sp, e := s.spaceForEntity(r.Context(), p, r.PathValue("space"))
	if e != nil {
		return e
	}
	if in.Definition.Format != 1 || len(in.Definition.Key) > 48 || !entity.ValidKey(in.Definition.Key) || len(in.Definition.Label) > 160 || in.Definition.Label == "" {
		return bad("Provide an entity key and name.")
	}
	b, _ := json.Marshal(in.Definition)
	if len(b) > entity.MaxDefinitionBytes {
		return bad("Definition exceeds its encoded-size limit.")
	}
	e = s.mutate(r.Context(), p, "entity.manage", Resource{Tenant: str(sp["tenant_id"]), Space: str(sp["id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM entity_definitions WHERE space_id=?", sp["id"]).Scan(&n); e != nil {
			return e
		}
		if n >= 100 {
			return conflict("Space entity limit reached.")
		}
		_, e := tx.ExecContext(r.Context(), "INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,draft_json) VALUES(?,?,?,?,?,?)", in.ID, sp["tenant_id"], sp["id"], in.Definition.Key, in.Definition.Label, b)
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, str(sp["tenant_id"]), p.ID, "entity.created", "entity", in.ID, r.Header.Get("X-Request-ID"), Row{})
	})
	if e != nil {
		return e
	}
	r.SetPathValue("entity", in.ID)
	return s.entityGet(w, r, p)
}

func (s *Server) entityGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.entityRow(r.Context(), s.DB, p, r.PathValue("entity"), "entity.manage")
	if e != nil {
		return e
	}
	if section := r.URL.Query().Get("section"); section != "" {
		version := r.URL.Query().Get("version")
		if version == "" {
			version = "draft"
		}
		definition := v["draft_json"]
		if version == "published" {
			if v["active_version_id"] == nil {
				return missing()
			}
			active, err := row(r.Context(), s.DB, "SELECT definition_json FROM entity_versions WHERE id=?", v["active_version_id"])
			if err != nil {
				return err
			}
			definition = active["definition_json"]
		} else if version != "draft" {
			return bad("version must be draft or published.")
		}
		d, err := defOf(definition)
		if err != nil {
			return err
		}
		value, err := entitySection(d, section, r.URL.Query().Get("field"))
		if err != nil {
			return err
		}
		send(w, 200, Row{"id": v["id"], "space_id": v["space_id"], "revision": v["revision"], "version": version, "active_version_id": v["active_version_id"], "definition_hash": entity.Hash(d), "section": section, "field": r.URL.Query().Get("field"), "partial": r.URL.Query().Get("field") != "", "value": value})
		return nil
	}
	if r.URL.Query().Get("field") != "" || r.URL.Query().Get("version") != "" {
		return bad("field/version require a section.")
	}
	d, e := defOf(v["draft_json"])
	if e != nil {
		return e
	}
	v["diagnostics"] = d.Validate()
	v["draft_contract"] = Row{"definition_hash": entity.Hash(d), "json_schema": d.JSONSchema(), "create_schema": d.Schema("create"), "update_schema": d.Schema("update"), "typescript": d.TypeScript()}
	versions, e := rows(r.Context(), s.DB, "SELECT id,state,error_message,definition_hash,base_version_id,created_at FROM entity_versions WHERE entity_id=? ORDER BY created_at DESC LIMIT 20", v["id"])
	if e != nil {
		return e
	}
	v["publications"] = versions
	if v["active_version_id"] != nil {
		active, e := row(r.Context(), s.DB, "SELECT definition_json FROM entity_versions WHERE id=?", v["active_version_id"])
		if e != nil {
			return e
		}
		v["active_definition"] = active["definition_json"]
		published, e := defOf(active["definition_json"])
		if e != nil {
			return e
		}
		v["published_contract"] = Row{"version_id": v["active_version_id"], "definition_hash": entity.Hash(published), "json_schema": published.JSONSchema(), "create_schema": published.Schema("create"), "update_schema": published.Schema("update"), "typescript": published.TypeScript()}
	}
	send(w, 200, v)
	return nil
}

func (s *Server) entitySave(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		Definition entity.Definition `json:"definition"`
		State      string            `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	old, e := s.entityRow(r.Context(), s.DB, p, r.PathValue("entity"), "entity.manage")
	if e != nil {
		return e
	}
	if in.Definition.Key != str(old["entity_key"]) || in.Definition.Format != 1 || in.Definition.Label == "" || len(in.Definition.Label) > 160 {
		return bad("Entity key/format are immutable and a name is required.")
	}
	b, _ := json.Marshal(in.Definition)
	if len(b) > entity.MaxDefinitionBytes {
		return bad("Definition exceeds its encoded-size limit.")
	}
	e = s.mutate(r.Context(), p, "entity.manage", Resource{Tenant: str(old["tenant_id"]), Space: str(old["space_id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		v, e := row(r.Context(), tx, "SELECT revision,pending_version_id FROM entity_definitions WHERE id=? FOR UPDATE", old["id"])
		if e != nil {
			return e
		}
		if e = checkRevision(integer(v["revision"]), rev); e != nil {
			return e
		}
		state := str(old["state"])
		if in.State != "" {
			if !slices.Contains([]string{stateActive, stateArchived}, in.State) {
				return bad("Invalid lifecycle state.")
			}
			if v["pending_version_id"] != nil {
				return conflict("Wait for publishing before archiving.")
			}
			if in.State == stateArchived {
				refs, e := rows(r.Context(), tx, "SELECT v.definition_json FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? AND e.state='active' AND e.id<>?", old["space_id"], old["id"])
				if e != nil {
					return e
				}
				for _, v := range refs {
					d, e := defOf(v["definition_json"])
					if e != nil {
						return e
					}
					for _, f := range d.Fields {
						if f.Target == old["id"] {
							return conflict("An active entity references this entity.")
						}
					}
				}
			}
			state = in.State
		}
		_, e = tx.ExecContext(r.Context(), "UPDATE entity_definitions SET label=?,draft_json=?,state=?,revision=revision+1 WHERE id=?", in.Definition.Label, b, state, old["id"])
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, str(old["tenant_id"]), p.ID, "entity.saved", "entity", str(old["id"]), r.Header.Get("X-Request-ID"), Row{"state": state})
	})
	if e != nil {
		return e
	}
	return s.entityGet(w, r, p)
}

func (s *Server) validateEntityRefs(ctx context.Context, q Queryer, d entity.Definition, v Row) error {
	return s.validateEntityRefsCandidates(ctx, q, d, v, nil)
}

func (s *Server) validateEntityRefsCandidates(ctx context.Context, q Queryer, d entity.Definition, v Row, candidates map[string]bool) error {
	tenant, space, entityID := str(v["tenant_id"]), str(v["space_id"]), str(v["id"])
	for _, p := range d.Permissions {
		for _, rid := range p.Roles {
			if e := id(rid); e != nil {
				return e
			}
			var n int
			if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM roles WHERE id=? AND tenant_id=? AND space_id=? AND app_id IS NULL AND domain='data' AND (state='active' OR ?=1)", rid, tenant, space, candidates != nil).Scan(&n); e != nil {
				return e
			}
			if n != 1 {
				return bad("A permission references an unavailable Space business role.")
			}
		}
		if p.Subject != "" {
			if e := id(p.Subject); e != nil {
				return e
			}
			var n int
			// Captured batches copy already-published ACLs. A later suspension
			// must keep that reference inert, rather than making copying fail.
			// Live actor resolution still excludes inactive users and teams.
			query := "SELECT COUNT(*) FROM groups WHERE id=? AND tenant_id=? AND (state='active' OR ?=1)"
			if p.Relationship == "user" {
				query = "SELECT COUNT(*) FROM tenant_memberships m JOIN users u ON u.id=m.user_id WHERE m.user_id=? AND m.tenant_id=? AND ((u.state='active' AND m.state='active') OR ?=1)"
			}
			if e := q.QueryRowContext(ctx, query, p.Subject, tenant, candidates != nil).Scan(&n); e != nil {
				return e
			}
			if n != 1 {
				return bad("Unavailable permission recipient.")
			}
		}
	}
	for _, f := range d.Fields {
		if f.Type == "reference" {
			if e := id(f.Target); e != nil {
				return e
			}
		}
		if f.Type == "reference" && f.Target != entityID && !candidates[f.Target] {
			var n int
			if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM entity_definitions WHERE id=? AND tenant_id=? AND space_id=? AND state='active' AND active_version_id IS NOT NULL", f.Target, tenant, space).Scan(&n); e != nil {
				return e
			}
			if n != 1 {
				return bad("Reference target must be a published entity in this Space.")
			}
		}
	}
	return nil
}

func (s *Server) entityValidate(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.entityRow(r.Context(), s.DB, p, r.PathValue("entity"), "entity.manage")
	if e != nil {
		return e
	}
	var in struct {
		Definition entity.Definition `json:"definition"`
	}
	if e = decode(w, r, &in); e != nil {
		return e
	}
	view := r.URL.Query().Get("view")
	if view != "" && view != "diagnostics" {
		return bad("Unknown validation view.")
	}
	diags := in.Definition.Validate()
	if e = s.validateEntityRefs(r.Context(), s.DB, in.Definition, v); e != nil {
		diags = append(diags, entity.Diagnostic{Code: "invalid_reference", Path: "permissions", Message: e.Error()})
	}
	out := Row{"diagnostics": diags}
	if view != "diagnostics" {
		out["schema"], out["typescript"] = in.Definition.JSONSchema(), in.Definition.TypeScript()
	}
	send(w, 200, out)
	return nil
}

func (s *Server) entityPublish(w http.ResponseWriter, r *http.Request, p Principal) error {
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	v, e := s.entityRow(r.Context(), s.DB, p, r.PathValue("entity"), "entity.publish")
	if e != nil {
		return e
	}
	version := operationID(r.Context())
	e = s.mutate(r.Context(), p, "entity.publish", Resource{Tenant: str(v["tenant_id"]), Space: str(v["space_id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		v, e = row(r.Context(), tx, "SELECT * FROM entity_definitions WHERE id=? FOR UPDATE", v["id"])
		if e != nil {
			return e
		}
		if e = checkRevision(integer(v["revision"]), rev); e != nil {
			return e
		}
		if v["pending_version_id"] != nil {
			return conflict("A publication is already pending.")
		}
		var queued int
		if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM entity_versions WHERE tenant_id=? AND state IN ('queued','publishing','prepared')", v["tenant_id"]).Scan(&queued); e != nil {
			return e
		}
		if queued >= 16 {
			return conflict("Publication queue limit reached. Wait for queued changes to finish.")
		}
		if v["state"] != stateActive {
			return conflict("Archived entities cannot publish.")
		}
		return s.queueDefinition(r.Context(), tx, p, v, version, nil)
	})
	if e != nil {
		return e
	}
	send(w, 202, Row{"id": version, "state": "queued"})
	return nil
}

func (s *Server) entityPublication(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.entityRow(r.Context(), s.DB, p, r.PathValue("entity"), "entity.manage")
	if e != nil {
		return e
	}
	out, e := row(r.Context(), s.DB, "SELECT id,base_version_id,state,error_message,definition_hash,created_at,finished_at FROM entity_versions WHERE id=? AND entity_id=?", r.PathValue("version"), v["id"])
	if e == nil {
		send(w, 200, out)
	}
	return e
}

func (s *Server) entityTest(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.entityRow(r.Context(), s.DB, p, r.PathValue("entity"), "entity.manage")
	if e != nil {
		return e
	}
	var in struct {
		Definition entity.Definition `json:"definition"`
		App        string            `json:"app_id"`
		User       string            `json:"user_id"`
		Operation  string            `json:"operation"`
		Record     map[string]any    `json:"record"`
		Values     map[string]any    `json:"values"`
	}
	if e = decode(w, r, &in); e != nil {
		return e
	}
	if ds := in.Definition.Validate(); len(ds) > 0 {
		send(w, 200, Row{"diagnostics": ds, "allowed": false})
		return nil
	}
	if in.User == "" {
		in.User = p.ID
	}
	c, a, e := s.actorContext(r.Context(), s.DB, in.User, str(v["tenant_id"]), str(v["space_id"]), in.App)
	if e != nil {
		return e
	}
	c.Record = in.Record
	c.Old = in.Record
	c.Now = time.Now().UTC()
	candidate := in.Record
	if in.Operation == "create" || in.Operation == "update" {
		recordID := str(in.Record["id"])
		if recordID == "" {
			recordID = newID()
		}
		revision := integer(in.Record["revision"]) + 1
		if in.Operation == "create" {
			revision = 1
		}
		candidate, e = in.Definition.Prepare(c, in.Values, in.Operation == "create", recordID, revision)
	}
	out := Row{"allowed": false, "not_checked": []string{"live uniqueness", "reference existence/read authority", "record shares"}, "diagnostics": []any{}}
	if e != nil {
		out["error"] = e.Error()
	} else {
		c.Record = candidate
		fields := []string{}
		if in.Operation == "read" {
			fields = in.Definition.Projection
		} else {
			for k := range in.Values {
				fields = append(fields, k)
			}
		}
		allowed := in.Definition.Allowed(str(v["id"]), in.Operation, fields, c, a, nil)
		if in.Operation == "read" {
			allowed = true
			for _, key := range fields {
				allowed = allowed && in.Definition.Allowed(str(v["id"]), "read", []string{key}, c, a, nil)
			}
		}
		out["allowed"] = allowed
		out["candidate"] = candidate
		explanation := []Row{}
		for _, permission := range in.Definition.Permissions {
			if permission.Operation != in.Operation {
				continue
			}
			one := in.Definition
			one.Permissions = []entity.Permission{permission}
			explanation = append(explanation, Row{"key": permission.Key, "label": permission.Label, "fields": permission.Fields, "matches_complete_path": one.Allowed(str(v["id"]), in.Operation, nil, c, a, nil)})
		}
		out["permission_paths"] = explanation
		if e = in.Definition.CheckRules(c, in.Operation == "create"); (in.Operation == "create" || in.Operation == "update") && e != nil {
			out["allowed"] = false
			out["error"] = e.Error()
		}
	}
	send(w, 200, out)
	return nil
}

func (s *Server) actorContext(ctx context.Context, q Queryer, user, tenant, space, app string) (entity.Context, entity.AppAccess, error) {
	c := entity.Context{Actor: user, Now: time.Now().UTC(), Roles: map[string]bool{}, Teams: map[string]bool{}, ReviewTeams: map[string]bool{}, AdminTeams: map[string]bool{}}
	a := entity.AppAccess{Inherit: true}
	if e := s.member(ctx, q, Principal{ID: user}, tenant, true); e != nil {
		return c, a, e
	}
	query := strings.Replace(effectiveAssignmentSQL, " ORDER BY g.scope_kind,g.id", " AND (g.scope_kind='team' OR (g.scope_kind='space' AND g.space_id=?) OR (g.scope_kind='app' AND g.app_id=?)) ORDER BY g.scope_kind,g.id", 1)
	assignments, e := rows(ctx, q, query, user, tenant, space, app)
	if e != nil {
		return c, a, e
	}
	for _, g := range assignments {
		if (g["scope_kind"] == "space" && str(g["space_id"]) == space) || (g["scope_kind"] == "app" && str(g["app_id"]) == app) {
			c.Roles[str(g["role_id"])] = true
		}
		if g["scope_kind"] == "team" {
			if g["code"] == "team.reviewer" {
				c.ReviewTeams[str(g["scope_team_id"])] = true
			}
			if g["code"] == "team.admin" {
				c.AdminTeams[str(g["scope_team_id"])] = true
			}
		}
	}
	teams, e := rows(ctx, q, "SELECT gm.group_id FROM group_memberships gm JOIN groups g ON g.id=gm.group_id AND g.tenant_id=gm.tenant_id AND g.state='active' WHERE gm.tenant_id=? AND gm.user_id=? AND gm.state='active' LIMIT ?", tenant, user, entity.MaxActorTeams+1)
	if e != nil {
		return c, a, e
	}
	if len(teams) > entity.MaxActorTeams {
		return c, a, conflict("Membership expansion limit exceeded.")
	}
	for _, g := range teams {
		c.Teams[str(g["group_id"])] = true
	}
	if app != "" {
		v, e := row(ctx, q, "SELECT data_access_json FROM apps WHERE id=? AND tenant_id=? AND space_id=? AND state='active'", app, tenant, space)
		if e != nil {
			return c, a, e
		}
		b, _ := json.Marshal(v["data_access_json"])
		if e = json.Unmarshal(b, &a); e != nil {
			return c, a, e
		}
	}
	return c, a, nil
}

func (s *Server) appContracts(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if e = s.authorize(r.Context(), s.DB, p, "app.manage", Resource{Tenant: str(v["tenant_id"]), App: str(v["id"])}, true); e != nil {
		return e
	}
	a, e := accessOf(v)
	if e != nil {
		return e
	}
	manager := s.authorize(r.Context(), s.DB, p, "entity.manage", Resource{Tenant: str(v["tenant_id"]), Space: str(v["space_id"])}, true) == nil
	vs, e := rows(r.Context(), s.DB, "SELECT e.id,e.active_version_id,v.definition_hash,v.definition_json FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? AND e.state='active' ORDER BY e.label,e.id", v["space_id"])
	if e != nil {
		return e
	}
	out := []Row{}
	for _, value := range vs {
		if selected := r.URL.Query().Get("entity_id"); selected != "" && selected != str(value["id"]) {
			continue
		}
		d, e := defOf(value["definition_json"])
		if e != nil {
			return e
		}
		perms := []Row{}
		allowedFields := []string{}
		for _, permission := range d.Permissions {
			pid := entity.PermissionID(str(value["id"]), permission.Key)
			if !manager && !a.Inherit && !slices.ContainsFunc(a.Ceiling, func(g entity.Gate) bool { return g.Permission == pid }) {
				continue
			}
			fields := permission.Fields
			if !manager && !a.Inherit {
				fields = []string{}
				for _, g := range a.Ceiling {
					if g.Permission == pid {
						fields = append(fields, g.Fields...)
					}
				}
			}
			allowedFields = append(allowedFields, fields...)
			perms = append(perms, Row{"id": pid, "key": permission.Key, "label": permission.Label, "operation": permission.Operation, "fields": fields})
		}
		if len(perms) == 0 && !manager {
			continue
		}
		fields := []entity.Field{}
		for _, f := range d.Fields {
			if manager || slices.Contains(allowedFields, f.Key) {
				fields = append(fields, f)
			}
		}
		d.Fields = fields
		d.Permissions = nil
		d.Rules = nil
		section, field := r.URL.Query().Get("section"), r.URL.Query().Get("field")
		if version := r.URL.Query().Get("version"); version != "" && version != "published" {
			return bad("App contracts are always published.")
		}
		if section == "inventory" {
			if field != "" {
				return bad("field requires a generated contract section.")
			}
			out = append(out, Row{"id": value["id"], "published_version_id": value["active_version_id"], "definition_hash": value["definition_hash"], "key": d.Key, "label": d.Label, "section": "inventory", "fields": fieldInventory(fields), "permissions": perms})
			continue
		}
		if section != "" {
			var part any
			if section == "definition" {
				if field != "" {
					return bad("field applies only to generated contract sections.")
				}
				public, e := publicEntityFields(fields)
				if e != nil {
					return e
				}
				part = Row{"key": d.Key, "label": d.Label, "fields": public, "permissions": perms}
			} else {
				part, e = entitySection(d, section, field)
				if e != nil {
					return e
				}
			}
			out = append(out, Row{"id": value["id"], "published_version_id": value["active_version_id"], "definition_hash": value["definition_hash"], "section": section, "field": field, "partial": field != "", "value": part})
			continue
		}
		if field != "" {
			return bad("field requires a generated contract section.")
		}
		public, e := publicEntityFields(fields)
		if e != nil {
			return e
		}
		out = append(out, Row{"id": value["id"], "published_version_id": value["active_version_id"], "definition_hash": value["definition_hash"], "definition": Row{"key": d.Key, "label": d.Label, "fields": public, "permissions": perms}, "json_schema": d.JSONSchema(), "create_schema": d.Schema("create"), "update_schema": d.Schema("update"), "typescript": d.TypeScript()})
	}
	roles, e := rows(r.Context(), s.DB, "SELECT id,display_name FROM roles WHERE app_id=? AND domain='data' AND state='active'", v["id"])
	if e != nil {
		return e
	}
	response := Row{"app_id": v["id"], "space_id": v["space_id"], "app_revision": v["revision"], "entities": out, "roles": roles}
	if r.URL.Query().Get("section") == "inventory" {
		var page pageInfo
		out, page, e = pageItems(out, r.URL.Query())
		if e != nil {
			return e
		}
		response["entities"], response["pagination"] = out, page
	}
	send(w, 200, response)
	return nil
}

func (s *Server) schemaCredentials(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in credentials
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if in.Username == "" || in.Password == "" {
		return bad("Schema credentials are required.")
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	tenant, record := r.PathValue("tenant"), r.PathValue("id")
	e = s.mutate(r.Context(), p, "database.manage", Resource{Tenant: tenant}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		v, e := row(r.Context(), tx, "SELECT revision,management_mode FROM database_resources WHERE tenant_id=? AND id=? FOR UPDATE", tenant, record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(v["revision"]), rev); e != nil {
			return e
		}
		if v["management_mode"] != "registered" {
			return bad("Managed storage uses its target's schema authority.")
		}
		enc, e := s.seal("schema", record, tenant, in)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(r.Context(), "UPDATE database_resources SET schema_credentials_enc=?,credential_generation=credential_generation+1,revision=revision+1 WHERE id=?", enc, record)
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, tenant, p.ID, "database.schema_credentials_changed", "database", record, r.Header.Get("X-Request-ID"), Row{})
	})
	if e != nil {
		return e
	}
	send(w, 200, Row{"ok": true})
	return nil
}

// Reuse the canonical entity generators; selecting a section/field bounds agent
// context without inventing a second schema or exposing App-hidden rules/ACLs.
func entitySection(d entity.Definition, section, field string) (any, error) {
	if section == "definition" {
		if field != "" {
			return nil, bad("field applies only to generated contract sections.")
		}
		return d, nil
	}
	if field != "" {
		found := false
		for _, f := range d.Fields {
			if f.Key == field {
				d.Fields = []entity.Field{f}
				found = true
				break
			}
		}
		if !found {
			return nil, missing()
		}
	}
	switch section {
	case "json_schema":
		return d.JSONSchema(), nil
	case "create_schema":
		return d.Schema("create"), nil
	case "update_schema":
		return d.Schema("update"), nil
	case "typescript":
		return d.TypeScript(), nil
	default:
		return nil, bad("Unknown entity section.")
	}
}

func fieldInventory(fields []entity.Field) []Row {
	out := []Row{}
	for _, f := range fields {
		out = append(out, Row{"key": f.Key, "label": f.Label, "type": f.Type, "target_entity_id": f.Target})
	}
	return out
}
