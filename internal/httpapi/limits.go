package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/registry"
)

type rateEntry struct {
	tokens float64
	at     time.Time
}

// Limiter is a bounded per-process token bucket keyed by caller.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

// Allow spends the administrative budget for key.
// Per-process fairness, not a distributed billing quota. The bounded map evicts
// idle entries; a full map rejects new keys instead of allocating without limit.
func (l *Limiter) Allow(key string, now time.Time) bool {
	return l.AllowRate(key, now, 120, 20)
}

// AllowRate spends one token from key's bucket of burst refilled per second.
func (l *Limiter) AllowRate(key string, now time.Time, burst, refill float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]rateEntry)
	}
	v, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= 10000 {
			for k, old := range l.entries {
				if now.Sub(old.at) > time.Minute {
					delete(l.entries, k)
				}
			}
			if len(l.entries) >= 10000 {
				return false
			}
		}
		v = rateEntry{tokens: burst, at: now}
	}
	v.tokens = min(burst, v.tokens+now.Sub(v.at).Seconds()*refill)
	v.at = now
	allowed := v.tokens >= 1
	if allowed {
		v.tokens--
	}
	l.entries[key] = v
	return allowed
}

// Reset forgets every bucket.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

// Has reports whether key currently has a bucket.
func (l *Limiter) Has(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.entries[key]
	return ok
}

// LimitRequest spends the per-user budget of p and, for a request naming a
// Tenant, that Tenant's shared budget after checking live membership.
func (l *Limiter) LimitRequest(w http.ResponseWriter, r *http.Request, c *access.Checker, q registry.Queryer, p access.Principal) error {
	now := time.Now()
	if !l.Allow("user:"+p.ID, now) {
		return RateLimited(w)
	}
	if tenant := r.PathValue("tenant"); tenant != "" {
		// Bound unauthorized traffic per user, but never let an untrusted Tenant
		// ID allocate or consume another Tenant's shared request budget.
		if !p.Platform {
			if e := c.Member(r.Context(), q, p, tenant, false); e != nil {
				return e
			}
		}
		if !l.Allow("tenant:"+tenant, time.Now()) {
			return RateLimited(w)
		}
	}
	return nil
}

// RateLimited sets Retry-After and returns the rate_limited problem.
func RateLimited(w http.ResponseWriter) error {
	w.Header().Set("Retry-After", "1")
	return registry.Problem(429, "rate_limited", "Too many requests. Retry shortly.")
}
