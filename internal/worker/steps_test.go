package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/aantich/lastapp/internal/registry"
)

// The queue adapters never report busy (only provisioning paces the leader),
// pass failures through and keep the publication cursor for one term only.
func TestStepAdapters(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("job failed")
	j := Job{Label: "deployments", Do: func(context.Context, registry.Queryer) error { return boom }}
	if busy, e := j.Run(ctx, nil); busy || !errors.Is(e, boom) || j.Name() != "deployments" {
		t.Fatal("job adapter", busy, e)
	}
	var seen []string
	p := &Publication{Job: func(_ context.Context, _ registry.Queryer, cursor ...*string) (bool, error) {
		seen = append(seen, *cursor[0])
		*cursor[0] = "tenant-b"
		return true, nil
	}}
	for range 2 {
		if busy, e := p.Run(ctx, nil); busy || e != nil {
			t.Fatal("publication adapter", busy, e)
		}
	}
	if e := p.Start(ctx, nil); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Run(ctx, nil); e != nil {
		t.Fatal(e)
	}
	if len(seen) != 3 || seen[0] != "" || seen[1] != "tenant-b" || seen[2] != "" {
		t.Fatalf("publication cursor %q", seen)
	}
	var _ Starter = &Cleanup{}
	var _ Starter = p
}
