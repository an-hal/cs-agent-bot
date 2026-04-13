package auth

import (
	"sync"
	"time"
)

// RateLimiter implements a sliding-window in-memory rate limiter keyed by
// arbitrary strings (typically client IPs or normalized emails).
//
// It mirrors the BFF rate limiter (sliding window log, 60s default). It is
// safe for concurrent use. We deliberately keep it in-process (no Redis) so
// the dashboard backend has no new infra dependency for this single feature.
type RateLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	max      int
	hits     map[string][]time.Time
	lastSwept time.Time
	now      func() time.Time
}

// NewRateLimiter constructs a rate limiter with the given window and max
// requests per window.
func NewRateLimiter(window time.Duration, max int) *RateLimiter {
	return &RateLimiter{
		window: window,
		max:    max,
		hits:   make(map[string][]time.Time),
		now:    time.Now,
	}
}

// Allow records a request for `key` and returns whether it is within the
// configured budget. Remaining is the number of requests still permitted in
// the current window after this call. ResetUnix is the unix timestamp (seconds)
// when the oldest in-window hit will expire.
func (l *RateLimiter) Allow(key string) (allowed bool, remaining int, resetUnix int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	// Periodic sweep to keep the map from growing unbounded.
	if now.Sub(l.lastSwept) > 5*time.Minute {
		l.sweepLocked(cutoff)
		l.lastSwept = now
	}

	hits := l.hits[key]
	pruned := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}

	if len(pruned) >= l.max {
		oldest := pruned[0]
		l.hits[key] = pruned
		return false, 0, oldest.Add(l.window).Unix()
	}

	pruned = append(pruned, now)
	l.hits[key] = pruned
	remaining = l.max - len(pruned)
	resetUnix = pruned[0].Add(l.window).Unix()
	return true, remaining, resetUnix
}

// Limit returns the configured max requests per window.
func (l *RateLimiter) Limit() int { return l.max }

func (l *RateLimiter) sweepLocked(cutoff time.Time) {
	for k, ts := range l.hits {
		filtered := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			delete(l.hits, k)
		} else {
			l.hits[k] = filtered
		}
	}
}
