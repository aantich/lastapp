package control

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type pageInfo struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	HasMore bool `json:"has_more"`
}
type pageContextKey struct{}

// PageRequest is a bounded collection request. Zero Page/Limit select the
// defaults; any other value outside the bounds is rejected by info().
type PageRequest struct {
	Page, Limit int
	Search      string
}

// Page is one bounded page of a collection.
type Page[T any] struct {
	Items      []T      `json:"items"`
	Pagination pageInfo `json:"pagination"`
}

// pageRequest reads page, limit and q from a query string. Unparseable or
// non-positive explicit values become -1 so that info() rejects them.
func pageRequest(values url.Values) PageRequest {
	number := func(key string) int {
		if !values.Has(key) {
			return 0
		}
		n, e := strconv.Atoi(values.Get(key))
		if e != nil || n < 1 {
			return -1
		}
		return n
	}
	return PageRequest{Page: number("page"), Limit: number("limit"), Search: values.Get("q")}
}

// Every collection is bounded, including default reads. Permission predicates
// stay inside the original query, before search and LIMIT/OFFSET.
func (r PageRequest) info() (pageInfo, string, error) {
	info := pageInfo{Page: 1, Limit: 100}
	if r.Page != 0 {
		if r.Page < 1 || r.Page > 10000 {
			return info, "", bad("Page must be between 1 and 10000.")
		}
		info.Page = r.Page
	}
	if r.Limit != 0 {
		if r.Limit < 1 || r.Limit > 200 {
			return info, "", bad("Limit must be between 1 and 200.")
		}
		info.Limit = r.Limit
	}
	if len(r.Search) > 120 {
		return info, "", bad("Search must be at most 120 bytes.")
	}
	return info, r.Search, nil
}

// collection names a paged collection's searchable output columns and its
// stable order. Only these static names enter SQL; search text is bound.
type collection struct{ columns, order string }

var defaultCollection = collection{"id,display_name", "display_name,id"}

// collections maps each collection key to its search columns and order.
var collections = map[string]collection{
	"me":                  {"id,display_name,slug,state", "display_name,id"},
	"my-tenants":          {"id,display_name,slug,state", "display_name,id"},
	"tenants":             {"id,display_name,slug,state", "display_name,id"},
	"plans":               defaultCollection,
	"database-resources":  {"id,database_name,state", "created_at DESC,id"},
	"my-role-assignments": {"id,display_name,code,scope_kind,scope_team_id,space_id,app_id", "scope_kind,id"},
	"role-assignments":    {"id,principal_name,access_name,scope_name", "created_at DESC,id"},
	"memberships":         {"user_id,display_name,email,state", "display_name,user_id"},
	"administrators":      {"id,user_id,display_name,email,state", "display_name,id"},
	"recipients":          {"id,user_id,display_name,email", "display_name,id"},
	"team-recipients":     defaultCollection,
	"members":             {"user_id,display_name", "display_name,user_id"},
	"audit-events":        {"id,action,resource_type,resource_id,actor_name,details_json", "id DESC"},
	"roles":               {"id,display_name,code,state", "protected DESC,display_name,id"},
	"builds":              {"id,content_hash,entrypoint", "created_at DESC,id"},
	"releases":            {"id,display_name", "created_at DESC,id"},
	"workspaces":          {"id,display_name", "created_at DESC,id"},
	"spaces":              {"id,display_name,slug,state", "created_at DESC,id"},
	"apps":                {"id,display_name,slug,state", "created_at DESC,id"},
	"microapps":           {"id,display_name,slug,state", "navigation_order,created_at DESC,id"},
	"teams":               {"id,display_name,slug,state", "display_name,id"},
	"users":               {"id,display_name,email,oidc_issuer,oidc_subject,state", "display_name,id"},
	"agent-access":        {"id,client_name,client_id,scope_kind,state", "created_at DESC,id"},
}

func collectionFor(key string) collection {
	if c, ok := collections[key]; ok {
		return c
	}
	return defaultCollection
}

// pageQuery runs one bounded page of query, an already permission-filtered
// statement, searching and ordering by the collection's static columns.
func pageQuery(ctx context.Context, q Queryer, req PageRequest, c collection, query string, args ...any) (Page[Row], error) {
	info, query, args, e := pageSQL(req, c, query, args)
	if e != nil {
		return Page[Row]{}, e
	}
	v, e := rows(ctx, q, query, args...)
	if e != nil {
		return Page[Row]{}, e
	}
	return pageOf(info, v), nil
}

// pageScan is pageQuery for a typed registry record.
func pageScan[T any, P record[T]](ctx context.Context, q Queryer, req PageRequest, c collection, query string, args ...any) (Page[T], error) {
	info, query, args, e := pageSQL(req, c, query, args)
	if e != nil {
		return Page[T]{}, e
	}
	v, e := scanAll[T, P](ctx, q, query, args...)
	if e != nil {
		return Page[T]{}, e
	}
	return pageOf(info, v), nil
}

// pageSQL wraps query in the collection's search, order and one-extra LIMIT.
func pageSQL(req PageRequest, c collection, query string, args []any) (pageInfo, string, []any, error) {
	info, search, e := req.info()
	if e != nil {
		return info, "", nil, e
	}
	query = "SELECT * FROM (" + query + ") AS collection"
	if search != "" {
		search = strings.NewReplacer("=", "==", "%", "=%", "_", "=_").Replace(search)
		query += " WHERE CONCAT_WS(' '," + c.columns + ") LIKE ? ESCAPE '='"
		args = append(args, "%"+search+"%")
	}
	query += " ORDER BY " + c.order + " LIMIT ? OFFSET ?"
	args = append(args, info.Limit+1, (info.Page-1)*info.Limit)
	return info, query, args, nil
}

// pageOf cuts the one-extra row that reports has_more.
func pageOf[T any](info pageInfo, v []T) Page[T] {
	info.HasMore = len(v) > info.Limit
	if info.HasMore {
		v = v[:info.Limit]
	}
	return Page[T]{Items: v, Pagination: info}
}

// pageRows is the shim for handlers not yet migrated to operations: it takes
// the collection key from the request path and reports paging through ctx.
func pageRows(ctx context.Context, q Queryer, r *http.Request, query string, args ...any) ([]Row, error) {
	values := r.URL.Query()
	key := path.Base(r.URL.Path)
	if key == "recipients" && values.Get("type") == "team" {
		key = "team-recipients"
	}
	page, e := pageQuery(ctx, q, pageRequest(values), collectionFor(key), query, args...)
	if e != nil {
		return nil, e
	}
	if dest, ok := ctx.Value(pageContextKey{}).(*pageInfo); ok {
		*dest = page.Pagination
	}
	return page.Items, nil
}

func sendPage(w http.ResponseWriter, r *http.Request, v []Row) {
	info, _ := r.Context().Value(pageContextKey{}).(*pageInfo)
	send(w, 200, Row{"items": v, "pagination": info})
}

// Only pass already-authorized metadata here. Filtering must precede paging;
// serialize the resulting page rather than an oversized complete inventory.
func pageItems(items []Row, req PageRequest) ([]Row, pageInfo, error) {
	info, search, err := req.info()
	if err != nil {
		return nil, info, err
	}
	filtered := []Row{}
	for _, v := range items {
		if search != "" {
			text := strings.ToLower(str(v["id"]) + " " + str(v["label"]) + " " + str(v["display_name"]) + " " + str(v["key"]) + " " + str(v["entity_key"]) + " " + str(v["code"]) + " " + str(v["description"]))
			if !strings.Contains(text, strings.ToLower(search)) {
				continue
			}
		}
		filtered = append(filtered, v)
	}
	start := min((info.Page-1)*info.Limit, len(filtered))
	end := min(start+info.Limit, len(filtered))
	info.HasMore = end < len(filtered)
	return filtered[start:end], info, nil
}
