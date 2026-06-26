// Package cache wraps a shared Redis instance for two cross-cutting concerns:
// caching public read responses and enforcing cluster-wide rate limits. Every
// helper degrades gracefully — when Redis is absent or unreachable the cache
// reports a miss and the limiter reports "not limited", so the application keeps
// working (just without the shared optimisation).
package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"flutter-admin-go/internal/config"

	"github.com/redis/go-redis/v9"
)

var (
	once   sync.Once
	client *redis.Client
)

// Client returns the lazily-initialised shared Redis client, or nil when no
// Redis address is configured. The timeouts are deliberately short so a slow or
// down Redis never blocks a request for long before falling back.
func Client() *redis.Client {
	once.Do(func() {
		addr := config.Load().Redis.Addr
		if addr == "" {
			return
		}
		client = redis.NewClient(&redis.Options{
			Addr:         addr,
			DialTimeout:  time.Second,
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
		})
	})
	return client
}

// Ping reports whether Redis is reachable, used by the health check.
func Ping(ctx context.Context) error {
	c := Client()
	if c == nil {
		return redis.ErrClosed
	}
	return c.Ping(ctx).Err()
}

var (
	availMu      sync.Mutex
	availChecked time.Time
	availOK      bool
)

// Available reports whether Redis is currently usable, caching the probe result
// for a few seconds so it can be called on hot paths (e.g. the login limiter)
// without pinging on every request.
func Available() bool {
	c := Client()
	if c == nil {
		return false
	}
	availMu.Lock()
	defer availMu.Unlock()
	if !availChecked.IsZero() && time.Since(availChecked) < 5*time.Second {
		return availOK
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	availOK = c.Ping(ctx).Err() == nil
	availChecked = time.Now()
	return availOK
}

// GetJSON loads and decodes a cached JSON value into dest, returning true only
// on a hit that decodes cleanly. Any miss or error is reported as false.
func GetJSON(ctx context.Context, key string, dest any) bool {
	c := Client()
	if c == nil {
		return false
	}
	raw, err := c.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

// SetJSON stores a JSON-encoded value with a TTL. Errors are ignored: a failed
// cache write must never fail the request that produced the value.
func SetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	c := Client()
	if c == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = c.Set(ctx, key, raw, ttl).Err()
}

// Incr increments the counter at key, applying the TTL on the first increment,
// and returns the new count. ok is false when Redis is unavailable, signalling
// callers to use their local fallback.
func Incr(ctx context.Context, key string, ttl time.Duration) (count int64, ok bool) {
	c := Client()
	if c == nil {
		return 0, false
	}
	n, err := c.Incr(ctx, key).Result()
	if err != nil {
		return 0, false
	}
	if n == 1 {
		_ = c.Expire(ctx, key, ttl).Err()
	}
	return n, true
}

// GetCount reads the current value of a counter key (0 when missing). ok is
// false when Redis is unavailable.
func GetCount(ctx context.Context, key string) (count int64, ok bool) {
	c := Client()
	if c == nil {
		return 0, false
	}
	n, err := c.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, true
	}
	if err != nil {
		return 0, false
	}
	return n, true
}

// TTL reports the remaining lifetime of key. ok is false when Redis is
// unavailable or the key has no expiry.
func TTL(ctx context.Context, key string) (time.Duration, bool) {
	c := Client()
	if c == nil {
		return 0, false
	}
	d, err := c.TTL(ctx, key).Result()
	if err != nil || d < 0 {
		return 0, false
	}
	return d, true
}

// Del removes keys, ignoring errors.
func Del(ctx context.Context, keys ...string) {
	c := Client()
	if c == nil || len(keys) == 0 {
		return
	}
	_ = c.Del(ctx, keys...).Err()
}

// LimiterStore adapts the package helpers to common.SharedStore so rate limiters
// can enforce limits cluster-wide. Each method first checks Availability so a
// down Redis cleanly hands control back to the in-memory fallback.
type LimiterStore struct{}

func (LimiterStore) Incr(key string, ttl time.Duration) (int64, bool) {
	if !Available() {
		return 0, false
	}
	return Incr(context.Background(), key, ttl)
}

func (LimiterStore) Count(key string) (int64, bool) {
	if !Available() {
		return 0, false
	}
	return GetCount(context.Background(), key)
}

func (LimiterStore) TTL(key string) (time.Duration, bool) {
	if !Available() {
		return 0, false
	}
	return TTL(context.Background(), key)
}

func (LimiterStore) Del(key string) {
	Del(context.Background(), key)
}
