package gateway

import (
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestMemoryCacheGuildLifecycle(t *testing.T) {
	cache := NewMemoryCache(0)
	g := &types.Guild{ID: "g1", Name: "test"}
	cache.SetGuild(g)

	if _, ok := cache.GetGuild("g1"); !ok {
		t.Fatalf("expected guild to be cached")
	}
	cache.RemoveGuild("g1")
	if _, ok := cache.GetGuild("g1"); ok {
		t.Fatalf("expected guild removed")
	}
}

func TestMemoryCacheTTL(t *testing.T) {
	cache := NewMemoryCache(10 * time.Millisecond)
	g := &types.Guild{ID: "g2", Name: "tempo"}
	cache.SetGuild(g)
	time.Sleep(20 * time.Millisecond)
	if _, ok := cache.GetGuild("g2"); ok {
		t.Fatalf("expected guild to expire")
	}
}

func TestMemoryCacheStats(t *testing.T) {
	cache := NewMemoryCache(0)
	cache.GetGuild("missing")
	cache.SetGuild(&types.Guild{ID: "g3"})
	cache.GetGuild("g3")
	stats := cache.Stats()
	if stats.GuildHits != 1 || stats.GuildMisses != 1 {
		t.Fatalf("unexpected stats %+v", stats)
	}
}

func TestMemoryCacheMemberLifecycle(t *testing.T) {
	cache := NewMemoryCache(0)
	m := &types.Member{User: &types.User{ID: "u1"}, Nick: "nick"}
	cache.SetMember("g4", m)
	if _, ok := cache.GetMember("g4", "u1"); !ok {
		t.Fatalf("expected member present")
	}
	cache.RemoveMember("g4", "u1")
	if _, ok := cache.GetMember("g4", "u1"); ok {
		t.Fatalf("expected member removed")
	}
}
