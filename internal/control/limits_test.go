package control

import "testing"

func TestTenantRateLimitIsolation(t *testing.T) {
	s, c, _, owner, outsider := foundationFixture(t)
	tenant := newID()
	c.want("POST", "/admin/v1/tenants", Row{"id": tenant, "slug": "limiter-" + tenant, "display_name": "Rate limit isolation", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin_user_id": owner}, 0, 201)
	base := "/v1/tenants/" + tenant
	admin, outside := c.as(owner), c.as(outsider)
	reset := func() {
		s.limiter.mu.Lock()
		defer s.limiter.mu.Unlock()
		s.limiter.entries = nil
	}
	assertUncharged := func(t *testing.T, tenant string) {
		t.Helper()
		s.limiter.mu.Lock()
		defer s.limiter.mu.Unlock()
		if _, exists := s.limiter.entries["tenant:"+tenant]; exists {
			t.Fatal("an unauthorized request allocated or consumed a Tenant bucket")
		}
		if _, exists := s.limiter.entries["user:"+outsider]; !exists {
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
		s.limiter.mu.Lock()
		_, charged := s.limiter.entries["tenant:"+tenant]
		s.limiter.mu.Unlock()
		if !charged {
			t.Fatal("authorized traffic bypassed the Tenant limiter")
		}
	})
	t.Run("untrusted Tenant IDs do not allocate buckets", func(t *testing.T) {
		outside := outside
		outside.t = t
		reset()
		for _, target := range []string{tenant, newID(), "not-a-tenant"} {
			outside.want("GET", "/v1/tenants/"+target+"/capabilities", nil, 0, 404)
			assertUncharged(t, target)
		}
	})
	t.Run("disabled membership cannot spend Tenant budget", func(t *testing.T) {
		outside, admin := outside, admin
		outside.t, admin.t = t, t
		reset()
		admin.want("POST", base+"/memberships", Row{"user_id": outsider}, 0, 201)
		admin.want("PATCH", base+"/memberships/"+outsider, Row{"state": "disabled"}, 1, 200)
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
		s.limiter.mu.Lock()
		_, charged := s.limiter.entries["tenant:"+tenant]
		s.limiter.mu.Unlock()
		if !charged {
			t.Fatal("platform access bypassed the Tenant limiter")
		}
	})
}
