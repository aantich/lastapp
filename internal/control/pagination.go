package control

import (
	"context"
	"net/http"
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

// Every collection is bounded, including default reads. Permission predicates
// stay inside the original query, before search and LIMIT/OFFSET.
func pageRows(ctx context.Context, q Queryer, r *http.Request, query string, args ...any) ([]Row, error) {
	values := r.URL.Query()
	info := pageInfo{Page: 1, Limit: 100}
	var e error
	if values.Has("page") {
		info.Page, e = strconv.Atoi(values.Get("page"))
		if e != nil || info.Page < 1 || info.Page > 10000 {
			return nil, bad("Page must be between 1 and 10000.")
		}
	}
	if values.Has("limit") {
		info.Limit, e = strconv.Atoi(values.Get("limit"))
		if e != nil || info.Limit < 1 || info.Limit > 200 {
			return nil, bad("Limit must be between 1 and 200.")
		}
	}
	search := values.Get("q")
	if len(search) > 120 {
		return nil, bad("Search must be at most 120 bytes.")
	}
	// Only static, output-column names enter SQL. All user search text is bound.
	key := path.Base(r.URL.Path)
	columns, order := "id,display_name", "display_name,id"
	switch key {
	case "me", "my-tenants", "tenants":
		columns, order = "id,display_name,slug,state", "display_name,id"
	case "database-resources":
		columns, order = "id,database_name,state", "created_at DESC,id"
	case "access-grants":
		columns, order = "id,principal_name,access_name,scope_name", "created_at DESC,id"
	case "memberships":
		columns, order = "user_id,display_name,email,state", "display_name,user_id"
	case "members":
		columns, order = "user_id,display_name", "display_name,user_id"
	case "audit-events":
		columns, order = "id,action,resource_type,resource_id,actor_name,details_json", "id DESC"
	case "roles":
		columns, order = "id,display_name,code,state", "protected DESC,display_name,id"
	case "spaces", "apps":
		columns, order = "id,display_name,slug,state", "created_at DESC,id"
	case "microapps":
		columns, order = "id,display_name,slug,state", "navigation_order,created_at DESC,id"
	case "groups":
		columns = "id,display_name,slug,state"
	case "users":
		columns = "id,display_name,email,oidc_issuer,oidc_subject,state"
	}
	query = "SELECT * FROM (" + query + ") AS collection"
	if search != "" {
		search = strings.NewReplacer("=", "==", "%", "=%", "_", "=_").Replace(search)
		query += " WHERE CONCAT_WS(' '," + columns + ") LIKE ? ESCAPE '='"
		args = append(args, "%"+search+"%")
	}
	query += " ORDER BY " + order + " LIMIT ? OFFSET ?"
	args = append(args, info.Limit+1, (info.Page-1)*info.Limit)
	v, e := rows(ctx, q, query, args...)
	if e != nil {
		return nil, e
	}
	info.HasMore = len(v) > info.Limit
	if info.HasMore {
		v = v[:info.Limit]
	}
	if dest, ok := ctx.Value(pageContextKey{}).(*pageInfo); ok {
		*dest = info
	}
	return v, nil
}
func sendPage(w http.ResponseWriter, r *http.Request, v []Row) {
	info, _ := r.Context().Value(pageContextKey{}).(*pageInfo)
	send(w, 200, Row{"items": v, "pagination": info})
}
