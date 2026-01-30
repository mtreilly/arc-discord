package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Bucket represents a rate limit bucket for a specific route
type Bucket struct {
	// Key is the bucket identifier (from X-RateLimit-Bucket header)
	Key string

	// Limit is the maximum number of requests allowed
	Limit int

	// Remaining is the number of requests remaining
	Remaining int

	// Reset is the time when the bucket resets
	Reset time.Time

	// Global indicates if this is a global rate limit
	Global bool
}

// Tracker interface defines methods for tracking rate limits
type Tracker interface {
	// Wait blocks until the rate limit allows the request
	Wait(ctx context.Context, route string) error

	// Update updates the rate limit information from response headers
	Update(route string, headers http.Header)

	// GetBucket returns the current rate limit bucket for a route
	GetBucket(route string) *Bucket

	// Clear removes all stored rate limit information
	Clear()
}

// MemoryTracker implements an in-memory rate limit tracker
type MemoryTracker struct {
	buckets       map[string]*Bucket
	routeToBucket map[string]string
	global        *Bucket
	mu            sync.RWMutex
}

// NewMemoryTracker creates a new in-memory rate limit tracker
func NewMemoryTracker() *MemoryTracker {
	return &MemoryTracker{
		buckets:       make(map[string]*Bucket),
		routeToBucket: make(map[string]string),
	}
}

// Wait blocks until the rate limit allows the request
func (t *MemoryTracker) Wait(ctx context.Context, route string) error {
	t.mu.RLock()

	// Check global rate limit first
	if t.global != nil && time.Now().Before(t.global.Reset) {
		globalReset := t.global.Reset
		t.mu.RUnlock()

		waitDuration := time.Until(globalReset)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			return nil
		}
	}

	// Check route-specific rate limit
	bucket, exists := t.getBucketByRouteLocked(route)
	if !exists || bucket.Remaining > 0 {
		t.mu.RUnlock()
		return nil
	}

	// Need to wait for reset
	resetTime := bucket.Reset
	t.mu.RUnlock()

	if time.Now().Before(resetTime) {
		waitDuration := time.Until(resetTime)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			return nil
		}
	}

	return nil
}

// Update updates the rate limit information from response headers
func (t *MemoryTracker) Update(route string, headers http.Header) {
	// Parse rate limit headers
	limit := parseIntHeader(headers, "X-RateLimit-Limit")
	remaining := parseIntHeader(headers, "X-RateLimit-Remaining")
	resetAfter := parseFloatHeader(headers, "X-RateLimit-Reset-After")
	bucketKey := headers.Get("X-RateLimit-Bucket")
	global := headers.Get("X-RateLimit-Global") == "true"

	// Calculate reset time
	var resetTime time.Time
	if resetAfter > 0 {
		resetTime = time.Now().Add(time.Duration(resetAfter * float64(time.Second)))
	} else {
		// Fallback to Reset header (Unix timestamp)
		resetUnix := parseFloatHeader(headers, "X-RateLimit-Reset")
		if resetUnix > 0 {
			resetTime = time.Unix(int64(resetUnix), 0)
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	bucket := &Bucket{
		Key:       bucketKey,
		Limit:     limit,
		Remaining: remaining,
		Reset:     resetTime,
		Global:    global,
	}

	if global {
		t.global = bucket
	} else {
		key := bucketKey
		if key == "" {
			key = route
		}

		t.buckets[key] = bucket
		t.routeToBucket[route] = key
	}

	// Clean up expired buckets
	t.cleanupExpired()
}

// GetBucket returns the current rate limit bucket for a route
func (t *MemoryTracker) GetBucket(route string) *Bucket {
	t.mu.RLock()
	defer t.mu.RUnlock()

	bucket, exists := t.getBucketByRouteLocked(route)
	if !exists {
		return nil
	}

	// Return a copy to avoid external mutation
	return &Bucket{
		Key:       bucket.Key,
		Limit:     bucket.Limit,
		Remaining: bucket.Remaining,
		Reset:     bucket.Reset,
		Global:    bucket.Global,
	}
}

// Clear removes all stored rate limit information
func (t *MemoryTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buckets = make(map[string]*Bucket)
	t.routeToBucket = make(map[string]string)
	t.global = nil
}

// cleanupExpired removes expired buckets (must be called with lock held)
func (t *MemoryTracker) cleanupExpired() {
	now := time.Now()
	expiredKeys := make(map[string]struct{})

	for key, bucket := range t.buckets {
		if now.After(bucket.Reset) {
			expiredKeys[key] = struct{}{}
			delete(t.buckets, key)
		}
	}

	if len(expiredKeys) > 0 {
		for route, key := range t.routeToBucket {
			if _, ok := expiredKeys[key]; ok {
				delete(t.routeToBucket, route)
			}
		}
	}

	// Clear global if expired
	if t.global != nil && now.After(t.global.Reset) {
		t.global = nil
	}
}

// getBucketByRouteLocked retrieves a bucket using route aliases.
// Caller must hold at least a read lock.
func (t *MemoryTracker) getBucketByRouteLocked(route string) (*Bucket, bool) {
	if key, ok := t.routeToBucket[route]; ok {
		bucket, exists := t.buckets[key]
		return bucket, exists
	}

	bucket, exists := t.buckets[route]
	return bucket, exists
}

// Helper functions

func parseIntHeader(headers http.Header, key string) int {
	value := headers.Get(key)
	if value == "" {
		return 0
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return intValue
}

func parseFloatHeader(headers http.Header, key string) float64 {
	value := headers.Get(key)
	if value == "" {
		return 0
	}

	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}

	return floatValue
}

// RouteFromEndpoint extracts a rate limit route identifier from an endpoint
// Discord uses major parameters (guild_id, channel_id, etc.) for route bucketing
func RouteFromEndpoint(method, endpoint string) string {
	// This is a simplified implementation
	// In production, you'd parse the endpoint and replace IDs with placeholders
	// For example: /channels/123456/messages/789 -> /channels/:id/messages/:id
	return fmt.Sprintf("%s:%s", method, endpoint)
}
