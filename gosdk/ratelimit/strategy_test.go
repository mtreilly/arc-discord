package ratelimit

import (
	"testing"
	"time"
)

func TestReactiveStrategy(t *testing.T) {
	strategy := NewReactiveStrategy()

	if strategy.Name() != "reactive" {
		t.Errorf("expected name 'reactive', got '%s'", strategy.Name())
	}

	tests := []struct {
		name         string
		bucket       *Bucket
		shouldWait   bool
		expectWait   bool
		waitDuration time.Duration
	}{
		{
			name:         "nil bucket should not wait",
			bucket:       nil,
			shouldWait:   false,
			expectWait:   false,
			waitDuration: 0,
		},
		{
			name: "with remaining requests should not wait",
			bucket: &Bucket{
				Limit:     10,
				Remaining: 5,
				Reset:     time.Now().Add(30 * time.Second),
			},
			shouldWait:   false,
			expectWait:   false,
			waitDuration: 0,
		},
		{
			name: "no remaining requests should wait",
			bucket: &Bucket{
				Limit:     10,
				Remaining: 0,
				Reset:     time.Now().Add(30 * time.Second),
			},
			shouldWait:   true,
			expectWait:   true,
			waitDuration: 30 * time.Second,
		},
		{
			name: "expired bucket should not wait",
			bucket: &Bucket{
				Limit:     10,
				Remaining: 0,
				Reset:     time.Now().Add(-1 * time.Second),
			},
			shouldWait:   false,
			expectWait:   false,
			waitDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldWait := strategy.ShouldWait(tt.bucket)
			if shouldWait != tt.shouldWait {
				t.Errorf("ShouldWait() = %v, want %v", shouldWait, tt.shouldWait)
			}

			waitDuration := strategy.CalculateWait(tt.bucket)

			if tt.expectWait {
				// Allow 1 second tolerance for timing differences
				diff := waitDuration - tt.waitDuration
				if diff < -1*time.Second || diff > 1*time.Second {
					t.Errorf("CalculateWait() = %v, want ~%v", waitDuration, tt.waitDuration)
				}
			} else {
				if waitDuration != 0 {
					t.Errorf("CalculateWait() = %v, want 0", waitDuration)
				}
			}
		})
	}
}

func TestProactiveStrategy(t *testing.T) {
	t.Run("default proactive strategy", func(t *testing.T) {
		strategy := NewDefaultProactiveStrategy()

		if strategy.Name() != "proactive" {
			t.Errorf("expected name 'proactive', got '%s'", strategy.Name())
		}

		if strategy.Threshold != 0.1 {
			t.Errorf("expected default threshold 0.1, got %f", strategy.Threshold)
		}

		if strategy.SafetyMargin != 1 {
			t.Errorf("expected default safety margin 1, got %d", strategy.SafetyMargin)
		}
	})

	t.Run("custom proactive strategy", func(t *testing.T) {
		strategy := NewProactiveStrategy(0.2, 5)

		if strategy.Threshold != 0.2 {
			t.Errorf("expected threshold 0.2, got %f", strategy.Threshold)
		}

		if strategy.SafetyMargin != 5 {
			t.Errorf("expected safety margin 5, got %d", strategy.SafetyMargin)
		}
	})

	t.Run("threshold bounds checking", func(t *testing.T) {
		// Test negative threshold
		strategy := NewProactiveStrategy(-0.5, 1)
		if strategy.Threshold != 0 {
			t.Errorf("negative threshold should be clamped to 0, got %f", strategy.Threshold)
		}

		// Test threshold > 1
		strategy = NewProactiveStrategy(1.5, 1)
		if strategy.Threshold != 1 {
			t.Errorf("threshold > 1 should be clamped to 1, got %f", strategy.Threshold)
		}

		// Test negative safety margin
		strategy = NewProactiveStrategy(0.1, -5)
		if strategy.SafetyMargin != 0 {
			t.Errorf("negative safety margin should be clamped to 0, got %d", strategy.SafetyMargin)
		}
	})

	tests := []struct {
		name         string
		threshold    float64
		safetyMargin int
		bucket       *Bucket
		shouldWait   bool
	}{
		{
			name:         "nil bucket should not wait",
			threshold:    0.1,
			safetyMargin: 1,
			bucket:       nil,
			shouldWait:   false,
		},
		{
			name:         "well above threshold should not wait",
			threshold:    0.1,
			safetyMargin: 1,
			bucket: &Bucket{
				Limit:     100,
				Remaining: 50, // 50% remaining
				Reset:     time.Now().Add(30 * time.Second),
			},
			shouldWait: false,
		},
		{
			name:         "at threshold should wait",
			threshold:    0.1,
			safetyMargin: 1,
			bucket: &Bucket{
				Limit:     100,
				Remaining: 10, // 10% remaining (at threshold)
				Reset:     time.Now().Add(30 * time.Second),
			},
			shouldWait: true,
		},
		{
			name:         "below threshold should wait",
			threshold:    0.1,
			safetyMargin: 1,
			bucket: &Bucket{
				Limit:     100,
				Remaining: 5, // 5% remaining (below threshold)
				Reset:     time.Now().Add(30 * time.Second),
			},
			shouldWait: true,
		},
		{
			name:         "at safety margin should wait",
			threshold:    0.1,
			safetyMargin: 5,
			bucket: &Bucket{
				Limit:     100,
				Remaining: 5, // At safety margin
				Reset:     time.Now().Add(30 * time.Second),
			},
			shouldWait: true,
		},
		{
			name:         "below safety margin should wait",
			threshold:    0.1,
			safetyMargin: 5,
			bucket: &Bucket{
				Limit:     100,
				Remaining: 3, // Below safety margin
				Reset:     time.Now().Add(30 * time.Second),
			},
			shouldWait: true,
		},
		{
			name:         "expired bucket should not wait",
			threshold:    0.1,
			safetyMargin: 1,
			bucket: &Bucket{
				Limit:     100,
				Remaining: 5,
				Reset:     time.Now().Add(-1 * time.Second),
			},
			shouldWait: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewProactiveStrategy(tt.threshold, tt.safetyMargin)

			shouldWait := strategy.ShouldWait(tt.bucket)
			if shouldWait != tt.shouldWait {
				t.Errorf("ShouldWait() = %v, want %v", shouldWait, tt.shouldWait)
			}

			waitDuration := strategy.CalculateWait(tt.bucket)
			if tt.shouldWait {
				if waitDuration <= 0 {
					t.Errorf("CalculateWait() = %v, want > 0", waitDuration)
				}
			}
		})
	}
}

func TestProactiveStrategyWaitCalculation(t *testing.T) {
	strategy := NewProactiveStrategy(0.1, 1)

	t.Run("proportional wait time", func(t *testing.T) {
		// Test that wait time increases as we get closer to the limit
		bucket1 := &Bucket{
			Limit:     100,
			Remaining: 10, // 10% - at threshold
			Reset:     time.Now().Add(60 * time.Second),
		}

		bucket2 := &Bucket{
			Limit:     100,
			Remaining: 5, // 5% - halfway to 0
			Reset:     time.Now().Add(60 * time.Second),
		}

		bucket3 := &Bucket{
			Limit:     100,
			Remaining: 1, // 1% - almost at 0
			Reset:     time.Now().Add(60 * time.Second),
		}

		wait1 := strategy.CalculateWait(bucket1)
		wait2 := strategy.CalculateWait(bucket2)
		wait3 := strategy.CalculateWait(bucket3)

		// Wait time should increase as remaining decreases
		if wait1 >= wait2 {
			t.Errorf("expected wait1 (%v) < wait2 (%v)", wait1, wait2)
		}
		if wait2 >= wait3 {
			t.Errorf("expected wait2 (%v) < wait3 (%v)", wait2, wait3)
		}
	})
}

func TestAdaptiveStrategy(t *testing.T) {
	t.Run("default adaptive strategy", func(t *testing.T) {
		strategy := NewDefaultAdaptiveStrategy()

		if strategy.Name() != "adaptive" {
			t.Errorf("expected name 'adaptive', got '%s'", strategy.Name())
		}

		if strategy.MinThreshold != 0.05 {
			t.Errorf("expected default min threshold 0.05, got %f", strategy.MinThreshold)
		}

		if strategy.MaxThreshold != 0.3 {
			t.Errorf("expected default max threshold 0.3, got %f", strategy.MaxThreshold)
		}

		if strategy.LearningWindow != 50 {
			t.Errorf("expected default learning window 50, got %d", strategy.LearningWindow)
		}

		// Current threshold should start in the middle
		expectedCurrent := (strategy.MinThreshold + strategy.MaxThreshold) / 2
		if strategy.CurrentThreshold != expectedCurrent {
			t.Errorf("expected current threshold %f, got %f", expectedCurrent, strategy.CurrentThreshold)
		}
	})

	t.Run("custom adaptive strategy", func(t *testing.T) {
		strategy := NewAdaptiveStrategy(0.1, 0.5, 100)

		if strategy.MinThreshold != 0.1 {
			t.Errorf("expected min threshold 0.1, got %f", strategy.MinThreshold)
		}

		if strategy.MaxThreshold != 0.5 {
			t.Errorf("expected max threshold 0.5, got %f", strategy.MaxThreshold)
		}

		if strategy.LearningWindow != 100 {
			t.Errorf("expected learning window 100, got %d", strategy.LearningWindow)
		}
	})

	t.Run("threshold bounds validation", func(t *testing.T) {
		// Min > Max should swap
		strategy := NewAdaptiveStrategy(0.5, 0.1, 100)
		if strategy.MinThreshold >= strategy.MaxThreshold {
			t.Errorf("min threshold should be less than max threshold")
		}

		// Negative min should be clamped
		strategy = NewAdaptiveStrategy(-0.5, 0.5, 100)
		if strategy.MinThreshold < 0 {
			t.Errorf("negative min threshold should be adjusted")
		}

		// Max > 1 should be clamped
		strategy = NewAdaptiveStrategy(0.1, 1.5, 100)
		if strategy.MaxThreshold > 1 {
			t.Errorf("max threshold > 1 should be clamped")
		}

		// Small learning window should be adjusted
		strategy = NewAdaptiveStrategy(0.1, 0.5, 5)
		if strategy.LearningWindow < 10 {
			t.Errorf("small learning window should be adjusted to at least 10")
		}
	})

	t.Run("should wait based on current threshold", func(t *testing.T) {
		strategy := NewAdaptiveStrategy(0.05, 0.3, 10)

		bucket := &Bucket{
			Limit:     100,
			Remaining: 15, // 15% remaining
			Reset:     time.Now().Add(30 * time.Second),
		}

		// Current threshold starts at 0.175 (midpoint)
		// So 15% remaining should trigger wait
		shouldWait := strategy.ShouldWait(bucket)
		if !shouldWait {
			t.Errorf("ShouldWait() = false, want true for 15%% remaining with threshold ~17.5%%")
		}
	})
}

func TestAdaptiveStrategyLearning(t *testing.T) {
	strategy := NewAdaptiveStrategy(0.05, 0.3, 10)

	bucket := &Bucket{
		Limit:     100,
		Remaining: 50,
		Reset:     time.Now().Add(60 * time.Second),
	}

	initialThreshold := strategy.CurrentThreshold

	t.Run("recording successful requests", func(t *testing.T) {
		// Record 10 successful requests
		for i := 0; i < 10; i++ {
			strategy.RecordRequest(bucket, false)
		}

		stats := strategy.GetStats()

		if stats.SuccessfulRequests != 10 {
			t.Errorf("expected 10 successful requests, got %d", stats.SuccessfulRequests)
		}

		if stats.RateLimitHits != 0 {
			t.Errorf("expected 0 rate limit hits, got %d", stats.RateLimitHits)
		}

		if stats.HitRate != 0 {
			t.Errorf("expected hit rate 0, got %f", stats.HitRate)
		}

		// With all successful requests, threshold should decrease (be more aggressive)
		if strategy.CurrentThreshold >= initialThreshold {
			t.Errorf("threshold should decrease with successful requests, was %f, now %f",
				initialThreshold, strategy.CurrentThreshold)
		}
	})

	t.Run("recording rate limit hits", func(t *testing.T) {
		strategy := NewAdaptiveStrategy(0.05, 0.3, 10)
		initialThreshold := strategy.CurrentThreshold

		// Record 5 successful, 5 rate limit hits
		for i := 0; i < 10; i++ {
			hitLimit := i >= 5
			strategy.RecordRequest(bucket, hitLimit)
		}

		stats := strategy.GetStats()

		if stats.SuccessfulRequests != 5 {
			t.Errorf("expected 5 successful requests, got %d", stats.SuccessfulRequests)
		}

		if stats.RateLimitHits != 5 {
			t.Errorf("expected 5 rate limit hits, got %d", stats.RateLimitHits)
		}

		if stats.HitRate != 0.5 {
			t.Errorf("expected hit rate 0.5, got %f", stats.HitRate)
		}

		// With high hit rate, threshold should increase (be more conservative)
		if strategy.CurrentThreshold <= initialThreshold {
			t.Errorf("threshold should increase with rate limit hits, was %f, now %f",
				initialThreshold, strategy.CurrentThreshold)
		}
	})

	t.Run("threshold stays within bounds", func(t *testing.T) {
		strategy := NewAdaptiveStrategy(0.05, 0.3, 10)

		// Record many rate limit hits to push threshold to max
		for i := 0; i < 100; i++ {
			strategy.RecordRequest(bucket, true)
		}

		if strategy.CurrentThreshold > strategy.MaxThreshold {
			t.Errorf("threshold exceeded max: %f > %f",
				strategy.CurrentThreshold, strategy.MaxThreshold)
		}

		// Record many successes to push threshold to min
		strategy = NewAdaptiveStrategy(0.05, 0.3, 10)
		for i := 0; i < 100; i++ {
			strategy.RecordRequest(bucket, false)
		}

		if strategy.CurrentThreshold < strategy.MinThreshold {
			t.Errorf("threshold went below min: %f < %f",
				strategy.CurrentThreshold, strategy.MinThreshold)
		}
	})

	t.Run("history window limiting", func(t *testing.T) {
		strategy := NewAdaptiveStrategy(0.05, 0.3, 10)

		// Record 20 requests (more than learning window)
		for i := 0; i < 20; i++ {
			strategy.RecordRequest(bucket, false)
		}

		stats := strategy.GetStats()

		// History should be limited to learning window
		if stats.HistorySize > strategy.LearningWindow {
			t.Errorf("history size %d exceeds learning window %d",
				stats.HistorySize, strategy.LearningWindow)
		}

		// But total counters should still be accurate
		if stats.SuccessfulRequests != 20 {
			t.Errorf("expected 20 successful requests, got %d", stats.SuccessfulRequests)
		}
	})
}

func TestAdaptiveStrategyWaitCalculation(t *testing.T) {
	strategy := NewAdaptiveStrategy(0.05, 0.3, 10)

	t.Run("wait increases with hit rate", func(t *testing.T) {
		bucket := &Bucket{
			Limit:     100,
			Remaining: 5, // 5% remaining
			Reset:     time.Now().Add(60 * time.Second),
		}

		// Get initial wait time with no history
		initialWait := strategy.CalculateWait(bucket)

		// Record many rate limit hits
		for i := 0; i < 10; i++ {
			strategy.RecordRequest(bucket, true)
		}

		// Wait time should increase with high hit rate
		newWait := strategy.CalculateWait(bucket)

		if newWait <= initialWait {
			t.Errorf("wait time should increase with high hit rate, initial: %v, new: %v",
				initialWait, newWait)
		}
	})

	t.Run("no wait above threshold", func(t *testing.T) {
		strategy := NewAdaptiveStrategy(0.05, 0.3, 10)

		bucket := &Bucket{
			Limit:     100,
			Remaining: 50, // 50% remaining (well above any threshold)
			Reset:     time.Now().Add(60 * time.Second),
		}

		wait := strategy.CalculateWait(bucket)
		if wait != 0 {
			t.Errorf("expected no wait above threshold, got %v", wait)
		}
	})
}

func BenchmarkReactiveStrategy(b *testing.B) {
	strategy := NewReactiveStrategy()
	bucket := &Bucket{
		Limit:     100,
		Remaining: 0,
		Reset:     time.Now().Add(30 * time.Second),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldWait(bucket)
		strategy.CalculateWait(bucket)
	}
}

func BenchmarkProactiveStrategy(b *testing.B) {
	strategy := NewDefaultProactiveStrategy()
	bucket := &Bucket{
		Limit:     100,
		Remaining: 5,
		Reset:     time.Now().Add(30 * time.Second),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldWait(bucket)
		strategy.CalculateWait(bucket)
	}
}

func BenchmarkAdaptiveStrategy(b *testing.B) {
	strategy := NewDefaultAdaptiveStrategy()
	bucket := &Bucket{
		Limit:     100,
		Remaining: 15,
		Reset:     time.Now().Add(30 * time.Second),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldWait(bucket)
		strategy.CalculateWait(bucket)
	}
}

func BenchmarkAdaptiveStrategyWithLearning(b *testing.B) {
	strategy := NewDefaultAdaptiveStrategy()
	bucket := &Bucket{
		Limit:     100,
		Remaining: 15,
		Reset:     time.Now().Add(30 * time.Second),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.RecordRequest(bucket, i%10 == 0) // 10% hit rate
		strategy.ShouldWait(bucket)
		strategy.CalculateWait(bucket)
	}
}
