package dataapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

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

func (s *Runtime) queryData(w http.ResponseWriter, r *http.Request, rt dataRuntime, q dataQuery) error {
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
	db, release, e := s.Publisher.RuntimeDB(r.Context(), rt.session.Resource)
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
		if e = s.Sealer.Unseal("data_cursor", rt.entityID, rt.session.TenantID, b, &cur); e != nil || cur.Hash != hash || len(cur.Values) != len(q.Sort) {
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
		full := entities.BusinessRecord(rt.d, v)
		projected := map[string]any{}
		for _, k := range q.Select {
			projected[k] = full[k]
		}
		items = append(items, projected)
	}
	if more {
		last := entities.BusinessRecord(rt.d, vs[len(vs)-1])
		values := []any{}
		for _, so := range q.Sort {
			values = append(values, last[so.Field])
		}
		b, e := s.Sealer.Seal("data_cursor", rt.entityID, rt.session.TenantID, dataCursor{hash, values})
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

type dataMutation struct {
	Operation string         `json:"operation,omitempty"`
	ID        string         `json:"id"`
	Revision  int64          `json:"revision,omitempty"`
	Values    map[string]any `json:"values,omitempty"`
}

func (s *Runtime) mutateData(w http.ResponseWriter, r *http.Request, rt dataRuntime, items []dataMutation) error {
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
	db, release, e := s.Publisher.RuntimeDB(r.Context(), rt.session.Resource)
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
			old = entities.BusinessRecord(rt.d, raw)
			shared, e = rt.shared(r.Context(), tx, in.ID)
			if e != nil {
				return e
			}
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

func (s *Runtime) checkDataReferences(ctx context.Context, tx *sql.Tx, rt dataRuntime, old, next map[string]any) error {
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
