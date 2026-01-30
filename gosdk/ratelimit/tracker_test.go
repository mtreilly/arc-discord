package ratelimit

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewMemoryTracker(t *testing.T) {
	tracker := NewMemoryTracker()
	if tracker == nil {
		t.Fatal("NewMemoryTracker() returned nil")
	}

	if tracker.buckets == nil {
		t.Error("tracker.buckets is nil")
	}
}

func TestMemoryTracker_Update(t *testing.T) {
	tracker := NewMemoryTracker()

	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "10")
	headers.Set("X-RateLimit-Remaining", "5")
	headers.Set("X-RateLimit-Reset-After", "60")
	headers.Set("X-RateLimit-Bucket", "test-bucket")

	route := "POST:/test/route"
	tracker.Update(route, headers)

	bucket := tracker.GetBucket("test-bucket")
	if bucket == nil {
		t.Fatal("GetBucket() returned nil")
	}

	if bucket.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", bucket.Limit)
	}

	if bucket.Remaining != 5 {
		t.Errorf("Expected remaining 5, got %d", bucket.Remaining)
	}

	if bucket.Key != "test-bucket" {
		t.Errorf("Expected bucket key 'test-bucket', got '%s'", bucket.Key)
	}

	aliasBucket := tracker.GetBucket(route)
	if aliasBucket == nil {
		t.Fatal("GetBucket() should resolve route alias")
	}
	if aliasBucket.Key != "test-bucket" {
		t.Errorf("Alias bucket key mismatch: got %s", aliasBucket.Key)
	}
}

func TestMemoryTracker_Update_GlobalRateLimit(t *testing.T) {
	tracker := NewMemoryTracker()

	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "50")
	headers.Set("X-RateLimit-Remaining", "0")
	headers.Set("X-RateLimit-Reset-After", "1")
	headers.Set("X-RateLimit-Global", "true")

	tracker.Update("/test/route", headers)

	// Global rate limit should block
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := tracker.Wait(ctx, "/any/route")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	if elapsed < 900*time.Millisecond {
		t.Errorf("Expected wait time >= 900ms, got %v", elapsed)
	}
}

func TestMemoryTracker_Wait_NoRateLimit(t *testing.T) {
	tracker := NewMemoryTracker()

	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "10")
	headers.Set("X-RateLimit-Remaining", "5")
	headers.Set("X-RateLimit-Reset-After", "60")

	tracker.Update("/test/route", headers)

	ctx := context.Background()
	start := time.Now()
	err := tracker.Wait(ctx, "/test/route")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	// Should not wait since we have remaining requests
	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait() took too long: %v", elapsed)
	}
}

func TestMemoryTracker_Wait_RateLimited(t *testing.T) {
	tracker := NewMemoryTracker()

	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "10")
	headers.Set("X-RateLimit-Remaining", "0")
	headers.Set("X-RateLimit-Reset-After", "1")
	headers.Set("X-RateLimit-Bucket", "test-bucket")

	route := "POST:/test/route"
	tracker.Update(route, headers)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := tracker.Wait(ctx, route)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	if elapsed < 900*time.Millisecond {
		t.Errorf("Expected wait time >= 900ms, got %v", elapsed)
	}
}

func TestMemoryTracker_RouteAliasCleanup(t *testing.T) {
	tracker := NewMemoryTracker()

	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "5")
	headers.Set("X-RateLimit-Remaining", "0")
	headers.Set("X-RateLimit-Reset", "0") // expired
	headers.Set("X-RateLimit-Bucket", "bucket-alias")

	route := "POST:/channels/123/messages"
	tracker.Update(route, headers)

	// Trigger cleanup with fresh bucket
	fresh := make(http.Header)
	fresh.Set("X-RateLimit-Limit", "1")
	fresh.Set("X-RateLimit-Remaining", "1")
	fresh.Set("X-RateLimit-Reset-After", "60")
	tracker.Update("/another/route", fresh)

	if tracker.GetBucket(route) != nil {
		t.Fatalf("Expected route alias bucket to expire")
	}

	// Wait should not block because alias is gone
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := tracker.Wait(ctx, route); err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Wait() returned unexpected error: %v", err)
	}
}

func TestMemoryTracker_Wait_ContextCanceled(t *testing.T) {
	tracker := NewMemoryTracker()

	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "10")
	headers.Set("X-RateLimit-Remaining", "0")
	headers.Set("X-RateLimit-Reset-After", "10")
	headers.Set("X-RateLimit-Bucket", "test-bucket")

	tracker.Update("/test/route", headers)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := tracker.Wait(ctx, "test-bucket")
	if err == nil {
		t.Error("Expected context deadline exceeded error, got nil")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestMemoryTracker_GetBucket_NotFound(t *testing.T) {
	tracker := NewMemoryTracker()

	bucket := tracker.GetBucket("nonexistent")
	if bucket != nil {
		t.Errorf("Expected nil bucket, got %+v", bucket)
	}
}

func TestMemoryTracker_Clear(t *testing.T) {
	tracker := NewMemoryTracker()

	// Add some buckets
	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "10")
	headers.Set("X-RateLimit-Remaining", "5")
	headers.Set("X-RateLimit-Reset-After", "60")
	headers.Set("X-RateLimit-Bucket", "test-bucket-1")

	tracker.Update("/route1", headers)

	headers.Set("X-RateLimit-Bucket", "test-bucket-2")
	tracker.Update("/route2", headers)

	// Verify buckets exist
	if tracker.GetBucket("test-bucket-1") == nil {
		t.Error("Bucket 1 should exist before clear")
	}
	if tracker.GetBucket("test-bucket-2") == nil {
		t.Error("Bucket 2 should exist before clear")
	}

	// Clear
	tracker.Clear()

	// Verify buckets are gone
	if tracker.GetBucket("test-bucket-1") != nil {
		t.Error("Bucket 1 should be cleared")
	}
	if tracker.GetBucket("test-bucket-2") != nil {
		t.Error("Bucket 2 should be cleared")
	}
}

func TestMemoryTracker_CleanupExpired(t *testing.T) {
	tracker := NewMemoryTracker()

	// Add bucket that expires in past
	headers := make(http.Header)
	headers.Set("X-RateLimit-Limit", "10")
	headers.Set("X-RateLimit-Remaining", "5")
	headers.Set("X-RateLimit-Reset", "0") // Unix epoch (expired)
	headers.Set("X-RateLimit-Bucket", "expired-bucket")

	tracker.Update("/expired", headers)

	// Add a fresh bucket
	headers.Set("X-RateLimit-Reset-After", "60")
	headers.Set("X-RateLimit-Bucket", "fresh-bucket")
	tracker.Update("/fresh", headers)

	// Trigger cleanup by updating again
	tracker.Update("/trigger-cleanup", headers)

	// Expired bucket should be gone
	if tracker.GetBucket("expired-bucket") != nil {
		t.Error("Expired bucket should be cleaned up")
	}

	// Fresh bucket should still exist
	if tracker.GetBucket("fresh-bucket") == nil {
		t.Error("Fresh bucket should still exist")
	}
}

func TestParseIntHeader(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{"valid int", "42", 42},
		{"zero", "0", 0},
		{"empty", "", 0},
		{"invalid", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(http.Header)
			if tt.value != "" {
				headers.Set("Test-Header", tt.value)
			}

			got := parseIntHeader(headers, "Test-Header")
			if got != tt.want {
				t.Errorf("parseIntHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFloatHeader(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  float64
	}{
		{"valid float", "1.5", 1.5},
		{"integer", "42", 42.0},
		{"zero", "0", 0.0},
		{"empty", "", 0.0},
		{"invalid", "abc", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(http.Header)
			if tt.value != "" {
				headers.Set("Test-Header", tt.value)
			}

			got := parseFloatHeader(headers, "Test-Header")
			if got != tt.want {
				t.Errorf("parseFloatHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRouteFromEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		endpoint string
		want     string
	}{
		{
			name:     "simple route",
			method:   "GET",
			endpoint: "/channels/123/messages",
			want:     "GET:/channels/123/messages",
		},
		{
			name:     "post route",
			method:   "POST",
			endpoint: "/channels/456/messages",
			want:     "POST:/channels/456/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RouteFromEndpoint(tt.method, tt.endpoint)
			if got != tt.want {
				t.Errorf("RouteFromEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryTracker_Concurrent(t *testing.T) {
	tracker := NewMemoryTracker()

	// Test concurrent updates and reads
	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			headers := make(http.Header)
			headers.Set("X-RateLimit-Limit", "10")
			headers.Set("X-RateLimit-Remaining", "5")
			headers.Set("X-RateLimit-Reset-After", "60")
			headers.Set("X-RateLimit-Bucket", "test-bucket")

			for j := 0; j < 100; j++ {
				tracker.Update("/test/route", headers)
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				_ = tracker.GetBucket("test-bucket")
				ctx := context.Background()
				_ = tracker.Wait(ctx, "test-bucket")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
