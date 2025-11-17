package monitor

import (
	"sync"
	"time"

	"github.com/quueli/mc-server-monitor/internal/mcping"
)

type cacheEntry struct {
	status    *mcping.Status
	expiresAt time.Time
}

// resultCache holds recent ping results so a burst of checks for the same host
// inside the ttl window does not hit the network again.
type resultCache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]cacheEntry
}

func newResultCache(ttl time.Duration) *resultCache {
	return &resultCache{ttl: ttl, items: make(map[string]cacheEntry)}
}

func (c *resultCache) get(key string) (*mcping.Status, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.status, true
}

func (c *resultCache) set(key string, s *mcping.Status) {
	c.mu.Lock()
	c.items[key] = cacheEntry{status: s, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *resultCache) purgeExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.items {
		if now.After(e.expiresAt) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}
