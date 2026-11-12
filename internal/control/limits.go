package control

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Administrative metadata has deliberately generous but finite limits. These are
// safety ceilings, independent of a subscription's usually smaller resource quotas.
var metadataLimits = map[string]int{
	"tenant_memberships": 1000, "groups": 200, "group_memberships": 10000,
	"roles": 200, "access_grants": 2000, "microapp_installations": 1000,
}

func metadataQuota(ctx context.Context, q Queryer, tenant, table string) error {
	var n int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id=?", tenant).Scan(&n); e != nil {
		return e
	}
	if n >= metadataLimits[table] {
		return problem(409, "metadata_limit", "The Tenant's administrative metadata limit has been reached.")
	}
	return nil
}

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
		v = rateEntry{tokens: 120, at: now}
	}
	v.tokens = min(120, v.tokens+now.Sub(v.at).Seconds()*20)
	v.at = now
	allowed := v.tokens >= 1
	if allowed {
		v.tokens--
	}
	l.entries[key] = v
	return allowed
}

func (s *Server) limitRequest(w http.ResponseWriter, p Principal, r *http.Request) error {
	now := time.Now()
	if !s.limiter.allow("user:"+p.ID, now) || (r.PathValue("tenant") != "" && !s.limiter.allow("tenant:"+r.PathValue("tenant"), now)) {
		w.Header().Set("Retry-After", "1")
		return problem(429, "rate_limited", "Too many requests. Retry shortly.")
	}
	return nil
}
