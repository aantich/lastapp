package control

import (
	"sync"

	"github.com/aantich/lastapp/internal/entity"
)

// Only active requests occupy this map; the global semaphore also bounds its size.
type inFlightTenants struct {
	mu     sync.Mutex
	counts map[string]int
}

func (l *inFlightTenants) reserve(tenant string) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts == nil {
		l.counts = map[string]int{}
	}
	if l.counts[tenant] >= entity.MaxTenantDataRequests {
		return nil, false
	}
	l.counts[tenant]++
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.counts[tenant]--
		if l.counts[tenant] == 0 {
			delete(l.counts, tenant)
		}
	}, true
}
