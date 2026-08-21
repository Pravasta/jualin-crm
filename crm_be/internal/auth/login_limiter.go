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
type LoginLimiter struct {
	base time.Duration
	cap  time.Duration

	mu    sync.Mutex
	state map[string]*loginBackoffState
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
		base:  loginBackoffBase,
		cap:   loginBackoffCap,
		state: make(map[string]*loginBackoffState),
	}
}

// Allow reports whether key may attempt a login right now. When it
// can't, retryAfter is how long the caller should wait.
func (l *LoginLimiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	s, exists := l.state[key]
	if !exists || now.After(s.nextAllowedAt) || now.Equal(s.nextAllowedAt) {
		return true, 0
	}
	return false, s.nextAllowedAt.Sub(now)
}

// RecordFailure increases key's backoff. Doubling per failure, capped —
// no amount of continued failure ever locks the account out permanently.
func (l *LoginLimiter) RecordFailure(key string) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	s, exists := l.state[key]
	if !exists {
		s = &loginBackoffState{}
		l.state[key] = s
	}
	s.failures++

	delay := l.base << (s.failures - 1) // #nosec G115 -- failures is bounded by real user retry cadence, not attacker-controlled input
	if delay > l.cap || delay <= 0 {    // delay <= 0 guards signed overflow on pathological failure counts
		delay = l.cap
	}
	s.nextAllowedAt = now.Add(delay)
}

// RecordSuccess clears key's backoff state entirely.
func (l *LoginLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, key)
}
