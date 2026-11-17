package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

func (s *Server) entityRoutes() {
	s.api("GET /v1/entity-capabilities", func(w http.ResponseWriter, r *http.Request, p access.Principal) error {
		httpapi.Send(w, 200, entity.Capabilities())
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

func (s *Server) actorContext(ctx context.Context, q registry.Queryer, user, tenant, space, app string) (entity.Context, entity.AppAccess, error) {
	c := entity.Context{Actor: user, Now: time.Now().UTC(), Roles: map[string]bool{}, Teams: map[string]bool{}, ReviewTeams: map[string]bool{}, AdminTeams: map[string]bool{}}
	a := entity.AppAccess{Inherit: true}
	if e := s.Member(ctx, q, access.Principal{ID: user}, tenant, true); e != nil {
		return c, a, e
	}
	query := strings.Replace(access.EffectiveAssignmentSQL, " ORDER BY g.scope_kind,g.id", " AND (g.scope_kind='team' OR (g.scope_kind='space' AND g.space_id=?) OR (g.scope_kind='app' AND g.app_id=?)) ORDER BY g.scope_kind,g.id", 1)
	assignments, e := registry.ScanAll[access.EffectiveAssignment](ctx, q, query, user, tenant, space, app)
	if e != nil {
		return c, a, e
	}
	for _, g := range assignments {
		if (g.ScopeKind == "space" && access.Deref(g.SpaceID) == space) || (g.ScopeKind == "app" && access.Deref(g.AppID) == app) {
			c.Roles[g.RoleID] = true
		}
		if g.ScopeKind == "team" {
			if g.Code == "team.reviewer" {
				c.ReviewTeams[access.Deref(g.ScopeTeamID)] = true
			}
			if g.Code == "team.admin" {
				c.AdminTeams[access.Deref(g.ScopeTeamID)] = true
			}
		}
	}
	teams, e := registry.ScanStrings(ctx, q, "SELECT gm.group_id FROM group_memberships gm JOIN groups g ON g.id=gm.group_id AND g.tenant_id=gm.tenant_id AND g.state='active' WHERE gm.tenant_id=? AND gm.user_id=? AND gm.state='active' LIMIT ?", tenant, user, entity.MaxActorTeams+1)
	if e != nil {
		return c, a, e
	}
	if len(teams) > entity.MaxActorTeams {
		return c, a, registry.Conflict("Membership expansion limit exceeded.")
	}
	for _, g := range teams {
		c.Teams[g] = true
	}
	if app != "" {
		var access registry.JSONValue
		if e := registry.Lookup(ctx, q, "SELECT data_access_json FROM apps WHERE id=? AND tenant_id=? AND space_id=? AND state='active'", []any{app, tenant, space}, &access); e != nil {
			return c, a, e
		}
		b, _ := json.Marshal(access)
		if e = json.Unmarshal(b, &a); e != nil {
			return c, a, e
		}
	}
	return c, a, nil
}

func (s *Server) schemaCredentials(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in databases.Credentials
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if in.Username == "" || in.Password == "" {
		return registry.Bad("Schema credentials are required.")
	}
	rev, e := httpapi.ExpectedRevision(r)
	if e != nil {
		return e
	}
	tenant, record := r.PathValue("tenant"), r.PathValue("id")
	e = s.mutate(r.Context(), p, "database.manage", access.Resource{Tenant: tenant}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		var current int64
		var mode string
		e := registry.Lookup(r.Context(), tx, "SELECT revision,management_mode FROM database_resources WHERE tenant_id=? AND id=? FOR UPDATE", []any{tenant, record}, &current, &mode)
		if e != nil {
			return e
		}
		if e = op.CheckRevision(current, rev); e != nil {
			return e
		}
		if mode != "registered" {
			return registry.Bad("Managed storage uses its target's schema authority.")
		}
		enc, e := s.seal("schema", record, tenant, in)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(r.Context(), "UPDATE database_resources SET schema_credentials_enc=?,credential_generation=credential_generation+1,revision=revision+1 WHERE id=?", enc, record)
		if e != nil {
			return e
		}
		return op.Audit(r.Context(), tx, tenant, p.ID, "database.schema_credentials_changed", "database", record, r.Header.Get("X-Request-ID"), registry.Row{})
	})
	if e != nil {
		return e
	}
	httpapi.Send(w, 200, registry.Row{"ok": true})
	return nil
}

func (s *Server) entityList(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.authoring.Entities(r.Context(), httpapi.Actor(r, p), r.PathValue("space"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) entityCreate(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in entities.EntityCreateInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	a := httpapi.Actor(r, p)
	created, e := s.authoring.CreateEntity(r.Context(), a, r.PathValue("space"), in)
	var v any = created
	if e == nil && entityViewOf(r) != (entities.EntityView{}) {
		v, e = s.authoring.Entity(r.Context(), a, created.ID, entityViewOf(r))
	}
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) entityGet(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.authoring.Entity(r.Context(), httpapi.Actor(r, p), r.PathValue("entity"), entityViewOf(r))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) entitySave(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in entities.EntitySaveInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	a := httpapi.Actor(r, p)
	saved, e := s.authoring.SaveEntity(r.Context(), a, r.PathValue("entity"), in, httpapi.IfMatch(r))
	var v any = saved
	if e == nil && entityViewOf(r) != (entities.EntityView{}) {
		v, e = s.authoring.Entity(r.Context(), a, r.PathValue("entity"), entityViewOf(r))
	}
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) entityValidate(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		Definition entity.Definition `json:"definition"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authoring.ValidateEntity(r.Context(), httpapi.Actor(r, p), r.PathValue("entity"), in.Definition, r.URL.Query().Get("view"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) entityPublish(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.authoring.PublishEntity(r.Context(), httpapi.Actor(r, p), r.PathValue("entity"), httpapi.IfMatch(r))
	if e == nil {
		httpapi.Send(w, 202, v)
	}
	return e
}

func (s *Server) entityPublication(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.authoring.Publication(r.Context(), httpapi.Actor(r, p), r.PathValue("entity"), r.PathValue("version"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) entityTest(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in entities.EntityTestInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authoring.TestEntity(r.Context(), httpapi.Actor(r, p), r.PathValue("entity"), in)
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) appContracts(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	q := r.URL.Query()
	view := entities.ContractView{Entity: q.Get("entity_id"), Section: q.Get("section"), Field: q.Get("field"), Version: q.Get("version"), Page: httpapi.PageRequest(q)}
	v, e := s.authoring.AppContracts(r.Context(), httpapi.Actor(r, p), r.PathValue("app"), view)
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) appAccessGet(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.authoring.AppAccess(r.Context(), httpapi.Actor(r, p), r.PathValue("app"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) appAccessSave(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in entity.AppAccess
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authoring.SaveAppAccess(r.Context(), httpapi.Actor(r, p), r.PathValue("app"), in, httpapi.IfMatch(r))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func entityViewOf(r *http.Request) entities.EntityView {
	q := r.URL.Query()
	return entities.EntityView{Section: q.Get("section"), Version: q.Get("version"), Field: q.Get("field")}
}
