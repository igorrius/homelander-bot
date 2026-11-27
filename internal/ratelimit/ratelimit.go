// Package ratelimit counts hits per key in a sliding window, in memory.
// It is enough for one replica; state is lost on restart.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter counts allowed hits per key.
type Limiter struct {
	mu   sync.Mutex
	now  func() time.Time
	hits map[string][]time.Time
	last time.Time // last full sweep of idle keys
}

// New returns a limiter reading time from now.
func New(now func() time.Time) *Limiter {
	return &Limiter{now: now, hits: map[string][]time.Time{}}
}

// Allow records a hit for key and reports whether it is within limit hits per
// window. A refused hit is not recorded, so it does not extend a block.
func (l *Limiter) Allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now, window)
	kept := prune(l.hits[key], now.Add(-window))
	if len(kept) >= limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

func prune(ts []time.Time, after time.Time) []time.Time {
	i := 0
	for i < len(ts) && !ts[i].After(after) {
		i++
	}
	return ts[i:]
}

// sweep drops keys whose newest hit is older than window, at most once per window.
func (l *Limiter) sweep(now time.Time, window time.Duration) {
	if now.Sub(l.last) < window {
		return
	}
	l.last = now
	for k, ts := range l.hits {
		if len(ts) == 0 || !ts[len(ts)-1].After(now.Add(-window)) {
			delete(l.hits, k)
		}
	}
}
