package control

import (
	"testing"

	"github.com/aantich/lastapp/internal/registry"
)

func TestTenantRateLimitIsolation(t *testing.T) {
	s, c, _, owner, outsider := foundationFixture(t)
	tenant := registry.NewID()
	c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "limiter-" + tenant, "display_name": "Rate limit isolation", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	base := "/v1/tenants/" + tenant
	admin, outside := c.as(owner), c.as(outsider)
	reset := s.limiter.Reset
	assertUncharged := func(t *testing.T, tenant string) {
		t.Helper()
		if s.limiter.Has("tenant:" + tenant) {
			t.Fatal("an unauthorized request allocated or consumed a Tenant bucket")
		}
		if !s.limiter.Has("user:" + outsider) {
			t.Fatal("unauthorized requests bypassed the per-user limiter")
		}
	}

	t.Run("non-member burst does not block Tenant administrator", func(t *testing.T) {
		outside, admin := outside, admin
		outside.t, admin.t = t, t
		reset()
		for range 120 {
			outside.want("GET", base+"/capabilities", nil, 0, 404)
		}
		admin.want("GET", base+"/capabilities", nil, 0, 200)
		// A legitimate request still spends the shared Tenant budget.
		charged := s.limiter.Has("tenant:" + tenant)
		if !charged {
			t.Fatal("authorized traffic bypassed the Tenant limiter")
		}
	})
	t.Run("untrusted Tenant IDs do not allocate buckets", func(t *testing.T) {
		outside := outside
		outside.t = t
		reset()
		for _, target := range []string{tenant, registry.NewID(), "not-a-tenant"} {
			outside.want("GET", "/v1/tenants/"+target+"/capabilities", nil, 0, 404)
			assertUncharged(t, target)
		}
	})
	t.Run("disabled membership cannot spend Tenant budget", func(t *testing.T) {
		outside, admin := outside, admin
		outside.t, admin.t = t, t
		reset()
		admin.want("POST", base+"/memberships", registry.Row{"user_id": outsider}, 0, 201)
		admin.want("PATCH", base+"/memberships/"+outsider, registry.Row{"state": "disabled"}, 1, 200)
		reset()
		outside.want("GET", base+"/capabilities", nil, 0, 404)
		assertUncharged(t, tenant)
		admin.want("GET", base+"/capabilities", nil, 0, 200)
	})
	t.Run("platform administrator may access a Tenant without membership", func(t *testing.T) {
		c := c
		c.t = t
		reset()
		c.want("GET", base+"/capabilities", nil, 0, 200)
		charged := s.limiter.Has("tenant:" + tenant)
		if !charged {
			t.Fatal("platform access bypassed the Tenant limiter")
		}
	})
}
