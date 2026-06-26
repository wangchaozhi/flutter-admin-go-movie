package common

import (
	"net/http"
	"testing"
	"time"
)

func TestLoginLimiterBlocksAfterMax(t *testing.T) {
	l := NewLimiter("test-block", 3, time.Minute)
	key := "1.2.3.4"

	for i := 0; i < 3; i++ {
		if blocked, _ := l.Blocked(key); blocked {
			t.Fatalf("should not be blocked before reaching max (attempt %d)", i)
		}
		l.Fail(key)
	}
	blocked, retry := l.Blocked(key)
	if !blocked {
		t.Fatal("expected key to be blocked after 3 failures")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("unexpected retry-after: %v", retry)
	}
}

func TestLoginLimiterResetClearsFailures(t *testing.T) {
	l := NewLimiter("test-reset", 2, time.Minute)
	key := "5.6.7.8"
	l.Fail(key)
	l.Fail(key)
	if blocked, _ := l.Blocked(key); !blocked {
		t.Fatal("expected blocked before reset")
	}
	l.Reset(key)
	if blocked, _ := l.Blocked(key); blocked {
		t.Fatal("expected not blocked after reset")
	}
}

func TestLoginLimiterWindowExpiry(t *testing.T) {
	l := NewLimiter("test-expiry", 1, 10*time.Millisecond)
	key := "9.9.9.9"
	l.Fail(key)
	if blocked, _ := l.Blocked(key); !blocked {
		t.Fatal("expected blocked immediately after failure")
	}
	time.Sleep(20 * time.Millisecond)
	if blocked, _ := l.Blocked(key); blocked {
		t.Fatal("expected window to expire and unblock")
	}
}

func TestLimiterAllowThrottlesEveryCall(t *testing.T) {
	l := NewLimiter("test-allow", 2, time.Minute)
	key := "u-1"

	if ok, _ := l.Allow(key); !ok {
		t.Fatal("first call should be allowed")
	}
	if ok, _ := l.Allow(key); !ok {
		t.Fatal("second call (at max) should be allowed")
	}
	ok, retry := l.Allow(key)
	if ok {
		t.Fatal("third call should be throttled")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("unexpected retry-after: %v", retry)
	}

	// A different key has its own window.
	if ok, _ := l.Allow("u-2"); !ok {
		t.Fatal("a distinct key should not be affected")
	}
}

func TestLimiterAllowWindowResets(t *testing.T) {
	l := NewLimiter("test-allow-reset", 1, 10*time.Millisecond)
	key := "u-3"
	if ok, _ := l.Allow(key); !ok {
		t.Fatal("first call should be allowed")
	}
	if ok, _ := l.Allow(key); ok {
		t.Fatal("second call should be throttled within window")
	}
	time.Sleep(20 * time.Millisecond)
	if ok, _ := l.Allow(key); !ok {
		t.Fatal("call after window reset should be allowed again")
	}
}

func TestClientIPPrefersForwardedFor(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:54321"
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := ClientIP(r); got != "203.0.113.7" {
		t.Fatalf("expected forwarded client IP, got %q", got)
	}

	r2, _ := http.NewRequest(http.MethodGet, "/", nil)
	r2.RemoteAddr = "10.0.0.2:1111"
	if got := ClientIP(r2); got != "10.0.0.2" {
		t.Fatalf("expected remote host, got %q", got)
	}
}
