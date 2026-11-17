package worker

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/aantich/lastapp/internal/registry"
)

// The publication adapter reports busy when the publisher ran a job, passes
// failures through and keeps its cursor for one term only.
func TestPublicationAdapter(t *testing.T) {
	ctx := context.Background()
	var seen []string
	found := true
	p := &Publication{Job: func(_ context.Context, _ registry.Queryer, cursor ...*string) (bool, error) {
		seen = append(seen, *cursor[0])
		*cursor[0] = "tenant-b"
		return found, nil
	}}
	for range 2 {
		if busy, e := p.Run(ctx, nil); !busy || e != nil {
			t.Fatal("publication adapter", busy, e)
		}
	}
	if e := p.Start(ctx, nil); e != nil {
		t.Fatal(e)
	}
	found = false
	if busy, e := p.Run(ctx, nil); busy || e != nil {
		t.Fatal("idle publication reported busy", busy, e)
	}
	if len(seen) != 3 || seen[0] != "" || seen[1] != "tenant-b" || seen[2] != "" {
		t.Fatalf("publication cursor %q", seen)
	}
	var _ Starter = &Cleanup{}
	var _ Starter = p
	var _ Starter = &Fair{}
}

// queue is a fake TenantJob: each Tenant's head job is waiting, progressing or
// failing; it records the Tenants it visited.
type queue struct {
	heads   map[string]string // Tenant → "wait", "work" or "fail"
	visited []string
}

func (q *queue) job(_ context.Context, _ registry.Queryer, after string) (string, bool, error) {
	var tenants []string
	for t := range q.heads {
		if t > after {
			tenants = append(tenants, t)
		}
	}
	if len(tenants) == 0 {
		return "", false, nil
	}
	t := slices.Min(tenants)
	q.visited = append(q.visited, t)
	switch q.heads[t] {
	case "work":
		return t, true, nil
	case "fail":
		return t, false, errors.New("job failed")
	}
	return t, false, nil
}

// (a) A Tenant whose head job waits is skipped: the next Tenant's job runs in
// the same pass, Tenants take turns, a failing Tenant is passed over next time,
// and a pass where everything waits visits each Tenant once and reports idle.
func TestFairSkipsWaitingTenants(t *testing.T) {
	ctx := context.Background()
	q := &queue{heads: map[string]string{"a": "wait", "b": "work", "c": "work"}}
	f := &Fair{Label: "deployments", Job: q.job}
	for _, want := range [][]string{{"a", "b"}, {"c"}, {"a", "b"}} {
		q.visited = nil
		if busy, e := f.Run(ctx, nil); !busy || e != nil {
			t.Fatal("pass did not run the waiting Tenant's neighbour", busy, e)
		}
		if !slices.Equal(q.visited, want) {
			t.Fatalf("visited %v, want %v", q.visited, want)
		}
	}
	q.heads = map[string]string{"a": "wait", "b": "wait", "c": "wait"}
	q.visited = nil
	if busy, e := f.Run(ctx, nil); busy || e != nil {
		t.Fatal("all-waiting pass reported busy", busy, e)
	}
	if !slices.Equal(q.visited, []string{"c", "a", "b"}) {
		t.Fatalf("all-waiting pass visited %v", q.visited)
	}
	q.heads = map[string]string{"a": "fail", "b": "work"}
	if e := f.Start(ctx, nil); e != nil {
		t.Fatal(e)
	}
	q.visited = nil
	if _, e := f.Run(ctx, nil); e == nil {
		t.Fatal("job error swallowed")
	}
	if busy, e := f.Run(ctx, nil); !busy || e != nil || !slices.Equal(q.visited, []string{"a", "b"}) {
		t.Fatalf("failing Tenant starved the next one: %v %v %v", busy, e, q.visited)
	}
	q.heads = map[string]string{}
	if busy, e := f.Run(ctx, nil); busy || e != nil {
		t.Fatal("empty queue", busy, e)
	}
}
