package control

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

// sameJSON requires that a typed registry record marshals byte-for-byte like
// the untyped Row reader did for the same query (keys, order, nulls, *_json
// documents, numbers and timestamps), over at least one row.
func sameJSON[T any, P record[T]](t *testing.T, q Queryer, query string, args ...any) {
	t.Helper()
	ctx := context.Background()
	untyped, e := rows(ctx, q, query, args...)
	if e != nil {
		t.Fatal(e)
	}
	typed, e := scanAll[T, P](ctx, q, query, args...)
	if e != nil {
		t.Fatal(e)
	}
	if len(untyped) == 0 {
		t.Fatalf("no rows to compare for %s", query)
	}
	want, _ := json.Marshal(untyped)
	got, _ := json.Marshal(typed)
	if !bytes.Equal(want, got) {
		t.Fatalf("typed JSON differs for %s\nwant: %s\ngot:  %s", query, want, got)
	}
}

func TestRegistryRecordsMatchUntypedRows(t *testing.T) {
	s, platform, _, _, _ := foundationFixture(t)
	limited := Row{"id": newID(), "code": "limited-" + newID()[:8], "display_name": "Limited", "allow_shared_database": true, "allow_managed_database": true, "default_layout": "shared", "max_spaces": 2, "max_apps": 0}
	platform.want("POST", "/admin/v1/plans", limited, 0, 201)
	sameJSON[Plan](t, s.DB, "SELECT "+planColumns+" FROM plans ORDER BY id")
}
