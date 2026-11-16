package control

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
type requestLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

// Per-process fairness, not a distributed billing quota. The bounded map evicts
// idle entries; a full map rejects new keys instead of allocating without limit.
func (l *requestLimiter) allow(key string, now time.Time) bool {
	return l.allowRate(key, now, 120, 20)
}

func (l *requestLimiter) allowRate(key string, now time.Time, burst, refill float64) bool {
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

func (s *Server) limitRequest(w http.ResponseWriter, p access.Principal, r *http.Request) error {
	now := time.Now()
	if !s.limiter.allow("user:"+p.ID, now) {
		return rateLimited(w)
	}
	if tenant := r.PathValue("tenant"); tenant != "" {
		// Bound unauthorized traffic per user, but never let an untrusted Tenant
		// ID allocate or consume another Tenant's shared request budget.
		if !p.Platform {
			if e := s.Member(r.Context(), s.DB, p, tenant, false); e != nil {
				return e
			}
		}
		if !s.limiter.allow("tenant:"+tenant, time.Now()) {
			return rateLimited(w)
		}
	}
	return nil
}

func rateLimited(w http.ResponseWriter) error {
	w.Header().Set("Retry-After", "1")
	return registry.Problem(429, "rate_limited", "Too many requests. Retry shortly.")
}
