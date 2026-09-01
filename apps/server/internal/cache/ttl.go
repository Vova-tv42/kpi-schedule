// Package cache provides a minimal generic in-memory TTL cache.
// It is process-local by design: group schedules are deliberately never
// persisted to Postgres (see docs/architecture/data-storage.md), so a cache
// miss just means one more upstream fetch, never a correctness problem.
package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

type TTL[V any] struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]entry[V]
}

func New[V any](ttl time.Duration) *TTL[V] {
	return &TTL[V]{
		ttl:     ttl,
		entries: make(map[string]entry[V]),
	}
}

func (c *TTL[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *TTL[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
}
