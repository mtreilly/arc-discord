package cache

import (
	"sync"
	"sync/atomic"
)

// CacheStats exposes hit/miss/eviction totals.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
}

type entry[K comparable, V any] struct {
	key   K
	value V
	prev  *entry[K, V]
	next  *entry[K, V]
}

// LRUCache provides a capacity-limited cache with warm/invalidate helpers.
type LRUCache[K comparable, V any] struct {
	capacity int
	items    map[K]*entry[K, V]
	head     *entry[K, V]
	tail     *entry[K, V]

	hits      int64
	misses    int64
	evictions int64

	mu sync.Mutex
}

// NewLRUCache creates a cache with the requested capacity (>0).
func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 128
	}
	return &LRUCache[K, V]{
		capacity: capacity,
		items:    make(map[K]*entry[K, V], capacity),
	}
}

// Get returns a cached value and marks it as recently used.
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ent, ok := c.items[key]; ok {
		c.moveToFront(ent)
		atomic.AddInt64(&c.hits, 1)
		return ent.value, true
	}
	var zero V
	atomic.AddInt64(&c.misses, 1)
	return zero, false
}

// Set stores a value and evicts the least-recently used item if needed.
func (c *LRUCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ent, ok := c.items[key]; ok {
		ent.value = value
		c.moveToFront(ent)
		return
	}
	ent := &entry[K, V]{key: key, value: value}
	c.items[key] = ent
	c.prepend(ent)
	if len(c.items) > c.capacity {
		c.evict()
	}
}

// Delete removes an entry from the cache.
func (c *LRUCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ent, ok := c.items[key]; ok {
		c.remove(ent)
		delete(c.items, key)
	}
}

// Warm injects entries without affecting eviction priority.
func (c *LRUCache[K, V]) Warm(entries map[K]V) {
	for k, v := range entries {
		c.Set(k, v)
	}
}

// Invalidate removes entries matching the predicate.
func (c *LRUCache[K, V]) Invalidate(fn func(K, V) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, ent := range c.items {
		if fn(k, ent.value) {
			c.remove(ent)
			delete(c.items, k)
		}
	}
}

// Stats returns snapshot of cache metrics.
func (c *LRUCache[K, V]) Stats() CacheStats {
	return CacheStats{
		Hits:      atomic.LoadInt64(&c.hits),
		Misses:    atomic.LoadInt64(&c.misses),
		Evictions: atomic.LoadInt64(&c.evictions),
	}
}

func (c *LRUCache[K, V]) prepend(ent *entry[K, V]) {
	ent.prev = nil
	ent.next = c.head
	if c.head != nil {
		c.head.prev = ent
	}
	c.head = ent
	if c.tail == nil {
		c.tail = ent
	}
}

func (c *LRUCache[K, V]) remove(ent *entry[K, V]) {
	if ent.prev != nil {
		ent.prev.next = ent.next
	}
	if ent.next != nil {
		ent.next.prev = ent.prev
	}
	if c.head == ent {
		c.head = ent.next
	}
	if c.tail == ent {
		c.tail = ent.prev
	}
	ent.prev = nil
	ent.next = nil
}

func (c *LRUCache[K, V]) moveToFront(ent *entry[K, V]) {
	if c.head == ent {
		return
	}
	c.remove(ent)
	c.prepend(ent)
}

func (c *LRUCache[K, V]) evict() {
	if c.tail == nil {
		return
	}
	ent := c.tail
	c.remove(ent)
	delete(c.items, ent.key)
	atomic.AddInt64(&c.evictions, 1)
}
