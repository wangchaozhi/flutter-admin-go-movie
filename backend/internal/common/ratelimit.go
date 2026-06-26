package common

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SharedStore is an optional cluster-wide counter backend (e.g. Redis) used to
// enforce rate limits across instances. It is injected at startup via
// UseSharedStore so this package keeps no hard dependency on Redis; when unset,
// or when a call reports ok=false (backend unreachable), limiters transparently
// fall back to a process-local counter. Keys are already namespaced per limiter.
type SharedStore interface {
	// Incr increments the counter at key, applying ttl on first increment, and
	// returns the new count.
	Incr(key string, ttl time.Duration) (count int64, ok bool)
	// Count reads the current counter value (0 when absent).
	Count(key string) (count int64, ok bool)
	// TTL reports the remaining lifetime of key.
	TTL(key string) (ttl time.Duration, ok bool)
	// Del removes the counter at key.
	Del(key string)
}

var shared SharedStore

// UseSharedStore installs the shared rate-limit backend. Call once at startup
// before serving traffic; limiters read it dynamically so package-level limiters
// constructed at init time pick it up.
func UseSharedStore(s SharedStore) { shared = s }

// Limiter is a fixed-window counter used both as a login brute-force guard
// (count only failures via Fail, gate with Blocked, clear on success via Reset)
// and as a generic per-action throttle (count every call via Allow). When a
// shared store is installed it enforces the limit cluster-wide; otherwise it
// uses an in-memory map so limiting keeps working even when Redis is down.
type Limiter struct {
	name     string
	mu       sync.Mutex
	attempts map[string]*failureWindow
	max      int
	window   time.Duration
}

type failureWindow struct {
	count   int
	resetAt time.Time
}

// NewLimiter creates a limiter that trips a key once its count reaches max
// inside window. name namespaces the key so distinct limiters (e.g. admin vs
// mobile login) never share a counter in the shared store.
func NewLimiter(name string, max int, window time.Duration) *Limiter {
	return &Limiter{
		name:     name,
		attempts: make(map[string]*failureWindow),
		max:      max,
		window:   window,
	}
}

func (l *Limiter) sharedKey(key string) string {
	return "rl:" + l.name + ":" + key
}

// Blocked reports whether the key has reached the failure threshold within the
// current window, plus how long until it resets.
func (l *Limiter) Blocked(key string) (bool, time.Duration) {
	if s := shared; s != nil {
		if n, ok := s.Count(l.sharedKey(key)); ok {
			if n >= int64(l.max) {
				return true, l.retryAfter(s, key)
			}
			return false, 0
		}
	}
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

// Fail records one failed attempt, starting a fresh window when the previous one
// expired. It opportunistically sweeps stale entries so the map stays bounded.
func (l *Limiter) Fail(key string) {
	if s := shared; s != nil {
		if _, ok := s.Incr(l.sharedKey(key), l.window); ok {
			return
		}
	}
	l.localIncr(key)
}

// Allow records one action and reports whether it is still within max per
// window. Unlike Fail/Blocked, every call counts — use it to throttle actions
// such as posting comments.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if s := shared; s != nil {
		if n, ok := s.Incr(l.sharedKey(key), l.window); ok {
			if n > int64(l.max) {
				return false, l.retryAfter(s, key)
			}
			return true, 0
		}
	}
	count := l.localIncr(key)
	if count > l.max {
		l.mu.Lock()
		w := l.attempts[key]
		l.mu.Unlock()
		if w != nil {
			return false, time.Until(w.resetAt)
		}
		return false, l.window
	}
	return true, 0
}

// Reset clears recorded counts for the key, e.g. after a successful login.
func (l *Limiter) Reset(key string) {
	if s := shared; s != nil {
		s.Del(l.sharedKey(key))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func (l *Limiter) retryAfter(s SharedStore, key string) time.Duration {
	if ttl, ok := s.TTL(l.sharedKey(key)); ok && ttl > 0 {
		return ttl
	}
	return l.window
}

// localIncr increments the in-memory counter and returns the new count.
func (l *Limiter) localIncr(key string) int {
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
		return 1
	}
	w.count++
	return w.count
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
