package common

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LoginLimiter is a process-local fixed-window failure counter used to slow down
// credential brute-force on the login endpoints. It counts only failed attempts
// per key (typically client IP) and blocks further tries once the threshold is
// reached within the window.
//
// It is intentionally in-memory so that login keeps working even when Redis is
// down. For multi-instance deployments this should be backed by a shared store
// (e.g. Redis) so the limit is enforced cluster-wide.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*failureWindow
	max      int
	window   time.Duration
}

type failureWindow struct {
	count   int
	resetAt time.Time
}

// NewLoginLimiter creates a limiter that blocks a key after max failures inside
// the rolling window.
func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{
		attempts: make(map[string]*failureWindow),
		max:      max,
		window:   window,
	}
}

// Blocked reports whether the key has exceeded the failure threshold within the
// current window, plus how long until the window resets.
func (l *LoginLimiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.attempts[key]
	if w == nil || time.Now().After(w.resetAt) {
		return false, 0
	}
	if w.count >= l.max {
		return true, time.Until(w.resetAt)
	}
	return false, 0
}

// Fail records one failed attempt for the key, starting a fresh window when the
// previous one has expired. It opportunistically sweeps stale entries so the map
// does not grow without bound.
func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.attempts) > 1024 {
		for k, w := range l.attempts {
			if now.After(w.resetAt) {
				delete(l.attempts, k)
			}
		}
	}
	w := l.attempts[key]
	if w == nil || now.After(w.resetAt) {
		l.attempts[key] = &failureWindow{count: 1, resetAt: now.Add(l.window)}
		return
	}
	w.count++
}

// Reset clears recorded failures for the key, e.g. after a successful login.
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// ClientIP best-effort extracts the originating client IP, honouring the first
// hop in X-Forwarded-For when present (set by a trusted reverse proxy) and
// falling back to the transport remote address.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
