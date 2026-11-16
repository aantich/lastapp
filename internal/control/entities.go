package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
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

// EntityDefinition is a Space entity: its draft, lifecycle and active and
// pending publications. The authoring view (Entity) adds the optional keys.
type EntityDefinition struct {
	ActiveDefinition  *jsonValue           `json:"active_definition,omitempty"`
	ActiveVersion     *string              `json:"active_version_id"`
	CreatedAt         time.Time            `json:"created_at"`
	Diagnostics       *[]entity.Diagnostic `json:"diagnostics,omitempty"`
	DraftContract     Row                  `json:"draft_contract,omitempty"`
	Draft             jsonValue            `json:"draft_json"`
	Key               string               `json:"entity_key"`
	ID                string               `json:"id"`
	Label             string               `json:"label"`
	PendingVersion    *string              `json:"pending_version_id"`
	Publications      *[]publicationItem   `json:"publications,omitempty"`
	PublishedContract Row                  `json:"published_contract,omitempty"`
	Revision          int64                `json:"revision"`
	SpaceID           string               `json:"space_id"`
	State             string               `json:"state"`
	TenantID          string               `json:"tenant_id"`
}

const entityColumns = "active_version_id,created_at,draft_json,entity_key,id,label,pending_version_id,revision,space_id,state,tenant_id"

func (v *EntityDefinition) fields() []any {
	return []any{&v.ActiveVersion, &v.CreatedAt, &v.Draft, &v.Key, &v.ID, &v.Label, &v.PendingVersion, &v.Revision, &v.SpaceID, &v.State, &v.TenantID}
}

// draft decodes the saved draft definition.
func (v EntityDefinition) draft() (entity.Definition, error) { return defOf(v.Draft.value()) }

// entityItem is an entity in a Space's entity collection.
type entityItem struct {
	ActiveVersion  *string `json:"active_version_id"`
	Key            string  `json:"entity_key"`
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	PendingVersion *string `json:"pending_version_id"`
	Revision       int64   `json:"revision"`
	State          string  `json:"state"`
}

func (v *entityItem) fields() []any {
	return []any{&v.ActiveVersion, &v.Key, &v.ID, &v.Label, &v.PendingVersion, &v.Revision, &v.State}
}

// publicationItem is one of an entity's recent publications.
type publicationItem struct {
	BaseVersion *string   `json:"base_version_id"`
	CreatedAt   time.Time `json:"created_at"`
	Hash        string    `json:"definition_hash"`
	Error       *string   `json:"error_message"`
	ID          string    `json:"id"`
	State       string    `json:"state"`
}

func (v *publicationItem) fields() []any {
	return []any{&v.BaseVersion, &v.CreatedAt, &v.Hash, &v.Error, &v.ID, &v.State}
}

// Publication is an entity publication with its completion time.
type Publication struct {
	BaseVersion *string    `json:"base_version_id"`
	CreatedAt   time.Time  `json:"created_at"`
	Hash        string     `json:"definition_hash"`
	Error       *string    `json:"error_message"`
	FinishedAt  *time.Time `json:"finished_at"`
	ID          string     `json:"id"`
	State       string     `json:"state"`
}

func (v *Publication) fields() []any {
	return []any{&v.BaseVersion, &v.CreatedAt, &v.Hash, &v.Error, &v.FinishedAt, &v.ID, &v.State}
}

// storedDefinition is a stored definition document.
type storedDefinition struct{ Definition jsonValue }

func (v *storedDefinition) fields() []any { return []any{&v.Definition} }

// publishedEntity is an active entity with its published definition.
type publishedEntity struct {
	ID, ActiveVersion, Hash string
	Definition              jsonValue
}

func (v *publishedEntity) fields() []any {
	return []any{&v.ID, &v.ActiveVersion, &v.Hash, &v.Definition}
}

// versionDefinition reads a stored entity version's definition document.
func versionDefinition(ctx context.Context, q Queryer, version string) (jsonValue, error) {
	var v storedDefinition
	return v.Definition, lookup(ctx, q, "SELECT definition_json FROM entity_versions WHERE id=?", []any{version}, &v.Definition)
}

func validateEntityRefs(ctx context.Context, q Queryer, d entity.Definition, v EntityDefinition) error {
	return validateEntityRefsCandidates(ctx, q, d, v.TenantID, v.SpaceID, v.ID, nil)
}

func validateEntityRefsCandidates(ctx context.Context, q Queryer, d entity.Definition, tenant, space, entityID string, candidates map[string]bool) error {
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

func (s *Server) actorContext(ctx context.Context, q Queryer, user, tenant, space, app string) (entity.Context, entity.AppAccess, error) {
	c := entity.Context{Actor: user, Now: time.Now().UTC(), Roles: map[string]bool{}, Teams: map[string]bool{}, ReviewTeams: map[string]bool{}, AdminTeams: map[string]bool{}}
	a := entity.AppAccess{Inherit: true}
	if e := s.member(ctx, q, Principal{ID: user}, tenant, true); e != nil {
		return c, a, e
	}
	query := strings.Replace(effectiveAssignmentSQL, " ORDER BY g.scope_kind,g.id", " AND (g.scope_kind='team' OR (g.scope_kind='space' AND g.space_id=?) OR (g.scope_kind='app' AND g.app_id=?)) ORDER BY g.scope_kind,g.id", 1)
	assignments, e := scanAll[EffectiveAssignment](ctx, q, query, user, tenant, space, app)
	if e != nil {
		return c, a, e
	}
	for _, g := range assignments {
		if (g.ScopeKind == "space" && deref(g.SpaceID) == space) || (g.ScopeKind == "app" && deref(g.AppID) == app) {
			c.Roles[g.RoleID] = true
		}
		if g.ScopeKind == "team" {
			if g.Code == "team.reviewer" {
				c.ReviewTeams[deref(g.ScopeTeamID)] = true
			}
			if g.Code == "team.admin" {
				c.AdminTeams[deref(g.ScopeTeamID)] = true
			}
		}
	}
	teams, e := scanStrings(ctx, q, "SELECT gm.group_id FROM group_memberships gm JOIN groups g ON g.id=gm.group_id AND g.tenant_id=gm.tenant_id AND g.state='active' WHERE gm.tenant_id=? AND gm.user_id=? AND gm.state='active' LIMIT ?", tenant, user, entity.MaxActorTeams+1)
	if e != nil {
		return c, a, e
	}
	if len(teams) > entity.MaxActorTeams {
		return c, a, conflict("Membership expansion limit exceeded.")
	}
	for _, g := range teams {
		c.Teams[g] = true
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

// authoring owns Space entity drafts, publication requests, policy checks and
// the App contracts and data access derived from them. Console handlers and
// MCP tools both call these methods.
type authoring struct {
	db           *sql.DB
	run          *runner
	config       *Config
	queue        func(ctx context.Context, tx *sql.Tx, p Principal, v EntityDefinition, version string, candidates map[string]bool) error
	actorContext func(ctx context.Context, q Queryer, user, tenant, space, app string) (entity.Context, entity.AppAccess, error)
}

type (
	entityCreateInput struct {
		ID         string            `json:"id"`
		Definition entity.Definition `json:"definition"`
	}
	entitySaveInput struct {
		Definition entity.Definition `json:"definition"`
		State      string            `json:"state"`
	}
	// entityView selects a generated section of a draft or published
	// definition; the zero value is the full authoring view.
	entityView struct {
		Section, Version, Field string
	}
	entityTestInput struct {
		Definition entity.Definition `json:"definition"`
		App        string            `json:"app_id"`
		User       string            `json:"user_id"`
		Operation  string            `json:"operation"`
		Record     map[string]any    `json:"record"`
		Values     map[string]any    `json:"values"`
	}
	// contractView selects App contracts: one entity, a generated section and
	// field, and the page of the inventory section.
	contractView struct {
		Entity, Section, Field, Version string
		Page                            PageRequest
	}
)

// entityRow reads an entity and checks action on its Space with the
// stricter mutating rules, as every entity operation does.
func (v *authoring) entityRow(ctx context.Context, q Queryer, a Actor, key, action string) (EntityDefinition, error) {
	out, e := scanOne[EntityDefinition](ctx, q, "SELECT "+entityColumns+" FROM entity_definitions WHERE id=?", key)
	if e != nil {
		return out, e
	}
	e = v.run.check(ctx, q, a, action, Resource{Tenant: out.TenantID, Space: out.SpaceID}, true)
	return out, e
}

// authoringSpace reads an active Space whose entities a may manage.
func (v *authoring) authoringSpace(ctx context.Context, a Actor, space string) (Resource, error) {
	res := Resource{Space: space}
	if e := lookup(ctx, v.db, "SELECT tenant_id FROM spaces WHERE id=? AND state='active'", []any{space}, &res.Tenant); e != nil {
		return res, e
	}
	return res, v.run.check(ctx, v.db, a, "entity.manage", res, true)
}

func defOf(v any) (entity.Definition, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return entity.Definition{}, e
	}
	return entity.Decode(b)
}

func (v *authoring) Entities(ctx context.Context, a Actor, space string) (Row, error) {
	sp, e := v.authoringSpace(ctx, a, space)
	if e != nil {
		return nil, e
	}
	items, e := scanAll[entityItem](ctx, v.db, "SELECT active_version_id,entity_key,id,label,pending_version_id,revision,state FROM entity_definitions WHERE space_id=? AND tenant_id=? ORDER BY label,id", sp.Space, sp.Tenant)
	if e != nil {
		return nil, e
	}
	return Row{"items": items}, nil
}

// CreateEntity adds a draft to a Space and returns its authoring view.
func (v *authoring) CreateEntity(ctx context.Context, a Actor, space string, in entityCreateInput) (EntityDefinition, error) {
	if in.ID == "" {
		in.ID = newID()
	}
	if e := id(in.ID); e != nil {
		return EntityDefinition{}, e
	}
	sp, e := v.authoringSpace(ctx, a, space)
	if e != nil {
		return EntityDefinition{}, e
	}
	if in.Definition.Format != 1 || len(in.Definition.Key) > 48 || !entity.ValidKey(in.Definition.Key) || len(in.Definition.Label) > 160 || in.Definition.Label == "" {
		return EntityDefinition{}, bad("Provide an entity key and name.")
	}
	b, _ := json.Marshal(in.Definition)
	if len(b) > entity.MaxDefinitionBytes {
		return EntityDefinition{}, bad("Definition exceeds its encoded-size limit.")
	}
	tenant := sp.Tenant
	e = v.run.mutate(ctx, a, "entity.manage", sp, func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM entity_definitions WHERE space_id=?", sp.Space).Scan(&n); e != nil {
			return e
		}
		if n >= 100 {
			return conflict("Space entity limit reached.")
		}
		_, e := tx.ExecContext(ctx, "INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,draft_json) VALUES(?,?,?,?,?,?)", in.ID, sp.Tenant, sp.Space, in.Definition.Key, in.Definition.Label, b)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "entity.created", "entity", in.ID, Row{})
	})
	if e != nil {
		return EntityDefinition{}, e
	}
	return v.detailOf(ctx, a, in.ID)
}

// Entity is the authoring view of a draft with its generated contracts and
// recent publications, or one selected section of it.
func (v *authoring) Entity(ctx context.Context, a Actor, key string, view entityView) (any, error) {
	out, e := v.entityRow(ctx, v.db, a, key, "entity.manage")
	if e != nil {
		return nil, e
	}
	if view.Section != "" {
		version := view.Version
		if version == "" {
			version = "draft"
		}
		definition := out.Draft
		if version == "published" {
			if out.ActiveVersion == nil {
				return nil, missing()
			}
			active, err := versionDefinition(ctx, v.db, *out.ActiveVersion)
			if err != nil {
				return nil, err
			}
			definition = active
		} else if version != "draft" {
			return nil, bad("version must be draft or published.")
		}
		d, err := defOf(definition.value())
		if err != nil {
			return nil, err
		}
		value, err := entitySection(d, view.Section, view.Field)
		if err != nil {
			return nil, err
		}
		return Row{"id": out.ID, "space_id": out.SpaceID, "revision": out.Revision, "version": version, "active_version_id": out.ActiveVersion, "definition_hash": entity.Hash(d), "section": view.Section, "field": view.Field, "partial": view.Field != "", "value": value}, nil
	}
	if view.Field != "" || view.Version != "" {
		return nil, bad("field/version require a section.")
	}
	return v.entityDetail(ctx, out)
}

// detailOf reads the full authoring view of an entity.
func (v *authoring) detailOf(ctx context.Context, a Actor, key string) (EntityDefinition, error) {
	out, e := v.entityRow(ctx, v.db, a, key, "entity.manage")
	if e != nil {
		return out, e
	}
	return v.entityDetail(ctx, out)
}

// entityDetail adds an entity's diagnostics, generated contracts and recent
// publications: the full authoring view.
func (v *authoring) entityDetail(ctx context.Context, out EntityDefinition) (EntityDefinition, error) {
	d, e := out.draft()
	if e != nil {
		return out, e
	}
	diagnostics := d.Validate()
	out.Diagnostics = &diagnostics
	out.DraftContract = Row{"definition_hash": entity.Hash(d), "json_schema": d.JSONSchema(), "create_schema": d.Schema("create"), "update_schema": d.Schema("update"), "typescript": d.TypeScript()}
	versions, e := scanAll[publicationItem](ctx, v.db, "SELECT base_version_id,created_at,definition_hash,error_message,id,state FROM entity_versions WHERE entity_id=? ORDER BY created_at DESC LIMIT 20", out.ID)
	if e != nil {
		return out, e
	}
	out.Publications = &versions
	if out.ActiveVersion != nil {
		active, e := versionDefinition(ctx, v.db, *out.ActiveVersion)
		if e != nil {
			return out, e
		}
		out.ActiveDefinition = &active
		published, e := defOf(active.value())
		if e != nil {
			return out, e
		}
		out.PublishedContract = Row{"version_id": out.ActiveVersion, "definition_hash": entity.Hash(published), "json_schema": published.JSONSchema(), "create_schema": published.Schema("create"), "update_schema": published.Schema("update"), "typescript": published.TypeScript()}
	}
	return out, nil
}

// SaveEntity replaces a draft, optionally archiving or reactivating it, and
// returns its authoring view.
func (v *authoring) SaveEntity(ctx context.Context, a Actor, key string, in entitySaveInput, revision int64) (EntityDefinition, error) {
	if revision < 1 {
		return EntityDefinition{}, revisionRequired()
	}
	old, e := v.entityRow(ctx, v.db, a, key, "entity.manage")
	if e != nil {
		return EntityDefinition{}, e
	}
	if in.Definition.Key != old.Key || in.Definition.Format != 1 || in.Definition.Label == "" || len(in.Definition.Label) > 160 {
		return EntityDefinition{}, bad("Entity key/format are immutable and a name is required.")
	}
	b, _ := json.Marshal(in.Definition)
	if len(b) > entity.MaxDefinitionBytes {
		return EntityDefinition{}, bad("Definition exceeds its encoded-size limit.")
	}
	tenant := old.TenantID
	e = v.run.mutate(ctx, a, "entity.manage", Resource{Tenant: tenant, Space: old.SpaceID}, func(tx *sql.Tx) error {
		var current int64
		var pending *string
		e := lookup(ctx, tx, "SELECT revision,pending_version_id FROM entity_definitions WHERE id=? FOR UPDATE", []any{old.ID}, &current, &pending)
		if e != nil {
			return e
		}
		if e = checkRevision(current, revision); e != nil {
			return e
		}
		state := old.State
		if in.State != "" {
			if !slices.Contains([]string{stateActive, stateArchived}, in.State) {
				return bad("Invalid lifecycle state.")
			}
			if pending != nil {
				return conflict("Wait for publishing before archiving.")
			}
			if in.State == stateArchived {
				refs, e := scanAll[storedDefinition](ctx, tx, "SELECT v.definition_json FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? AND e.state='active' AND e.id<>?", old.SpaceID, old.ID)
				if e != nil {
					return e
				}
				for _, ref := range refs {
					d, e := defOf(ref.Definition.value())
					if e != nil {
						return e
					}
					for _, f := range d.Fields {
						if f.Target == old.ID {
							return conflict("An active entity references this entity.")
						}
					}
				}
			}
			state = in.State
		}
		_, e = tx.ExecContext(ctx, "UPDATE entity_definitions SET label=?,draft_json=?,state=?,revision=revision+1 WHERE id=?", in.Definition.Label, b, state, old.ID)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "entity.saved", "entity", old.ID, Row{"state": state})
	})
	if e != nil {
		return EntityDefinition{}, e
	}
	return v.detailOf(ctx, a, key)
}

// ValidateEntity checks a candidate definition without saving it. The
// diagnostics view omits the generated schema and TypeScript.
func (v *authoring) ValidateEntity(ctx context.Context, a Actor, key string, d entity.Definition, view string) (Row, error) {
	current, e := v.entityRow(ctx, v.db, a, key, "entity.manage")
	if e != nil {
		return nil, e
	}
	if view != "" && view != "diagnostics" {
		return nil, bad("Unknown validation view.")
	}
	diags := d.Validate()
	if e = validateEntityRefs(ctx, v.db, d, current); e != nil {
		diags = append(diags, entity.Diagnostic{Code: "invalid_reference", Path: "permissions", Message: e.Error()})
	}
	out := Row{"diagnostics": diags}
	if view != "diagnostics" {
		out["schema"], out["typescript"] = d.JSONSchema(), d.TypeScript()
	}
	return out, nil
}

// PublishEntity queues the saved draft for the publication worker.
func (v *authoring) PublishEntity(ctx context.Context, a Actor, key string, revision int64) (Row, error) {
	if revision < 1 {
		return nil, revisionRequired()
	}
	current, e := v.entityRow(ctx, v.db, a, key, "entity.publish")
	if e != nil {
		return nil, e
	}
	version := a.operationID()
	e = v.run.mutate(ctx, a, "entity.publish", Resource{Tenant: current.TenantID, Space: current.SpaceID}, func(tx *sql.Tx) error {
		current, e = scanOne[EntityDefinition](ctx, tx, "SELECT "+entityColumns+" FROM entity_definitions WHERE id=? FOR UPDATE", current.ID)
		if e != nil {
			return e
		}
		if e = checkRevision(current.Revision, revision); e != nil {
			return e
		}
		if current.PendingVersion != nil {
			return conflict("A publication is already pending.")
		}
		var queued int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM entity_versions WHERE tenant_id=? AND state IN ('queued','publishing','prepared')", current.TenantID).Scan(&queued); e != nil {
			return e
		}
		if queued >= 16 {
			return conflict("Publication queue limit reached. Wait for queued changes to finish.")
		}
		if current.State != stateActive {
			return conflict("Archived entities cannot publish.")
		}
		return v.queue(ctx, tx, a.Principal, current, version, nil)
	})
	if e != nil {
		return nil, e
	}
	return Row{"id": version, "state": "queued"}, nil
}

func (v *authoring) Publication(ctx context.Context, a Actor, key, version string) (Publication, error) {
	current, e := v.entityRow(ctx, v.db, a, key, "entity.manage")
	if e != nil {
		return Publication{}, e
	}
	return scanOne[Publication](ctx, v.db, "SELECT base_version_id,created_at,definition_hash,error_message,finished_at,id,state FROM entity_versions WHERE id=? AND entity_id=?", version, current.ID)
}

// TestEntity evaluates one synthetic case against a candidate definition
// for a real Tenant member, without live uniqueness, references or shares.
func (v *authoring) TestEntity(ctx context.Context, a Actor, key string, in entityTestInput) (Row, error) {
	current, e := v.entityRow(ctx, v.db, a, key, "entity.manage")
	if e != nil {
		return nil, e
	}
	if ds := in.Definition.Validate(); len(ds) > 0 {
		return Row{"diagnostics": ds, "allowed": false}, nil
	}
	if in.User == "" {
		in.User = a.Principal.ID
	}
	c, access, e := v.actorContext(ctx, v.db, in.User, current.TenantID, current.SpaceID, in.App)
	if e != nil {
		return nil, e
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
		return out, nil
	}
	c.Record = candidate
	fields := []string{}
	if in.Operation == "read" {
		fields = in.Definition.Projection
	} else {
		for k := range in.Values {
			fields = append(fields, k)
		}
	}
	entityID := current.ID
	allowed := in.Definition.Allowed(entityID, in.Operation, fields, c, access, nil)
	if in.Operation == "read" {
		allowed = true
		for _, field := range fields {
			allowed = allowed && in.Definition.Allowed(entityID, "read", []string{field}, c, access, nil)
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
		explanation = append(explanation, Row{"key": permission.Key, "label": permission.Label, "fields": permission.Fields, "matches_complete_path": one.Allowed(entityID, in.Operation, nil, c, access, nil)})
	}
	out["permission_paths"] = explanation
	if e = in.Definition.CheckRules(c, in.Operation == "create"); (in.Operation == "create" || in.Operation == "update") && e != nil {
		out["allowed"] = false
		out["error"] = e.Error()
	}
	return out, nil
}

// AppContracts are the published entity contracts an App may use: its
// access ceiling narrows fields and permissions unless a is a Space manager.
func (v *authoring) AppContracts(ctx context.Context, a Actor, app string, view contractView) (Row, error) {
	current, e := row(ctx, v.db, "SELECT * FROM apps WHERE id=?", app)
	if e != nil {
		return nil, e
	}
	if e = v.run.check(ctx, v.db, a, "app.manage", Resource{Tenant: str(current["tenant_id"]), App: str(current["id"])}, true); e != nil {
		return nil, e
	}
	access, e := accessOf(current)
	if e != nil {
		return nil, e
	}
	manager := v.run.check(ctx, v.db, a, "entity.manage", Resource{Tenant: str(current["tenant_id"]), Space: str(current["space_id"])}, true) == nil
	vs, e := scanAll[publishedEntity](ctx, v.db, "SELECT e.id,e.active_version_id,v.definition_hash,v.definition_json FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.space_id=? AND e.state='active' ORDER BY e.label,e.id", current["space_id"])
	if e != nil {
		return nil, e
	}
	out := []Row{}
	for _, value := range vs {
		if view.Entity != "" && view.Entity != value.ID {
			continue
		}
		d, e := defOf(value.Definition.value())
		if e != nil {
			return nil, e
		}
		perms := []Row{}
		allowedFields := []string{}
		for _, permission := range d.Permissions {
			pid := entity.PermissionID(value.ID, permission.Key)
			if !manager && !access.Inherit && !slices.ContainsFunc(access.Ceiling, func(g entity.Gate) bool { return g.Permission == pid }) {
				continue
			}
			fields := permission.Fields
			if !manager && !access.Inherit {
				fields = []string{}
				for _, g := range access.Ceiling {
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
		section, field := view.Section, view.Field
		if view.Version != "" && view.Version != "published" {
			return nil, bad("App contracts are always published.")
		}
		if section == "inventory" {
			if field != "" {
				return nil, bad("field requires a generated contract section.")
			}
			out = append(out, Row{"id": value.ID, "published_version_id": value.ActiveVersion, "definition_hash": value.Hash, "key": d.Key, "label": d.Label, "section": "inventory", "fields": fieldInventory(fields), "permissions": perms})
			continue
		}
		if section != "" {
			var part any
			if section == "definition" {
				if field != "" {
					return nil, bad("field applies only to generated contract sections.")
				}
				public, e := publicEntityFields(fields)
				if e != nil {
					return nil, e
				}
				part = Row{"key": d.Key, "label": d.Label, "fields": public, "permissions": perms}
			} else {
				part, e = entitySection(d, section, field)
				if e != nil {
					return nil, e
				}
			}
			out = append(out, Row{"id": value.ID, "published_version_id": value.ActiveVersion, "definition_hash": value.Hash, "section": section, "field": field, "partial": field != "", "value": part})
			continue
		}
		if field != "" {
			return nil, bad("field requires a generated contract section.")
		}
		public, e := publicEntityFields(fields)
		if e != nil {
			return nil, e
		}
		out = append(out, Row{"id": value.ID, "published_version_id": value.ActiveVersion, "definition_hash": value.Hash, "definition": Row{"key": d.Key, "label": d.Label, "fields": public, "permissions": perms}, "json_schema": d.JSONSchema(), "create_schema": d.Schema("create"), "update_schema": d.Schema("update"), "typescript": d.TypeScript()})
	}
	roles, e := rows(ctx, v.db, "SELECT id,display_name FROM roles WHERE app_id=? AND domain='data' AND state='active'", current["id"])
	if e != nil {
		return nil, e
	}
	response := Row{"app_id": current["id"], "space_id": current["space_id"], "app_revision": current["revision"], "entities": out, "roles": roles}
	if view.Section == "inventory" {
		var page pageInfo
		out, page, e = pageItems(out, view.Page)
		if e != nil {
			return nil, e
		}
		response["entities"], response["pagination"] = out, page
	}
	return response, nil
}

// AppAccess is an App's data-access policy with its revision.
func (v *authoring) AppAccess(ctx context.Context, a Actor, app string) (Row, error) {
	current, e := row(ctx, v.db, "SELECT * FROM apps WHERE id=?", app)
	if e != nil {
		return nil, e
	}
	if e = v.run.check(ctx, v.db, a, "app.manage", Resource{Tenant: str(current["tenant_id"]), App: str(current["id"])}, true); e != nil {
		return nil, e
	}
	access, e := accessOf(current)
	if e != nil {
		return nil, e
	}
	return Row{"revision": current["revision"], "access": access}, nil
}

// SaveAppAccess replaces an App's origin, ceiling and restrictions. Only a
// Space manager may change the origin or widen the ceiling.
func (v *authoring) SaveAppAccess(ctx context.Context, a Actor, app string, in entity.AppAccess, revision int64) (Row, error) {
	if revision < 1 {
		return nil, revisionRequired()
	}
	current, e := row(ctx, v.db, "SELECT * FROM apps WHERE id=?", app)
	if e != nil {
		return nil, e
	}
	res := Resource{Tenant: str(current["tenant_id"]), App: str(current["id"])}
	e = v.run.mutate(ctx, a, "app.manage", res, func(tx *sql.Tx) error {
		current, e = row(ctx, tx, "SELECT * FROM apps WHERE id=?", str(current["id"]))
		if e != nil {
			return e
		}
		if e = checkRevision(integer(current["revision"]), revision); e != nil {
			return e
		}
		var wid sql.NullString
		if e = tx.QueryRowContext(ctx, "SELECT workspace_id FROM spaces WHERE id=?", current["space_id"]).Scan(&wid); e != nil {
			return e
		}
		workspace := wid.Valid
		old, e := accessOf(current)
		if e != nil {
			return e
		}
		spaceManager := v.run.authorizeTx(ctx, tx, a, "entity.manage", Resource{Tenant: res.Tenant, Space: str(current["space_id"])}) == nil
		if !spaceManager && (old.Origin != in.Origin || old.Inherit != in.Inherit || entity.Hash(old.Ceiling) != entity.Hash(in.Ceiling)) {
			return problem(403, "space_approval_required", "A Space manager must approve the origin and access ceiling.")
		}
		if in.Origin != "" {
			u, _ := url.Parse(in.Origin)
			admin, _ := url.Parse(v.config.PublicURL)
			loopback := u != nil && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback())
			if !validOrigin(in.Origin, false) || u == nil || len(in.Origin) > 500 || u.Scheme != "https" && !((v.config.Development || workspace) && loopback) || u.Hostname() != strings.ToLower(u.Hostname()) || strings.EqualFold(strings.TrimSuffix(u.Hostname(), "."), strings.TrimSuffix(admin.Hostname(), ".")) {
				return bad("App origin must use a separate host from the console.")
			}
		}
		if len(in.Ceiling)+len(in.Restrictions) > 128 {
			return bad("Too many App access gates.")
		}
		for _, gs := range [][]entity.Gate{in.Ceiling, in.Restrictions} {
			for _, g := range gs {
				if e := id(g.Permission); e != nil {
					return e
				}
				pv, e := scanOne[storedDefinition](ctx, tx, "SELECT v.definition_json FROM permissions p JOIN entity_versions v ON v.id=p.version_id WHERE p.id=? AND p.tenant_id=? AND p.space_id=? AND p.state='active' AND p.domain='data'", g.Permission, res.Tenant, current["space_id"])
				if e != nil {
					return bad("Choose an active permission from this Space.")
				}
				d, e := defOf(pv.Definition.value())
				if e != nil {
					return e
				}
				for _, f := range g.Fields {
					if _, ok := d.Field(f); !ok {
						return bad("Unknown gate field.")
					}
				}
				if g.When != nil {
					if e = d.CheckCondition(*g.When, false); e != nil {
						return bad(e.Error())
					}
				}
				for _, role := range g.Roles {
					var n int
					if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM roles WHERE id=? AND tenant_id=? AND app_id=? AND domain='data' AND state='active'", role, res.Tenant, current["id"]).Scan(&n); e != nil {
						return e
					}
					if n != 1 {
						return bad("App gates can reference only this App's business roles.")
					}
				}
			}
		}
		host := ""
		if in.Origin != "" && !workspace {
			parsed, _ := url.Parse(in.Origin)
			host = strings.ToLower(parsed.Hostname())
			var occupied int
			if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM apps WHERE origin_host=? AND id<>?", host, current["id"]).Scan(&occupied); e != nil {
				return e
			}
			if occupied > 0 {
				return conflict("Each App needs a separate host. This host is already approved for another App.")
			}
		}
		b, _ := json.Marshal(in)
		_, e = tx.ExecContext(ctx, "UPDATE apps SET data_access_json=?,approved_origin=?,origin_host=?,origin_generation=origin_generation+IF(?<>?,1,0),revision=revision+1 WHERE id=?", b, nullable(in.Origin), nullable(host), old.Origin, in.Origin, current["id"])
		if e != nil {
			return e
		}
		if e = projectAppPermissions(ctx, tx, str(current["id"]), in); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, res.Tenant, "app.data_access_changed", "app", str(current["id"]), Row{})
	})
	if e != nil {
		return nil, e
	}
	return v.AppAccess(ctx, a, app)
}

func entityViewOf(r *http.Request) entityView {
	q := r.URL.Query()
	return entityView{Section: q.Get("section"), Version: q.Get("version"), Field: q.Get("field")}
}

func (s *Server) entityList(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.authoring.Entities(r.Context(), httpActor(r, p), r.PathValue("space"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) entityCreate(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in entityCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	a := httpActor(r, p)
	created, e := s.authoring.CreateEntity(r.Context(), a, r.PathValue("space"), in)
	var v any = created
	if e == nil && entityViewOf(r) != (entityView{}) {
		v, e = s.authoring.Entity(r.Context(), a, created.ID, entityViewOf(r))
	}
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) entityGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.authoring.Entity(r.Context(), httpActor(r, p), r.PathValue("entity"), entityViewOf(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) entitySave(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in entitySaveInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	a := httpActor(r, p)
	saved, e := s.authoring.SaveEntity(r.Context(), a, r.PathValue("entity"), in, ifMatch(r))
	var v any = saved
	if e == nil && entityViewOf(r) != (entityView{}) {
		v, e = s.authoring.Entity(r.Context(), a, r.PathValue("entity"), entityViewOf(r))
	}
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) entityValidate(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		Definition entity.Definition `json:"definition"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authoring.ValidateEntity(r.Context(), httpActor(r, p), r.PathValue("entity"), in.Definition, r.URL.Query().Get("view"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) entityPublish(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.authoring.PublishEntity(r.Context(), httpActor(r, p), r.PathValue("entity"), ifMatch(r))
	if e == nil {
		send(w, 202, v)
	}
	return e
}

func (s *Server) entityPublication(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.authoring.Publication(r.Context(), httpActor(r, p), r.PathValue("entity"), r.PathValue("version"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) entityTest(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in entityTestInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authoring.TestEntity(r.Context(), httpActor(r, p), r.PathValue("entity"), in)
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) appContracts(w http.ResponseWriter, r *http.Request, p Principal) error {
	q := r.URL.Query()
	view := contractView{Entity: q.Get("entity_id"), Section: q.Get("section"), Field: q.Get("field"), Version: q.Get("version"), Page: pageRequest(q)}
	v, e := s.authoring.AppContracts(r.Context(), httpActor(r, p), r.PathValue("app"), view)
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) appAccessGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.authoring.AppAccess(r.Context(), httpActor(r, p), r.PathValue("app"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) appAccessSave(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in entity.AppAccess
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authoring.SaveAppAccess(r.Context(), httpActor(r, p), r.PathValue("app"), in, ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}
