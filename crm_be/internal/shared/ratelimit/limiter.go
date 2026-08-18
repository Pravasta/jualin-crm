// Package ratelimit provides in-memory rate limiting — no Redis (freeze:
// no infrastructure beyond PostgreSQL in the MVP). Hidden behind an
// interface so a distributed implementation can replace it later without
// touching any caller.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter reports whether the caller identified by key may proceed.
// Every login/register/resend/forgot-password call site checks its own
// keys (e.g. "register:ip:1.2.3.4", "resend:email:a@b.com") — the
// limiter itself has no notion of what a key represents.
type Limiter interface {
	Allow(key string) bool
}

// FixedWindow is a simple fixed-window counter: at most Limit calls to
// Allow with the same key succeed within any Window-sized bucket of wall
// clock time. Simplicity over precision is deliberate — this only needs
// to make brute-forcing and spam expensive, not implement a perfectly
// smooth rate.
type FixedWindow struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	windowStart time.Time
	count       int
}

func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

func (f *FixedWindow) Allow(key string) bool {
	now := time.Now()

	f.mu.Lock()
	defer f.mu.Unlock()

	b, ok := f.buckets[key]
	if !ok || now.Sub(b.windowStart) >= f.window {
		f.buckets[key] = &bucket{windowStart: now, count: 1}
		return true
	}

	if b.count >= f.limit {
		return false
	}
	b.count++
	return true
}
