// Package dataapi is the App business-data runtime behind /data/v1: App launch
// codes and bearer sessions, entity schema, record queries and mutations,
// capabilities, reference options and record shares. Every request rechecks the
// live session parent, App origin generation and actor context; permission
// predicates are applied in SQL before filtering, counting and paging. It sits
// between internal/control (which registers the single /data/v1/ mount and the
// console launch routes) and internal/publisher (business-database pools).
package dataapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/publisher"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/sealer"
)

// Runtime serves the App data API. Limiter and Proxies are shared with the
// console surface; Slots bounds concurrent data requests across all Tenants.
type Runtime struct {
	DB        *sql.DB
	Access    *access.Checker
	Authoring *entities.Authoring
	Publisher *publisher.Publisher
	Sealer    *sealer.Sealer
	Config    *config.Config
	Limiter   *httpapi.Limiter
	Proxies   *httpapi.TrustedProxies
	Slots     chan struct{}

	tenantDataSlots inFlightTenants
	definitions     definitionCache
}

type dataRuntime struct {
	session             AppSession
	d                   entity.Definition
	entityID, version   string
	actor               entity.Context
	access              entity.AppAccess
	permissionRevisions map[string]int64
	metadataShares      map[string]bool
}

func (s *Runtime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !s.Limiter.AllowRate("data-ip:"+s.Proxies.ClientIP(r), time.Now(), 600, 200) {
		httpapi.WriteError(w, r, httpapi.RateLimited(w))
		return
	}
	select {
	case s.Slots <- struct{}{}:
		defer func() { <-s.Slots }()
	default:
		httpapi.WriteError(w, r, httpapi.RateLimited(w))
		return
	}
	origin := r.Header.Get("Origin")
	if origin != "" {
		var n int
		e := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM apps WHERE state='active' AND approved_origin=?", origin).Scan(&n)
		if e != nil {
			httpapi.WriteError(w, r, e)
			return
		}
		if n > 0 {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
		}
	}
	if r.Method == "OPTIONS" {
		if w.Header().Get("Access-Control-Allow-Origin") == "" {
			httpapi.WriteError(w, r, registry.Problem(403, "invalid_origin", "Unregistered App origin."))
		} else {
			w.WriteHeader(204)
		}
		return
	}
	var err error
	if r.URL.Path == "/data/v1/session/exchange" && r.Method == "POST" {
		if e := s.Limiter.LimitRequest(w, r, s.Access, s.DB, access.Principal{ID: "exchange:" + s.Proxies.ClientIP(r)}); e != nil {
			httpapi.WriteError(w, r, e)
			return
		}
		err = s.Exchange(w, r)
	} else {
		var se AppSession
		se, err = s.Session(ctx, r)
		if err == nil {
			r.SetPathValue("tenant", se.TenantID)
			err = s.Limiter.LimitRequest(w, r, s.Access, s.DB, access.Principal{ID: se.UserID})
		}
		if err == nil {
			release, ok := s.tenantDataSlots.reserve(se.TenantID)
			if !ok {
				err = httpapi.RateLimited(w)
			} else {
				defer release()
				err = s.dataDispatch(w, r, se)
			}
		}
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
	}
}

func (s *Runtime) loadRuntime(ctx context.Context, se AppSession, key string) (dataRuntime, error) {
	rt := dataRuntime{session: se, permissionRevisions: map[string]int64{}}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return rt, e
	}
	defer tx.Rollback()
	var generation int64
	if e = tx.QueryRowContext(ctx, "SELECT origin_generation FROM apps WHERE id=? AND state='active'", se.AppID).Scan(&generation); e != nil || generation != se.OriginGeneration {
		return rt, registry.Problem(401, "unauthenticated", "App origin approval changed. Reconnect the App.")
	}
	rt.actor, rt.access, e = s.Authoring.ActorContext(ctx, tx, se.UserID, se.TenantID, se.SpaceID, se.AppID)
	if e != nil {
		return rt, e
	}
	var entityID, version, hash string
	e = registry.Lookup(ctx, tx, "SELECT e.id,e.active_version_id,v.definition_hash FROM entity_definitions e JOIN entity_versions v ON v.id=e.active_version_id WHERE e.tenant_id=? AND e.space_id=? AND e.entity_key=? AND e.state='active'", []any{se.TenantID, se.SpaceID, key}, &entityID, &version, &hash)
	if e != nil {
		return rt, e
	}
	cacheKey := version + ":" + hash
	var found bool
	rt.d, found = s.definitions.peek(cacheKey)
	if !found {
		var source string
		if e = tx.QueryRowContext(ctx, "SELECT definition_json FROM entity_versions WHERE id=? AND entity_id=?", version, entityID).Scan(&source); e != nil {
			return rt, e
		}
		rt.d, e = s.definitions.get(cacheKey, hash, source)
		if e != nil {
			return rt, e
		}
	}
	rt.entityID, rt.version = entityID, version
	ps, e := registry.ScanAll[publisher.ProjectedPermission](ctx, tx, "SELECT id,state,COALESCE(security_hash,''),security_revision FROM permissions WHERE entity_id=? AND state='active'", rt.entityID)
	if e != nil {
		return rt, e
	}
	for _, p := range ps {
		rt.permissionRevisions[p.ID] = p.Revision
	}
	if e = tx.Commit(); e != nil {
		return rt, e
	}
	rt.metadataShares = map[string]bool{}
	if rt.d.Sharing {
		db, release, e := s.Publisher.RuntimeDB(ctx, se.Resource)
		if e != nil {
			return rt, e
		}
		defer release()
		ids := []any{}
		for _, p := range rt.d.Permissions {
			ids = append(ids, entity.PermissionID(rt.entityID, p.Key))
		}
		if len(ids) > 0 {
			team, args := teamRecipientSQL(rt.actor)
			prefix := []any{se.TenantID, se.SpaceID, rt.entityID, rt.actor.Actor}
			prefix = append(prefix, args...)
			prefix = append(prefix, ids...)
			grants, e := registry.QueryRows(ctx, db, "SELECT DISTINCT permission_id,security_revision FROM _lastapp_record_acl acl WHERE tenant_id=? AND space_id=? AND entity_id=? AND ((recipient_kind='user' AND recipient_id=?) OR "+team+") AND permission_id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+")", prefix...)
			if e != nil {
				return rt, e
			}
			for _, g := range grants {
				pid := registry.Str(g["permission_id"])
				if registry.Integer(g["security_revision"]) == rt.permissionRevisions[pid] {
					rt.metadataShares[pid] = true
				}
			}
		}
	}
	return rt, nil
}

func (s *Runtime) dataDispatch(w http.ResponseWriter, r *http.Request, se AppSession) error {
	path := strings.TrimPrefix(r.URL.Path, "/data/v1/")
	if path == "schema" && r.Method == "GET" {
		keys, e := registry.ScanStrings(r.Context(), s.DB, "SELECT entity_key FROM entity_definitions WHERE tenant_id=? AND space_id=? AND state='active' AND active_version_id IS NOT NULL ORDER BY entity_key LIMIT 100", se.TenantID, se.SpaceID)
		if e != nil {
			return e
		}
		out := []any{}
		for _, key := range keys {
			rt, e := s.loadRuntime(r.Context(), se, key)
			if e != nil {
				return e
			}
			d := rt.d
			allowed := []entity.Field{}
			for _, f := range d.Fields {
				if rt.fieldExposed(f.Key) {
					allowed = append(allowed, f)
				}
			}
			if len(allowed) == 0 {
				continue
			}
			d.Fields = allowed
			d.Permissions = nil
			d.Rules = nil
			d.Owner = ""
			d.Team = ""
			d.Projection = slices.DeleteFunc(slices.Clone(d.Projection), func(k string) bool { return !rt.fieldExposed(k) })
			publicJSON, e := json.Marshal(d)
			if e != nil {
				return e
			}
			public := registry.Row{}
			if e = json.Unmarshal(publicJSON, &public); e != nil {
				return e
			}
			if public["fields"], e = entities.PublicEntityFields(allowed); e != nil {
				return e
			}
			out = append(out, registry.Row{"id": rt.entityID, "entity": public, "version": rt.version, "json_schema": d.JSONSchema(), "create_schema": d.Schema("create"), "update_schema": d.Schema("update"), "typescript": d.TypeScript()})
		}
		httpapi.Send(w, 200, registry.Row{"entities": out, "capabilities": entity.Capabilities()})
		return nil
	}
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "entities" {
		return registry.Missing()
	}
	rt, e := s.loadRuntime(r.Context(), se, parts[1])
	if e != nil {
		return e
	}
	switch {
	case len(parts) == 3 && parts[2] == "reference-options" && r.Method == "GET":
		return s.dataReferenceOptions(w, r, rt)
	case len(parts) == 3 && parts[2] == "query" && r.Method == "POST":
		var q dataQuery
		if e = httpapi.Decode(w, r, &q); e != nil {
			return e
		}
		return s.queryData(w, r, rt, q)
	case len(parts) == 4 && parts[2] == "records" && r.Method == "GET":
		selects := []string{}
		if v := r.URL.Query().Get("select"); v != "" {
			selects = strings.Split(v, ",")
		}
		return s.queryData(w, r, rt, dataQuery{Select: selects, Filter: &entity.Expr{Op: "eq", Args: []entity.Expr{{Field: "id"}, {Literal: parts[3]}}}, Limit: 1, single: true})
	case len(parts) == 3 && parts[2] == "records" && r.Method == "POST":
		var in dataMutation
		if e = httpapi.Decode(w, r, &in); e != nil {
			return e
		}
		in.Operation = "create"
		return s.mutateData(w, r, rt, []dataMutation{in})
	case len(parts) == 4 && parts[2] == "records" && (r.Method == "PATCH" || r.Method == "DELETE"):
		in := dataMutation{ID: parts[3], Operation: "delete"}
		if r.Method == "PATCH" {
			if e = httpapi.Decode(w, r, &in); e != nil {
				return e
			}
			in.ID = parts[3]
			in.Operation = "update"
		}
		in.Revision, e = httpapi.ExpectedRevision(r)
		if e != nil {
			return e
		}
		return s.mutateData(w, r, rt, []dataMutation{in})
	case len(parts) == 3 && parts[2] == "bulk" && r.Method == "POST":
		var in struct {
			Items []dataMutation `json:"items"`
		}
		if e = httpapi.Decode(w, r, &in); e != nil {
			return e
		}
		return s.mutateData(w, r, rt, in.Items)
	case len(parts) == 3 && parts[2] == "capabilities" && r.Method == "POST":
		return s.dataCapabilities(w, r, rt)
	case len(parts) == 5 && parts[2] == "records" && parts[4] == "shares":
		return s.dataShares(w, r, rt, parts[3])
	}
	return registry.Missing()
}

func (rt dataRuntime) fieldExposed(k string) bool {
	for _, p := range rt.d.Permissions {
		if !slices.Contains(p.Fields, k) && !((k == "id" || k == "revision") && p.Operation == "read") {
			continue
		}
		c := rt.actor
		if (len(p.Roles) > 0 || slices.Contains([]string{"member", "user", "team"}, p.Relationship)) && !rt.d.Activated(p, c) && !rt.metadataShares[entity.PermissionID(rt.entityID, p.Key)] {
			continue
		}
		id := entity.PermissionID(rt.entityID, p.Key) // relationship conditions are record-dependent; metadata reveals no values
		if rt.access.Inherit || gateHas(rt.access.Ceiling, id, k, rt.actor) {
			if !rt.access.Restricted || gateHas(rt.access.Restrictions, id, k, rt.actor) {
				return true
			}
		}
	}
	return false
}

func gateHas(gs []entity.Gate, id, k string, c entity.Context) bool {
	for _, g := range gs {
		if g.Permission == id && (slices.Contains(g.Fields, k) || k == "id" || k == "revision") {
			if len(g.Roles) > 0 {
				found := false
				for _, role := range g.Roles {
					found = found || c.Roles[role]
				}
				if !found {
					continue
				}
			}
			return true
		}
	}
	return false
}

func (rt dataRuntime) shared(ctx context.Context, q registry.Queryer, record string) (map[string]bool, error) {
	out := map[string]bool{}
	if !rt.d.Sharing {
		return out, nil
	}
	team, args := teamRecipientSQL(rt.actor)
	args = append([]any{rt.session.TenantID, rt.session.SpaceID, rt.entityID, record, rt.actor.Actor}, args...)
	vs, e := registry.QueryRows(ctx, q, "SELECT permission_id,security_revision FROM _lastapp_record_acl acl WHERE tenant_id=? AND space_id=? AND entity_id=? AND record_id=? AND ((recipient_kind='user' AND recipient_id=?) OR "+team+")", args...)
	if e != nil {
		return nil, e
	}
	for _, v := range vs {
		pid := registry.Str(v["permission_id"])
		if registry.Integer(v["security_revision"]) == rt.permissionRevisions[pid] {
			out[pid] = true
		}
	}
	return out, nil
}

func (rt dataRuntime) checkGate(ctx context.Context, q registry.Queryer, lock bool) error {
	suffix := ""
	if lock {
		suffix = " LOCK IN SHARE MODE"
	}
	var version string
	var open bool
	e := registry.Lookup(ctx, q, "SELECT COALESCE(version_id,''),gate_open FROM _lastapp_entity_state WHERE entity_id=? AND tenant_id=? AND space_id=?"+suffix, []any{rt.entityID, rt.session.TenantID, rt.session.SpaceID}, &version, &open)
	if e != nil {
		return e
	}
	if !open || version != rt.version {
		return registry.Problem(409, "publication_in_progress", "Entity definition changed or is publishing. Refresh and retry.")
	}
	return nil
}
