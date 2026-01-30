package cache

import "testing"

func TestLRUCacheEvicts(t *testing.T) {
	cache := NewLRUCache[string, string](2)
	cache.Set("a", "1")
	cache.Set("b", "2")
	cache.Set("c", "3")
	if _, ok := cache.Get("a"); ok {
		t.Fatalf("a should have evicted")
	}
	if stats := cache.Stats(); stats.Evictions != 1 {
		t.Fatalf("expected eviction count 1, got %d", stats.Evictions)
	}
}

func TestLRUCacheWarmInvalidate(t *testing.T) {
	cache := NewLRUCache[string, string](3)
	cache.Warm(map[string]string{
		"x": "1",
		"y": "2",
	})
	if _, ok := cache.Get("x"); !ok {
		t.Fatalf("expected warm entry")
	}
	cache.Invalidate(func(key string, value string) bool {
		return value == "2"
	})
	if _, ok := cache.Get("y"); ok {
		t.Fatalf("invalidated entry should be gone")
	}
}
