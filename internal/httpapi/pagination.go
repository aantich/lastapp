package httpapi

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

// WithPageInfo gives ctx a page destination for PageRows and SendPage.
func WithPageInfo(ctx context.Context) context.Context {
	return context.WithValue(ctx, pageContextKey{}, &op.PageInfo{})
}

// PageInfo returns the page destination placed in ctx by WithPageInfo.
func PageInfo(ctx context.Context) *op.PageInfo {
	info, _ := ctx.Value(pageContextKey{}).(*op.PageInfo)
	return info
}

// PageRequest reads page, limit and q from a query string. Unparseable or
// non-positive explicit values become -1 so that info() rejects them.
func PageRequest(values url.Values) op.PageRequest {
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

// PageRows is the shim for handlers not yet migrated to operations: it takes
// the collection key from the request path and reports paging through ctx.
func PageRows[T any, P registry.Record[T]](ctx context.Context, q registry.Queryer, r *http.Request, query string, args ...any) ([]T, error) {
	values := r.URL.Query()
	key := path.Base(r.URL.Path)
	if key == "recipients" && values.Get("type") == "team" {
		key = "team-recipients"
	}
	page, e := op.PageScan[T, P](ctx, q, PageRequest(values), op.CollectionFor(key), query, args...)
	if e != nil {
		return nil, e
	}
	if dest := PageInfo(ctx); dest != nil {
		*dest = page.Pagination
	}
	return page.Items, nil
}

// SendPage sends items with the pagination PageRows recorded.
func SendPage[T any](w http.ResponseWriter, r *http.Request, v []T) {
	Send(w, 200, registry.Row{"items": v, "pagination": PageInfo(r.Context())})
}
