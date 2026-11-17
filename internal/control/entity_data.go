package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

type dataRuntime struct {
	session             appSession
	d                   entity.Definition
	entityID, version   string
	actor               entity.Context
	access              entity.AppAccess
	permissionRevisions map[string]int64
	metadataShares      map[string]bool
}

func (s *Server) dataHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !s.limiter.AllowRate("data-ip:"+s.trustedProxies.ClientIP(r), time.Now(), 600, 200) {
		httpapi.WriteError(w, r, httpapi.RateLimited(w))
		return
	}
	select {
	case s.dataSlots <- struct{}{}:
		defer func() { <-s.dataSlots }()
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
		if e := s.limiter.LimitRequest(w, r, s.Checker, s.DB, access.Principal{ID: "exchange:" + s.trustedProxies.ClientIP(r)}); e != nil {
			httpapi.WriteError(w, r, e)
			return
		}
		err = s.exchange(w, r)
	} else {
		var se appSession
		se, err = s.dataSession(ctx, r)
		if err == nil {
			r.SetPathValue("tenant", se.TenantID)
			err = s.limiter.LimitRequest(w, r, s.Checker, s.DB, access.Principal{ID: se.UserID})
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

func (s *Server) loadRuntime(ctx context.Context, se appSession, key string) (dataRuntime, error) {
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
	rt.actor, rt.access, e = s.actorContext(ctx, tx, se.UserID, se.TenantID, se.SpaceID, se.AppID)
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
	ps, e := registry.ScanAll[projectedPermission](ctx, tx, "SELECT id,state,COALESCE(security_hash,''),security_revision FROM permissions WHERE entity_id=? AND state='active'", rt.entityID)
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
		db, release, e := s.runtimeDB(ctx, se.Resource)
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

func (s *Server) dataDispatch(w http.ResponseWriter, r *http.Request, se appSession) error {
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

func businessRecord(d entity.Definition, raw registry.Row) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		if strings.HasPrefix(k, "f_") {
			k = k[2:]
		} else if !slices.Contains([]string{"id", "revision", "created_at", "updated_at", "created_by", "updated_by"}, k) {
			continue
		}
		if f, ok := d.Field(k); ok && v != nil {
			switch f.Type {
			case "integer":
				v = registry.Integer(v)
			case "boolean":
				v = registry.Integer(v) != 0
			case "json":
				var obj any
				if json.Unmarshal([]byte(registry.Str(v)), &obj) == nil {
					v = obj
				}
			}
		}
		out[k] = v
	}
	return out
}

func (rt dataRuntime) shareSQL(p entity.Permission) (string, []any) {
	if !rt.d.Sharing {
		return "1=0", nil
	}
	pid := entity.PermissionID(rt.entityID, p.Key)
	team, args := teamRecipientSQL(rt.actor)
	s := "EXISTS(SELECT 1 FROM _lastapp_record_acl acl WHERE acl.tenant_id=r.tenant_id AND acl.space_id=r.space_id AND acl.entity_id=? AND acl.record_id=r.id AND acl.permission_id=? AND acl.security_revision=? AND ((acl.recipient_kind='user' AND acl.recipient_id=?) OR " + team + "))"
	return s, append([]any{rt.entityID, pid, rt.permissionRevisions[pid], rt.actor.Actor}, args...)
}

func teamRecipientSQL(c entity.Context) (string, []any) {
	keys := []string{}
	for k := range c.Teams {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		return "1=0", nil
	}
	args := []any{}
	for _, k := range keys {
		args = append(args, k)
	}
	return "(acl.recipient_kind='team' AND acl.recipient_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",") + "))", args
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

type dataSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}
type dataQuery struct {
	Select []string     `json:"select"`
	Filter *entity.Expr `json:"filter,omitempty"`
	Sort   []dataSort   `json:"sort,omitempty"`
	Limit  int          `json:"limit,omitempty"`
	Cursor string       `json:"cursor,omitempty"`
	Count  bool         `json:"include_count,omitempty"`
	single bool
}
type dataCursor struct {
	Hash   string
	Values []any
}

func (s *Server) queryData(w http.ResponseWriter, r *http.Request, rt dataRuntime, q dataQuery) error {
	defaultPage := q.Limit == 0
	inaccessibleField := func() error {
		if q.single {
			return registry.Missing()
		}
		return registry.Bad("Unknown or inaccessible query field.")
	}
	if q.Limit == 0 {
		q.Limit = entity.Limits.Page
	}
	if q.Limit < 1 || q.Limit > entity.Limits.MaxPage || len(q.Sort) > 4 || len(q.Select) > entity.Limits.Fields {
		return registry.Bad("Query exceeds supported limits.")
	}
	if len(q.Select) == 0 {
		q.Select = slices.Clone(rt.d.Projection)
	}
	if len(q.Select) == 0 {
		return registry.Bad("Select fields explicitly; this entity has no default projection.")
	}
	q.Select = append([]string{"id", "revision"}, q.Select...)
	q.Select = uniqueStrings(q.Select)
	needed := slices.Clone(q.Select)
	if q.Filter != nil {
		for _, key := range entity.Fields(*q.Filter) {
			if !rt.fieldExposed(key) {
				return inaccessibleField()
			}
		}
		if e := rt.d.CheckCondition(*q.Filter, false); e != nil {
			return registry.Bad(e.Error())
		}
		for _, f := range entity.Fields(*q.Filter) {
			def, _ := rt.d.Field(f)
			if !def.Filterable {
				return registry.Bad("Field is not filterable: " + f)
			}
			needed = append(needed, f)
		}
	}
	for _, so := range q.Sort {
		if !rt.fieldExposed(so.Field) {
			return inaccessibleField()
		}
		f, ok := rt.d.Field(so.Field)
		if !ok || !f.Sortable || (so.Direction != "asc" && so.Direction != "desc") {
			return registry.Bad("Unsupported sort field or direction.")
		}
		needed = append(needed, so.Field)
	}
	if !slices.ContainsFunc(q.Sort, func(x dataSort) bool { return x.Field == "id" }) {
		q.Sort = append(q.Sort, dataSort{"id", "asc"})
	}
	for _, k := range needed {
		if !rt.fieldExposed(k) {
			return inaccessibleField()
		}
		if _, ok := rt.d.Field(k); !ok {
			return registry.Bad("Unknown selected field.")
		}
	}
	widthFields := []entity.Field{}
	for _, key := range uniqueStrings(append(slices.Clone(q.Select), sortFields(q.Sort)...)) {
		field, _ := rt.d.Field(key)
		widthFields = append(widthFields, field)
	}
	maximum := entity.MaxReadResponseBytes / entity.EstimatedRecordBytes(widthFields)
	if maximum < 1 {
		maximum = 1
	}
	if q.Limit > maximum {
		if defaultPage {
			q.Limit = maximum
		} else {
			return registry.Bad("Selected fields exceed the response budget. Reduce the page size or select fewer fields.")
		}
	}
	pred, args, e := rt.d.ReadSQL(rt.entityID, uniqueStrings(needed), rt.actor, rt.access, rt.shareSQL)
	if e != nil {
		return registry.Bad(e.Error())
	}
	user, ua, e := rt.d.ExprSQL(q.Filter, rt.actor)
	if e != nil {
		return registry.Bad(e.Error())
	}
	where := "r.tenant_id=? AND r.space_id=? AND " + pred + " AND " + user
	args = append([]any{rt.session.TenantID, rt.session.SpaceID}, args...)
	args = append(args, ua...)
	if len(args) > entity.MaxSQLParameters {
		return registry.Bad("Query exceeds its parameter limit.")
	}
	db, release, e := s.runtimeDB(r.Context(), rt.session.Resource)
	if e != nil {
		return e
	}
	defer release()
	tx, e := db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = rt.checkGate(r.Context(), tx, false); e != nil {
		return e
	}
	out := registry.Row{}
	if q.Count {
		var count int64
		if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+data.Table(rt.entityID)+" r WHERE "+where, args...).Scan(&count); e != nil {
			return e
		}
		out["count"] = count
	}
	fingerprint := q
	fingerprint.Cursor = ""
	hash := entity.Hash([]any{fingerprint, rt.entityID, rt.version, rt.session.AppID, rt.actor.Actor})
	if q.Cursor != "" {
		b, e := base64Decode(q.Cursor)
		if e != nil {
			return registry.Bad("Invalid cursor.")
		}
		var cur dataCursor
		if e = s.sealer.Unseal("data_cursor", rt.entityID, rt.session.TenantID, b, &cur); e != nil || cur.Hash != hash || len(cur.Values) != len(q.Sort) {
			return registry.Bad("Cursor does not match this query or definition.")
		}
		cs, ca := cursorSQL(rt.d, q.Sort, cur.Values)
		where += " AND " + cs
		args = append(args, ca...)
		if len(args) > entity.MaxSQLParameters {
			return registry.Bad("Query exceeds its parameter limit.")
		}
	}
	cols := []string{}
	for _, k := range uniqueStrings(append(slices.Clone(q.Select), sortFields(q.Sort)...)) {
		cols = append(cols, entity.Column(k))
	}
	order := []string{}
	for _, so := range q.Sort {
		col := entity.Column(so.Field)
		order = append(order, "("+col+" IS NULL) ASC", col+" "+strings.ToUpper(so.Direction))
	}
	vs, e := registry.QueryRows(r.Context(), tx, "SELECT "+strings.Join(cols, ",")+" FROM "+data.Table(rt.entityID)+" r WHERE "+where+" ORDER BY "+strings.Join(order, ",")+" LIMIT ?", append(args, q.Limit+1)...)
	if e != nil {
		return e
	}
	more := len(vs) > q.Limit
	if more {
		vs = vs[:q.Limit]
	}
	items := []any{}
	for _, v := range vs {
		full := businessRecord(rt.d, v)
		projected := map[string]any{}
		for _, k := range q.Select {
			projected[k] = full[k]
		}
		items = append(items, projected)
	}
	if more {
		last := businessRecord(rt.d, vs[len(vs)-1])
		values := []any{}
		for _, so := range q.Sort {
			values = append(values, last[so.Field])
		}
		b, e := s.sealer.Seal("data_cursor", rt.entityID, rt.session.TenantID, dataCursor{hash, values})
		if e != nil {
			return e
		}
		out["next_cursor"] = base64Encode(b)
	}
	out["items"] = items
	out["version"] = rt.version
	if e = tx.Commit(); e != nil {
		return e
	}
	if q.single {
		if len(items) == 0 {
			return registry.Missing()
		}
		httpapi.Send(w, 200, items[0])
	} else {
		httpapi.Send(w, 200, out)
	}
	return nil
}

func uniqueStrings(v []string) []string {
	out := []string{}
	for _, k := range v {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

func sortFields(v []dataSort) []string {
	out := []string{}
	for _, s := range v {
		out = append(out, s.Field)
	}
	return out
}

func cursorSQL(d entity.Definition, order []dataSort, values []any) (string, []any) {
	parts := []string{}
	args := []any{}
	placeholder := func(key string) string {
		f, _ := d.Field(key)
		if f.Type == "decimal" {
			return fmt.Sprintf("CAST(? AS DECIMAL(%d,%d))", f.Precision, f.Scale)
		}
		return "?"
	}
	for i, so := range order {
		eq := []string{}
		ea := []any{}
		for j := 0; j < i; j++ {
			eq = append(eq, entity.Column(order[j].Field)+" <=> "+placeholder(order[j].Field))
			ea = append(ea, values[j])
		}
		if values[i] == nil {
			continue
		}
		col := entity.Column(so.Field)
		op := ">"
		if so.Direction == "desc" {
			op = "<"
		}
		eq = append(eq, "("+col+" IS NULL OR "+col+op+placeholder(so.Field)+")")
		ea = append(ea, values[i])
		parts = append(parts, "("+strings.Join(eq, " AND ")+")")
		args = append(args, ea...)
	}
	if len(parts) == 0 {
		return "1=0", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

type dataMutation struct {
	Operation string         `json:"operation,omitempty"`
	ID        string         `json:"id"`
	Revision  int64          `json:"revision,omitempty"`
	Values    map[string]any `json:"values,omitempty"`
}

func (s *Server) mutateData(w http.ResponseWriter, r *http.Request, rt dataRuntime, items []dataMutation) error {
	if len(items) == 0 || len(items) > entity.Limits.Bulk {
		return registry.Bad("Invalid bulk size.")
	}
	seen := map[string]bool{}
	for i := range items {
		for key, value := range items[i].Values {
			field, ok := rt.d.Field(key)
			if !ok || !rt.fieldExposed(key) || field.Calculation != nil || slices.Contains([]string{"id", "revision", "created_at", "updated_at", "created_by", "updated_by"}, key) {
				return registry.Bad("Unknown or read-only input field.")
			}
			if _, e := entity.Value(field, value, false); e != nil {
				return e
			}
		}
		if items[i].ID == "" && items[i].Operation == "create" {
			items[i].ID = registry.NewID()
		}
		if e := registry.ID(items[i].ID); e != nil {
			return e
		}
		if seen[items[i].ID] {
			return registry.Bad("Duplicate record ID in bulk.")
		}
		seen[items[i].ID] = true
		if !slices.Contains([]string{"create", "update", "delete"}, items[i].Operation) {
			return registry.Bad("Unsupported mutation.")
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	db, release, e := s.runtimeDB(r.Context(), rt.session.Resource)
	if e != nil {
		return e
	}
	defer release()
	tx, e := db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	lockIDs := []string{rt.entityID}
	for _, f := range rt.d.Fields {
		if f.Type == "reference" {
			lockIDs = append(lockIDs, f.Target)
		}
	}
	lockIDs = uniqueStrings(lockIDs)
	slices.Sort(lockIDs)
	for _, key := range lockIDs {
		var open bool
		if e := registry.Lookup(r.Context(), tx, "SELECT gate_open FROM _lastapp_entity_state WHERE entity_id=? AND tenant_id=? AND space_id=? LOCK IN SHARE MODE", []any{key, rt.session.TenantID, rt.session.SpaceID}, &open); e != nil {
			return e
		}
		if !open {
			return registry.Problem(409, "publication_in_progress", "A referenced entity is publishing. Retry shortly.")
		}
	}
	if e = rt.checkGate(r.Context(), tx, false); e != nil {
		return e
	}
	out := []any{}
	for _, in := range items {
		c := rt.actor
		old := map[string]any{}
		create := in.Operation == "create"
		shared := map[string]bool{}
		if !create {
			raw, e := registry.QueryRow(r.Context(), tx, "SELECT * FROM "+data.Table(rt.entityID)+" WHERE tenant_id=? AND space_id=? AND id=? FOR UPDATE", rt.session.TenantID, rt.session.SpaceID, in.ID)
			if e != nil {
				return e
			}
			old = businessRecord(rt.d, raw)
			shared, e = rt.shared(r.Context(), tx, in.ID)
			if e != nil {
				return e
			}
			c.Record = old
			c.Old = old

		}
		c.Record = old
		c.Old = old

		candidate := old
		fields := []string{}
		for k := range in.Values {
			fields = append(fields, k)
		}
		if in.Operation != "delete" {
			nextRevision := in.Revision + 1
			if create {
				nextRevision = 1
			}
			candidate, e = rt.d.Prepare(c, in.Values, create, in.ID, nextRevision)
			if e != nil {
				if !create {
					readable := true
					for _, f := range rt.d.Fields {
						readable = readable && rt.d.Allowed(rt.entityID, "read", []string{f.Key}, c, rt.access, shared)
					}
					if !readable {
						return registry.Missing()
					}
				}
				if _, ok := e.(entity.Diagnostic); !ok {
					return registry.Bad("Calculated values could not be produced for this change.")
				}
				return e
			}
		}

		c.Record = candidate
		if !rt.d.Allowed(rt.entityID, in.Operation, fields, c, rt.access, shared) {
			if !create {
				return registry.Missing()
			}
			return registry.Problem(403, "forbidden", "This change is not permitted.")
		}
		if !create {
			if in.Revision < 1 {
				return registry.Problem(428, "revision_required", "Every update/delete needs its current revision.")
			}
			if registry.Integer(old["revision"]) != in.Revision {
				return registry.Problem(412, "stale_revision", "Record changed. Refresh before saving.")
			}
		}
		if in.Operation != "delete" {
			if e = rt.d.CheckRules(c, create); e != nil {
				return e
			}
			if !create && ((rt.d.Owner != "" && old[rt.d.Owner] != candidate[rt.d.Owner]) || (rt.d.Team != "" && old[rt.d.Team] != candidate[rt.d.Team])) {
				if !rt.d.Allowed(rt.entityID, "acl.manage", nil, c, rt.access, shared) {
					return registry.Problem(403, "forbidden", "Managing access is required to change ownership.")
				}
				if _, e = tx.ExecContext(r.Context(), "DELETE FROM _lastapp_record_acl WHERE entity_id=? AND record_id=?", rt.entityID, in.ID); e != nil {
					return e
				}
			}
			if e = s.checkDataReferences(r.Context(), tx, rt, old, candidate); e != nil {
				return e
			}
		}
		revision := in.Revision + 1
		if create {
			revision = 1
		}
		stamp := c.Now.UTC().Format("2006-01-02T15:04:05.000000Z")
		if in.Operation == "delete" {
			_, e = tx.ExecContext(r.Context(), "DELETE FROM "+data.Table(rt.entityID)+" WHERE tenant_id=? AND space_id=? AND id=?", rt.session.TenantID, rt.session.SpaceID, in.ID)
			if e == nil {
				_, e = tx.ExecContext(r.Context(), "DELETE FROM _lastapp_record_acl WHERE entity_id=? AND record_id=?", rt.entityID, in.ID)
			}
		} else {
			cols := []string{}
			args := []any{}
			for _, f := range rt.d.Fields {
				cols = append(cols, data.FieldColumn(f.Key))
				val := candidate[f.Key]
				if f.Type == "json" && val != nil {
					b, _ := json.Marshal(val)
					val = string(b)
				}
				args = append(args, val)
			}
			if create {
				cols = append(cols, "id", "tenant_id", "space_id", "revision", "created_at", "updated_at", "created_by", "updated_by")
				args = append(args, in.ID, rt.session.TenantID, rt.session.SpaceID, revision, stamp, stamp, c.Actor, c.Actor)
				_, e = tx.ExecContext(r.Context(), "INSERT INTO "+data.Table(rt.entityID)+" ("+strings.Join(cols, ",")+") VALUES("+strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")+")", args...)
			} else {
				sets := []string{}
				for _, col := range cols {
					sets = append(sets, col+"=?")
				}
				sets = append(sets, "revision=?", "updated_at=?", "updated_by=?")
				args = append(args, revision, stamp, c.Actor, rt.session.TenantID, rt.session.SpaceID, in.ID)
				_, e = tx.ExecContext(r.Context(), "UPDATE "+data.Table(rt.entityID)+" SET "+strings.Join(sets, ",")+" WHERE tenant_id=? AND space_id=? AND id=?", args...)
			}
		}
		if e != nil {
			return e
		}
		changed := []string{}
		for _, f := range rt.d.Fields {
			if create || in.Operation == "delete" || entity.Hash(old[f.Key]) != entity.Hash(candidate[f.Key]) {
				changed = append(changed, f.Key)
			}
		}
		e = businessAudit(r.Context(), tx, rt, in.ID, in.Operation, revision, changed, r.Header.Get("X-Request-ID"))
		if e != nil {
			return e
		}
		out = append(out, registry.Row{"id": in.ID, "revision": revision})
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	httpapi.Send(w, 200, registry.Row{"items": out})
	return nil
}

func (s *Server) checkDataReferences(ctx context.Context, tx *sql.Tx, rt dataRuntime, old, next map[string]any) error {
	refs := slices.Clone(rt.d.Fields)
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Target+fmt.Sprint(next[refs[i].Key]) < refs[j].Target+fmt.Sprint(next[refs[j].Key])
	})
	for _, f := range refs {
		v := next[f.Key]
		if v == nil || fmt.Sprint(v) == fmt.Sprint(old[f.Key]) {
			continue
		}
		if f.Type == "user" || f.Type == "team" {
			query := "SELECT COUNT(*) FROM groups WHERE tenant_id=? AND id=? AND state='active'"
			if f.Type == "user" {
				query = "SELECT COUNT(*) FROM tenant_memberships m JOIN users u ON u.id=m.user_id AND u.state='active' WHERE m.tenant_id=? AND m.user_id=? AND m.state='active'"
			}
			var n int
			if e := s.DB.QueryRowContext(ctx, query, rt.session.TenantID, v).Scan(&n); e != nil {
				return e
			}
			if n != 1 {
				return registry.Bad("Unavailable user/team reference.")
			}
		}
		if f.Type == "reference" {
			var targetKey string
			if e := registry.Lookup(ctx, s.DB, "SELECT entity_key FROM entity_definitions WHERE id=? AND space_id=? AND state='active'", []any{f.Target, rt.session.SpaceID}, &targetKey); e != nil {
				return registry.Bad("Unavailable reference.")
			}
			target, e := s.loadRuntime(ctx, rt.session, targetKey)
			if e != nil {
				return e
			}
			if e = target.checkGate(ctx, tx, true); e != nil {
				return e
			}
			pred, args, e := target.d.ReadSQL(target.entityID, []string{"id"}, target.actor, target.access, target.shareSQL)
			if e != nil {
				return e
			}
			args = append([]any{rt.session.TenantID, rt.session.SpaceID, v}, args...)
			var found string
			if e = tx.QueryRowContext(ctx, "SELECT r.id FROM "+data.Table(target.entityID)+" r WHERE r.tenant_id=? AND r.space_id=? AND r.id=? AND "+pred+" LOCK IN SHARE MODE", args...).Scan(&found); e != nil {
				return registry.Bad("Unavailable reference.")
			}
		}
	}
	return nil
}

func businessAudit(ctx context.Context, q registry.Queryer, rt dataRuntime, record, action string, revision int64, fields []string, request string) error {
	if fields == nil {
		fields = []string{}
	}
	slices.Sort(fields)
	b, _ := json.Marshal(fields)
	if _, e := q.ExecContext(ctx, "INSERT INTO _lastapp_data_audit(tenant_id,space_id,entity_id,record_id,actor_id,action,revision,fields_json,request_id) VALUES(?,?,?,?,?,?,?,?,?)", rt.session.TenantID, rt.session.SpaceID, rt.entityID, record, rt.actor.Actor, action, revision, b, request); e != nil {
		return e
	}
	_, e := q.ExecContext(ctx, "DELETE FROM _lastapp_data_audit WHERE entity_id=? AND created_at<UTC_TIMESTAMP(6)-INTERVAL 90 DAY LIMIT 100", rt.entityID)
	return e
}
