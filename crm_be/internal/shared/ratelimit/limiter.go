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

// Allow is Take's boolean-only shortcut — every Phase 1 call site
// (register, resend, forgot-password) still only needs yes/no.
func (f *FixedWindow) Allow(key string) bool {
	return f.Take(key).Allowed
}

// Result is Take's richer answer — enough to populate the
// X-RateLimit-* response headers Phase 4's public API must send on
// every request (TD phase 4 §6), which Allow's bare bool never could.
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// Take is Allow's superset, added in Phase 4 (TD §6) once a second real
// caller needed more than yes/no (Rule #28) — same fixed-window bucket,
// same locking, just a richer return value. Allow is defined in terms of
// this, not the other way around, so there is exactly one place the
// bucket logic lives.
func (f *FixedWindow) Take(key string) Result {
	now := time.Now()

	f.mu.Lock()
	defer f.mu.Unlock()

	b, ok := f.buckets[key]
	if !ok || now.Sub(b.windowStart) >= f.window {
		b = &bucket{windowStart: now}
		f.buckets[key] = b
	}
	resetAt := b.windowStart.Add(f.window)

	if b.count >= f.limit {
		return Result{Allowed: false, Limit: f.limit, Remaining: 0, ResetAt: resetAt}
	}
	b.count++
	return Result{Allowed: true, Limit: f.limit, Remaining: f.limit - b.count, ResetAt: resetAt}
}
