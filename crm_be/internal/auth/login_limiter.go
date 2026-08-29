package auth

import (
	"sync"
	"time"
)

// LoginLimiter is a progressive-backoff rate limiter for login attempts,
// distinct from ratelimit.FixedWindow because TD phase 1 §11 requires
// something FixedWindow's flat Allow(key) bool can't express: the wait
// grows with each consecutive failure and resets on success, with no
// permanent lockout (a permanent lockout turns brute-forcing into a
// denial-of-service against the legitimate account owner instead).
//
// This is a single concrete type, not a new interface — ratelimit.Limiter
// stays exactly as-is for register/resend/forgot-password, which only
// need a flat window.
//
// Keyed by IP and email — input from someone who hasn't logged in yet —
// so state is bounded with the same two-generation eviction as
// ratelimit.FixedWindow (Phase 4.5 #58, TD §2.3): without it, an
// attacker who never repeats a key can grow this map without limit.
type LoginLimiter struct {
	base time.Duration
	cap  time.Duration

	mu       sync.Mutex
	current  map[string]*loginBackoffState
	previous map[string]*loginBackoffState
	genStart time.Time

	// now is overridable only from this package's own tests
	// (login_limiter_internal_test.go) to drive generation rollover
	// deterministically — production code always gets time.Now.
	now func() time.Time
}

type loginBackoffState struct {
	failures      int
	nextAllowedAt time.Time
}

const (
	loginBackoffBase = time.Second
	loginBackoffCap  = 5 * time.Minute
)

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		base:     loginBackoffBase,
		cap:      loginBackoffCap,
		current:  make(map[string]*loginBackoffState),
		previous: make(map[string]*loginBackoffState),
		now:      time.Now,
	}
}

// Allow reports whether key may attempt a login right now. When it
// can't, retryAfter is how long the caller should wait.
func (l *LoginLimiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.rollLocked(now)

	s := l.getLocked(key, now)
	if s == nil || now.After(s.nextAllowedAt) || now.Equal(s.nextAllowedAt) {
		return true, 0
	}
	return false, s.nextAllowedAt.Sub(now)
}

// RecordFailure increases key's backoff. Doubling per failure, capped —
// no amount of continued failure ever locks the account out permanently.
func (l *LoginLimiter) RecordFailure(key string) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.rollLocked(now)

	s := l.getLocked(key, now)
	if s == nil {
		s = &loginBackoffState{}
		l.current[key] = s
	}
	s.failures++

	delay := l.base << (s.failures - 1) // #nosec G115 -- failures is bounded by real user retry cadence, not attacker-controlled input
	if delay > l.cap || delay <= 0 {    // delay <= 0 guards signed overflow on pathological failure counts
		delay = l.cap
	}
	s.nextAllowedAt = now.Add(delay)
}

// RecordSuccess clears key's backoff state entirely, from both
// generations — a key demoted to previous but not yet read back (and so
// not yet migrated into current) must still be cleared.
func (l *LoginLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.current, key)
	delete(l.previous, key)
}

// getLocked finds key's state, migrating it from the previous generation
// into current when its backoff is still active — the same
// carry-forward shape as ratelimit.FixedWindow.Take (Phase 4.5 TD §2.3),
// so an in-progress backoff is never shortened by a generation swap
// (decision H4: entries whose backoff has already lapsed are simply
// dropped, which is indistinguishable from letting them expire).
// Caller must hold l.mu.
func (l *LoginLimiter) getLocked(key string, now time.Time) *loginBackoffState {
	if s, ok := l.current[key]; ok {
		return s
	}
	if s, ok := l.previous[key]; ok && now.Before(s.nextAllowedAt) {
		l.current[key] = s
		return s
	}
	return nil
}

// rollLocked swaps generations roughly every loginBackoffCap — long
// enough that an active backoff (capped at the same duration) always
// survives at least one swap via getLocked's carry-forward. If more
// than 2*cap has passed with no activity at all, both generations are
// stale and are dropped outright. Caller must hold l.mu.
func (l *LoginLimiter) rollLocked(now time.Time) {
	if l.genStart.IsZero() {
		l.genStart = now
		return
	}
	if now.Sub(l.genStart) < l.cap {
		return
	}
	if now.Sub(l.genStart) >= 2*l.cap {
		l.previous = map[string]*loginBackoffState{}
	} else {
		l.previous = l.current
	}
	l.current = map[string]*loginBackoffState{}
	l.genStart = now
}

// Len reports how many distinct keys are currently tracked across both
// generations. It exists only so tests can prove eviction actually
// bounds growth (Phase 4.5 #58) — not a general-purpose introspection
// API, and no production call site needs it.
func (l *LoginLimiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	seen := make(map[string]struct{}, len(l.current)+len(l.previous))
	for k := range l.current {
		seen[k] = struct{}{}
	}
	for k := range l.previous {
		seen[k] = struct{}{}
	}
	return len(seen)
}
