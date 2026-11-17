package dataapi

import (
	"testing"

	"github.com/aantich/lastapp/internal/entity"
)

func TestTenantDataFairness(t *testing.T) {
	var limiter inFlightTenants
	releases := []func(){}
	for i := 0; i < entity.MaxTenantDataRequests; i++ {
		release, ok := limiter.reserve("one")
		if !ok {
			t.Fatal("premature limit")
		}
		releases = append(releases, release)
	}
	if _, ok := limiter.reserve("one"); ok {
		t.Fatal("one Tenant exceeded its concurrency budget")
	}
	other, ok := limiter.reserve("two")
	if !ok {
		t.Fatal("another Tenant was blocked")
	}
	other()
	for _, release := range releases {
		release()
	}
	if len(limiter.counts) != 0 {
		t.Fatal("idle Tenant counters retained")
	}
}
