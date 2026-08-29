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
//
// Keys are chosen by the caller — IP addresses and email addresses, for
// every existing call site (Phase 1 #9, Phase 4 #47's publiclead:key:
// being the one exception, keyed by an authenticated api_key_id). An IP
// or email is input from someone who hasn't authenticated yet, so the
// bucket map is bounded with two-generation eviction (Phase 4.5 #58,
// TD §2) — without it, an attacker who never repeats a key can grow this
// map without limit. See docs/phases/04.5-hardening/td.md §2 for why
// two generations rather than a periodic sweep (a sweep over a
// multi-million-key map while holding the lock turns the defense into
// the attack).
type FixedWindow struct {
	limit  int
	window time.Duration

	mu       sync.Mutex
	current  map[string]*bucket
	previous map[string]*bucket
	genStart time.Time

	// now is overridable only from this package's own tests
	// (limiter_internal_test.go) to drive generation rollover
	// deterministically — production code always gets time.Now.
	now func() time.Time
}

type bucket struct {
	windowStart time.Time
	count       int
}

func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{
		limit:    limit,
		window:   window,
		current:  make(map[string]*bucket),
		previous: make(map[string]*bucket),
		now:      time.Now,
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
	now := f.now()

	f.mu.Lock()
	defer f.mu.Unlock()

	f.rollLocked(now)

	b, ok := f.current[key]
	if !ok {
		// Carry a still-live bucket forward from the previous generation
		// so a generation swap never hands a key a fresh budget mid-window
		// — this is what keeps per-key behavior identical to before
		// eviction existed (Phase 4.5 TD §2.2).
		if pb, pok := f.previous[key]; pok && now.Sub(pb.windowStart) < f.window {
			b = &bucket{windowStart: pb.windowStart, count: pb.count}
		} else {
			b = &bucket{windowStart: now}
		}
		f.current[key] = b
	}
	if now.Sub(b.windowStart) >= f.window {
		b.windowStart = now
		b.count = 0
	}
	resetAt := b.windowStart.Add(f.window)

	if b.count >= f.limit {
		return Result{Allowed: false, Limit: f.limit, Remaining: 0, ResetAt: resetAt}
	}
	b.count++
	return Result{Allowed: true, Limit: f.limit, Remaining: f.limit - b.count, ResetAt: resetAt}
}

// rollLocked swaps generations roughly every window: current becomes
// previous (carrying still-live buckets forward on next access), and a
// fresh current starts. If more than 2*window has passed with no calls
// at all, both generations are stale and are dropped outright instead of
// demoting a previous that's just as dead. Caller must hold f.mu.
func (f *FixedWindow) rollLocked(now time.Time) {
	if f.genStart.IsZero() {
		f.genStart = now
		return
	}
	if now.Sub(f.genStart) < f.window {
		return
	}
	if now.Sub(f.genStart) >= 2*f.window {
		f.previous = map[string]*bucket{}
	} else {
		f.previous = f.current
	}
	f.current = map[string]*bucket{}
	f.genStart = now
}

// Len reports how many distinct keys are currently tracked across both
// generations. It exists only so tests can prove eviction actually
// bounds growth (Phase 4.5 #58) — not a general-purpose introspection
// API, and no production call site needs it.
func (f *FixedWindow) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := make(map[string]struct{}, len(f.current)+len(f.previous))
	for k := range f.current {
		seen[k] = struct{}{}
	}
	for k := range f.previous {
		seen[k] = struct{}{}
	}
	return len(seen)
}
