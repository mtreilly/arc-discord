package ratelimit

import (
	"sync"
	"time"
)

// Strategy defines how the rate limiter should behave when approaching limits
type Strategy interface {
	// ShouldWait returns true if we should wait before making a request
	ShouldWait(bucket *Bucket) bool

	// CalculateWait returns how long to wait based on the bucket state
	CalculateWait(bucket *Bucket) time.Duration

	// Name returns the name of the strategy
	Name() string
}

// ReactiveStrategy waits only when we hit the rate limit (Remaining = 0)
// This is the simplest strategy - wait only when absolutely necessary
type ReactiveStrategy struct{}

// NewReactiveStrategy creates a new reactive rate limiting strategy
func NewReactiveStrategy() *ReactiveStrategy {
	return &ReactiveStrategy{}
}

// ShouldWait returns true only when we've exhausted the rate limit
func (s *ReactiveStrategy) ShouldWait(bucket *Bucket) bool {
	if bucket == nil {
		return false
	}
	// Only wait if we have no remaining requests and reset is in the future
	return bucket.Remaining == 0 && time.Now().Before(bucket.Reset)
}

// CalculateWait returns the time until the bucket resets
func (s *ReactiveStrategy) CalculateWait(bucket *Bucket) time.Duration {
	if bucket == nil || time.Now().After(bucket.Reset) {
		return 0
	}
	// Only return wait time if we should actually wait
	if !s.ShouldWait(bucket) {
		return 0
	}
	return time.Until(bucket.Reset)
}

// Name returns the strategy name
func (s *ReactiveStrategy) Name() string {
	return "reactive"
}

// ProactiveStrategy waits before hitting the rate limit to prevent 429 errors
// This strategy waits when we get close to the limit (configurable threshold)
type ProactiveStrategy struct {
	// Threshold is the percentage of limit at which we start waiting
	// For example, 0.1 means wait when we have 10% or less remaining
	Threshold float64

	// SafetyMargin is the number of requests to keep in reserve
	// For example, 1 means always keep at least 1 request available
	SafetyMargin int
}

// NewProactiveStrategy creates a new proactive rate limiting strategy
// threshold: percentage of remaining requests at which to start waiting (0.0-1.0)
// safetyMargin: minimum number of requests to keep in reserve
func NewProactiveStrategy(threshold float64, safetyMargin int) *ProactiveStrategy {
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1 {
		threshold = 1
	}
	if safetyMargin < 0 {
		safetyMargin = 0
	}

	return &ProactiveStrategy{
		Threshold:    threshold,
		SafetyMargin: safetyMargin,
	}
}

// NewDefaultProactiveStrategy creates a proactive strategy with sensible defaults
// - Wait when 10% or less requests remaining
// - Keep at least 1 request in reserve
func NewDefaultProactiveStrategy() *ProactiveStrategy {
	return NewProactiveStrategy(0.1, 1)
}

// ShouldWait returns true when we're approaching the rate limit
func (s *ProactiveStrategy) ShouldWait(bucket *Bucket) bool {
	if bucket == nil || bucket.Limit == 0 {
		return false
	}

	// Don't wait if the bucket has already reset
	if time.Now().After(bucket.Reset) {
		return false
	}

	// Check safety margin first
	if bucket.Remaining <= s.SafetyMargin {
		return true
	}

	// Check threshold percentage
	remainingPercent := float64(bucket.Remaining) / float64(bucket.Limit)
	return remainingPercent <= s.Threshold
}

// CalculateWait returns how long to wait based on the bucket state
// Uses a proportional wait time - the closer to the limit, the longer the wait
func (s *ProactiveStrategy) CalculateWait(bucket *Bucket) time.Duration {
	if bucket == nil || time.Now().After(bucket.Reset) {
		return 0
	}

	// If we're at or below safety margin, wait until reset
	if bucket.Remaining <= s.SafetyMargin {
		return time.Until(bucket.Reset)
	}

	// Calculate proportional wait time based on how close we are to the threshold
	remainingPercent := float64(bucket.Remaining) / float64(bucket.Limit)
	if remainingPercent > s.Threshold {
		return 0
	}

	// Wait time is proportional to how far below threshold we are
	// At threshold: minimal wait (10%), at 0: full wait until reset (100%)
	thresholdDistance := s.Threshold - remainingPercent

	// Ensure minimum wait ratio of 10% when at or near threshold
	waitRatio := thresholdDistance / s.Threshold
	if waitRatio < 0.1 {
		waitRatio = 0.1
	}

	fullWait := time.Until(bucket.Reset)
	return time.Duration(float64(fullWait) * waitRatio)
}

// Name returns the strategy name
func (s *ProactiveStrategy) Name() string {
	return "proactive"
}

// AdaptiveStrategy learns from request patterns and adapts wait times
// This strategy tracks request history and adjusts behavior dynamically
type AdaptiveStrategy struct {
	mu sync.RWMutex

	// MinThreshold is the minimum percentage threshold
	MinThreshold float64

	// MaxThreshold is the maximum percentage threshold
	MaxThreshold float64

	// CurrentThreshold is the current adaptive threshold
	CurrentThreshold float64

	// LearningWindow is how many requests to consider for adaptation
	LearningWindow int

	// requestHistory tracks recent request outcomes
	requestHistory []requestOutcome

	// rateLimitHits counts how many times we've hit rate limits
	rateLimitHits int

	// successfulRequests counts successful requests
	successfulRequests int

	// AdjustmentFactor determines how quickly we adapt (0.0-1.0)
	AdjustmentFactor float64
}

type requestOutcome struct {
	timestamp   time.Time
	hitLimit    bool
	remaining   int
	limit       int
	resetAfter  time.Duration
}

// NewAdaptiveStrategy creates a new adaptive rate limiting strategy
func NewAdaptiveStrategy(minThreshold, maxThreshold float64, learningWindow int) *AdaptiveStrategy {
	if minThreshold < 0 {
		minThreshold = 0.05
	}
	if maxThreshold > 1 {
		maxThreshold = 0.3
	}
	if minThreshold > maxThreshold {
		minThreshold, maxThreshold = maxThreshold, minThreshold
	}
	if learningWindow < 10 {
		learningWindow = 10
	}

	// Start in the middle
	currentThreshold := (minThreshold + maxThreshold) / 2

	return &AdaptiveStrategy{
		MinThreshold:     minThreshold,
		MaxThreshold:     maxThreshold,
		CurrentThreshold: currentThreshold,
		LearningWindow:   learningWindow,
		requestHistory:   make([]requestOutcome, 0, learningWindow),
		AdjustmentFactor: 0.1, // 10% adjustment per learning cycle
	}
}

// NewDefaultAdaptiveStrategy creates an adaptive strategy with sensible defaults
func NewDefaultAdaptiveStrategy() *AdaptiveStrategy {
	return NewAdaptiveStrategy(0.05, 0.3, 50)
}

// ShouldWait returns true based on the current adaptive threshold
func (s *AdaptiveStrategy) ShouldWait(bucket *Bucket) bool {
	if bucket == nil || bucket.Limit == 0 {
		return false
	}

	// Don't wait if the bucket has already reset
	if time.Now().After(bucket.Reset) {
		return false
	}

	s.mu.RLock()
	threshold := s.CurrentThreshold
	s.mu.RUnlock()

	// Check if we're below the adaptive threshold
	remainingPercent := float64(bucket.Remaining) / float64(bucket.Limit)
	return remainingPercent <= threshold
}

// CalculateWait returns how long to wait based on adaptive learning
func (s *AdaptiveStrategy) CalculateWait(bucket *Bucket) time.Duration {
	if bucket == nil || time.Now().After(bucket.Reset) {
		return 0
	}

	s.mu.RLock()
	threshold := s.CurrentThreshold
	s.mu.RUnlock()

	remainingPercent := float64(bucket.Remaining) / float64(bucket.Limit)

	// If we're above threshold, no wait
	if remainingPercent > threshold {
		return 0
	}

	// Calculate wait time based on how far below threshold we are
	thresholdDistance := threshold - remainingPercent
	waitRatio := thresholdDistance / threshold

	fullWait := time.Until(bucket.Reset)

	// Use adaptive factor to adjust wait time based on recent history
	s.mu.RLock()
	hitRate := s.calculateHitRate()
	s.mu.RUnlock()

	// If we're hitting limits frequently, increase wait time
	adaptiveFactor := 1.0 + (hitRate * 0.5) // Up to 50% longer waits if hitting limits

	return time.Duration(float64(fullWait) * waitRatio * adaptiveFactor)
}

// RecordRequest records the outcome of a request for learning
func (s *AdaptiveStrategy) RecordRequest(bucket *Bucket, hitLimit bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	outcome := requestOutcome{
		timestamp: time.Now(),
		hitLimit:  hitLimit,
	}

	if bucket != nil {
		outcome.remaining = bucket.Remaining
		outcome.limit = bucket.Limit
		outcome.resetAfter = time.Until(bucket.Reset)
	}

	// Add to history
	s.requestHistory = append(s.requestHistory, outcome)

	// Trim history to learning window
	if len(s.requestHistory) > s.LearningWindow {
		s.requestHistory = s.requestHistory[1:]
	}

	// Update counters
	if hitLimit {
		s.rateLimitHits++
	} else {
		s.successfulRequests++
	}

	// Adapt threshold if we have enough history
	if len(s.requestHistory) >= s.LearningWindow {
		s.adaptThreshold()
	}
}

// adaptThreshold adjusts the threshold based on recent request history
func (s *AdaptiveStrategy) adaptThreshold() {
	hitRate := s.calculateHitRate()

	// If we're hitting rate limits too often, increase threshold (be more conservative)
	// If we're not hitting limits, decrease threshold (be more aggressive)

	const targetHitRate = 0.01 // Target: less than 1% rate limit hits

	if hitRate > targetHitRate {
		// Increase threshold (be more conservative)
		adjustment := s.AdjustmentFactor * (hitRate / targetHitRate)
		s.CurrentThreshold += adjustment

		if s.CurrentThreshold > s.MaxThreshold {
			s.CurrentThreshold = s.MaxThreshold
		}
	} else if hitRate < targetHitRate && s.CurrentThreshold > s.MinThreshold {
		// Decrease threshold (be more aggressive)
		adjustment := s.AdjustmentFactor * (1.0 - hitRate/targetHitRate)
		s.CurrentThreshold -= adjustment

		if s.CurrentThreshold < s.MinThreshold {
			s.CurrentThreshold = s.MinThreshold
		}
	}
}

// calculateHitRate returns the rate limit hit rate from recent history
func (s *AdaptiveStrategy) calculateHitRate() float64 {
	if len(s.requestHistory) == 0 {
		return 0
	}

	hits := 0
	for _, outcome := range s.requestHistory {
		if outcome.hitLimit {
			hits++
		}
	}

	return float64(hits) / float64(len(s.requestHistory))
}

// GetStats returns statistics about the adaptive strategy
func (s *AdaptiveStrategy) GetStats() AdaptiveStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return AdaptiveStats{
		CurrentThreshold:   s.CurrentThreshold,
		MinThreshold:       s.MinThreshold,
		MaxThreshold:       s.MaxThreshold,
		HistorySize:        len(s.requestHistory),
		RateLimitHits:      s.rateLimitHits,
		SuccessfulRequests: s.successfulRequests,
		HitRate:            s.calculateHitRate(),
	}
}

// AdaptiveStats contains statistics about the adaptive strategy
type AdaptiveStats struct {
	CurrentThreshold   float64
	MinThreshold       float64
	MaxThreshold       float64
	HistorySize        int
	RateLimitHits      int
	SuccessfulRequests int
	HitRate            float64
}

// Name returns the strategy name
func (s *AdaptiveStrategy) Name() string {
	return "adaptive"
}
