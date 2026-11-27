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
	hits map[string]*entry
	last time.Time // last sweep of idle keys
}

// entry is one key's recent hits and the window it is counted in.
type entry struct {
	window time.Duration
	hits   []time.Time
}

// sweepEvery bounds how often idle keys are looked for.
const sweepEvery = time.Minute

// New returns a limiter reading time from now.
func New(now func() time.Time) *Limiter {
	return &Limiter{now: now, hits: map[string]*entry{}}
}

// Allow records a hit for key and reports whether it is within limit hits per
// window. A refused hit is not recorded, so it does not extend a block.
func (l *Limiter) Allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	e := l.hits[key]
	if e == nil {
		e = &entry{}
		l.hits[key] = e
	}
	e.window = max(e.window, window)
	e.hits = prune(e.hits, now.Add(-window))
	if len(e.hits) >= limit {
		return false
	}
	e.hits = append(e.hits, now)
	return true
}

func prune(ts []time.Time, after time.Time) []time.Time {
	i := 0
	for i < len(ts) && !ts[i].After(after) {
		i++
	}
	return ts[i:]
}

// sweep drops keys whose newest hit is older than the key's own window.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.last) < sweepEvery {
		return
	}
	l.last = now
	for k, e := range l.hits {
		if len(e.hits) == 0 || !e.hits[len(e.hits)-1].After(now.Add(-e.window)) {
			delete(l.hits, k)
		}
	}
}
