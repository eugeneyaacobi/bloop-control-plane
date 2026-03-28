package security

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	buckets map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		window:  window,
		limit:   limit,
		buckets: make(map[string][]time.Time),
	}
}

func (r *RateLimiter) Allow(key string, now time.Time) bool {
	if r == nil || r.limit <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-r.window)
	entries := r.buckets[key][:0]
	for _, ts := range r.buckets[key] {
		if ts.After(cutoff) {
			entries = append(entries, ts)
		}
	}
	if len(entries) >= r.limit {
		r.buckets[key] = entries
		return false
	}
	entries = append(entries, now)
	r.buckets[key] = entries
	return true
}
