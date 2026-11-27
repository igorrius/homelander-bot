package ratelimit

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newTest() (*Limiter, *fakeClock) {
	c := &fakeClock{t: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
	return New(c.now), c
}

func TestAllowWithinLimit(t *testing.T) {
	l, _ := newTest()
	for i := range 3 {
		if !l.Allow("k", 3, time.Minute) {
			t.Fatalf("hit %d refused", i+1)
		}
	}
	if l.Allow("k", 3, time.Minute) {
		t.Fatal("4th hit allowed")
	}
}

func TestAllowWindowSlides(t *testing.T) {
	l, c := newTest()
	l.Allow("k", 1, time.Minute)
	if l.Allow("k", 1, time.Minute) {
		t.Fatal("second hit in window allowed")
	}
	c.t = c.t.Add(time.Minute + time.Second)
	if !l.Allow("k", 1, time.Minute) {
		t.Fatal("hit after window refused")
	}
}

func TestAllowKeysIndependent(t *testing.T) {
	l, _ := newTest()
	l.Allow("a", 1, time.Minute)
	if !l.Allow("b", 1, time.Minute) {
		t.Fatal("key b limited by key a")
	}
}

func TestRefusedHitsDoNotExtendBlock(t *testing.T) {
	l, c := newTest()
	l.Allow("k", 1, time.Minute)
	for range 5 {
		c.t = c.t.Add(10 * time.Second)
		l.Allow("k", 1, time.Minute)
	}
	c.t = c.t.Add(11 * time.Second) // 61 s after the only allowed hit
	if !l.Allow("k", 1, time.Minute) {
		t.Fatal("refused hits kept the key blocked")
	}
}

func TestAllowForgetsOldKeys(t *testing.T) {
	l, c := newTest()
	l.Allow("old", 1, time.Minute)
	c.t = c.t.Add(3 * time.Minute)
	l.Allow("new", 1, time.Minute)
	if n := l.size(); n != 1 {
		t.Fatalf("size = %d, want 1", n)
	}
}

func TestShortWindowCallDoesNotResetLongWindowKeys(t *testing.T) {
	l, c := newTest()
	for range 5 {
		l.Allow("apply:ip", 5, time.Hour)
	}
	c.t = c.t.Add(16 * time.Minute)
	l.Allow("code:other", 3, 15*time.Minute) // triggers a sweep with a short window
	if l.Allow("apply:ip", 5, time.Hour) {
		t.Fatal("6th hit within the hour allowed after a short-window sweep")
	}
}
