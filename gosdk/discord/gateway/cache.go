package gateway

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// Cache tracks gateway state for guilds, channels, and members.
type Cache interface {
	GetGuild(guildID string) (*types.Guild, bool)
	SetGuild(guild *types.Guild)
	RemoveGuild(guildID string)

	GetChannel(channelID string) (*types.Channel, bool)
	SetChannel(channel *types.Channel)
	RemoveChannel(channelID string)

	GetMember(guildID, userID string) (*types.Member, bool)
	SetMember(guildID string, member *types.Member)
	RemoveMember(guildID, userID string)

	Stats() CacheStats
}

// CacheStats exposes hit/miss counts for diagnostics.
type CacheStats struct {
	GuildHits     int64 `json:"guild_hits"`
	GuildMisses   int64 `json:"guild_misses"`
	ChannelHits   int64 `json:"channel_hits"`
	ChannelMisses int64 `json:"channel_misses"`
	MemberHits    int64 `json:"member_hits"`
	MemberMisses  int64 `json:"member_misses"`
}

type cachedItem struct {
	created time.Time
	expires time.Time
}

type cachedGuild struct {
	*types.Guild
	cachedItem
}

type cachedChannel struct {
	*types.Channel
	cachedItem
}

type cachedMember struct {
	*types.Member
	cachedItem
}

// MemoryCache is a thread-safe in-memory cache with optional TTL.
type MemoryCache struct {
	guilds   map[string]cachedGuild
	channels map[string]cachedChannel
	members  map[string]map[string]cachedMember
	ttl      time.Duration
	mu       sync.RWMutex

	guildHits     int64
	guildMisses   int64
	channelHits   int64
	channelMisses int64
	memberHits    int64
	memberMisses  int64
}

// NewMemoryCache creates a cache. A ttl <= 0 disables expiration.
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		guilds:   map[string]cachedGuild{},
		channels: map[string]cachedChannel{},
		members:  map[string]map[string]cachedMember{},
		ttl:      ttl,
	}
}

func (c *MemoryCache) expiration() time.Time {
	if c.ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(c.ttl)
}

func (c *MemoryCache) GetGuild(guildID string) (*types.Guild, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.guilds[guildID]
	if !ok || entry.isExpired(c.ttl) {
		atomic.AddInt64(&c.guildMisses, 1)
		return nil, false
	}
	atomic.AddInt64(&c.guildHits, 1)
	return entry.Guild, true
}

func (c *MemoryCache) SetGuild(guild *types.Guild) {
	if guild == nil {
		return
	}

	c.mu.Lock()
	c.guilds[guild.ID] = cachedGuild{
		Guild: guild,
		cachedItem: cachedItem{
			created: time.Now(),
			expires: c.expiration(),
		},
	}
	c.mu.Unlock()
}

func (c *MemoryCache) RemoveGuild(guildID string) {
	c.mu.Lock()
	delete(c.guilds, guildID)
	c.mu.Unlock()
}

func (c *MemoryCache) GetChannel(channelID string) (*types.Channel, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.channels[channelID]
	if !ok || entry.isExpired(c.ttl) {
		atomic.AddInt64(&c.channelMisses, 1)
		return nil, false
	}
	atomic.AddInt64(&c.channelHits, 1)
	return entry.Channel, true
}

func (c *MemoryCache) SetChannel(channel *types.Channel) {
	if channel == nil {
		return
	}

	c.mu.Lock()
	c.channels[channel.ID] = cachedChannel{
		Channel: channel,
		cachedItem: cachedItem{
			created: time.Now(),
			expires: c.expiration(),
		},
	}
	c.mu.Unlock()
}

func (c *MemoryCache) RemoveChannel(channelID string) {
	c.mu.Lock()
	delete(c.channels, channelID)
	c.mu.Unlock()
}

func (c *MemoryCache) GetMember(guildID, userID string) (*types.Member, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	members, ok := c.members[guildID]
	if !ok {
		atomic.AddInt64(&c.memberMisses, 1)
		return nil, false
	}

	entry, ok := members[userID]
	if !ok || entry.isExpired(c.ttl) {
		atomic.AddInt64(&c.memberMisses, 1)
		return nil, false
	}
	atomic.AddInt64(&c.memberHits, 1)
	return entry.Member, true
}

func (c *MemoryCache) SetMember(guildID string, member *types.Member) {
	if member == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.members[guildID]; !ok {
		c.members[guildID] = map[string]cachedMember{}
	}
	c.members[guildID][member.User.ID] = cachedMember{
		Member: member,
		cachedItem: cachedItem{
			created: time.Now(),
			expires: c.expiration(),
		},
	}
}

func (c *MemoryCache) RemoveMember(guildID, userID string) {
	c.mu.Lock()
	if members, ok := c.members[guildID]; ok {
		delete(members, userID)
	}
	c.mu.Unlock()
}

func (c *MemoryCache) Stats() CacheStats {
	return CacheStats{
		GuildHits:     atomic.LoadInt64(&c.guildHits),
		GuildMisses:   atomic.LoadInt64(&c.guildMisses),
		ChannelHits:   atomic.LoadInt64(&c.channelHits),
		ChannelMisses: atomic.LoadInt64(&c.channelMisses),
		MemberHits:    atomic.LoadInt64(&c.memberHits),
		MemberMisses:  atomic.LoadInt64(&c.memberMisses),
	}
}

func (i cachedItem) isExpired(ttl time.Duration) bool {
	if ttl <= 0 {
		return false
	}
	return !i.expires.IsZero() && time.Now().After(i.expires)
}
