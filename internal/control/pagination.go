package control

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

type pageContextKey struct{}

// pageRequest reads page, limit and q from a query string. Unparseable or
// non-positive explicit values become -1 so that info() rejects them.
func pageRequest(values url.Values) op.PageRequest {
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
	return op.PageRequest{Page: number("page"), Limit: number("limit"), Search: values.Get("q")}
}

// pageRows is the shim for handlers not yet migrated to operations: it takes
// the collection key from the request path and reports paging through ctx.
func pageRows[T any, P registry.Record[T]](ctx context.Context, q registry.Queryer, r *http.Request, query string, args ...any) ([]T, error) {
	values := r.URL.Query()
	key := path.Base(r.URL.Path)
	if key == "recipients" && values.Get("type") == "team" {
		key = "team-recipients"
	}
	page, e := op.PageScan[T, P](ctx, q, pageRequest(values), op.CollectionFor(key), query, args...)
	if e != nil {
		return nil, e
	}
	if dest, ok := ctx.Value(pageContextKey{}).(*op.PageInfo); ok {
		*dest = page.Pagination
	}
	return page.Items, nil
}

func sendPage[T any](w http.ResponseWriter, r *http.Request, v []T) {
	info, _ := r.Context().Value(pageContextKey{}).(*op.PageInfo)
	send(w, 200, registry.Row{"items": v, "pagination": info})
}
