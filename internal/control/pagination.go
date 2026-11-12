package control

import (
	"context"
	"net/http"
	"strconv"
)

// Optional bounded offset pagination keeps the initial console API simple.
// Permission predicates remain inside the original query, before LIMIT/OFFSET.
func pageRows(ctx context.Context, q Queryer, r *http.Request, query string, args ...any) ([]Row, error) {
	values := r.URL.Query()
	if !values.Has("page") && !values.Has("limit") {
		return rows(ctx, q, query, args...)
	}
	page, limit := 1, 100
	var e error
	if values.Has("page") {
		page, e = strconv.Atoi(values.Get("page"))
		if e != nil || page < 1 || page > 10000 {
			return nil, bad("Page must be between 1 and 10000.")
		}
	}
	if values.Has("limit") {
		limit, e = strconv.Atoi(values.Get("limit"))
		if e != nil || limit < 1 || limit > 200 {
			return nil, bad("Limit must be between 1 and 200.")
		}
	}
	args = append(args, limit, (page-1)*limit)
	return rows(ctx, q, query+" LIMIT ? OFFSET ?", args...)
}
