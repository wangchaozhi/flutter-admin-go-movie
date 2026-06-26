package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

// inviteCodeErrorMessage must map every known store invite error to its own
// user-facing message and fall back to a generic one for anything else.
func TestInviteCodeErrorMessage(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{store.ErrInviteRequired, "请输入邀请码"},
		{store.ErrInviteDisabled, "邀请码已停用"},
		{store.ErrInviteExpired, "邀请码已过期"},
		{store.ErrInviteExhausted, "邀请码使用次数已达上限"},
		{errors.New("some other failure"), "邀请码无效"},
	}
	for _, c := range cases {
		if got := inviteCodeErrorMessage(c.err); got != c.want {
			t.Errorf("inviteCodeErrorMessage(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// currentMobileUserID extracts the user id from a valid signed token, and
// rejects requests with no token or a forged one.
func TestCurrentMobileUserID(t *testing.T) {
	token, err := admin.BuildMobileToken(42, "alice")
	if err != nil {
		t.Fatalf("BuildMobileToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/mobile/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if id, ok := currentMobileUserID(req); !ok || id != 42 {
		t.Fatalf("expected (42, true), got (%d, %v)", id, ok)
	}

	for _, raw := range []string{"", "Bearer not-a-jwt", "Bearer mobile-token:42"} {
		req := httptest.NewRequest(http.MethodGet, "/api/mobile/profile", nil)
		if raw != "" {
			req.Header.Set("Authorization", raw)
		}
		if _, ok := currentMobileUserID(req); ok {
			t.Fatalf("forged/empty authorization %q was accepted", raw)
		}
	}
}

// tooManyLoginAttempts gates on the limiter: once the key is blocked it writes a
// 429 with a positive Retry-After and reports true; otherwise it stays silent.
func TestTooManyLoginAttempts(t *testing.T) {
	limiter := common.NewLimiter("test_login_guard", 1, time.Minute)
	const key = "1.2.3.4"

	rec := httptest.NewRecorder()
	if tooManyLoginAttempts(rec, limiter, key) {
		t.Fatal("a fresh key must not be blocked")
	}

	limiter.Fail(key) // reaches max (1) -> blocked
	rec = httptest.NewRecorder()
	if !tooManyLoginAttempts(rec, limiter, key) {
		t.Fatal("expected the key to be blocked after reaching max")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Fatalf("expected a positive Retry-After, got %q", ra)
	}
}

// Login handlers reject non-POST verbs before touching credentials or the DB.
func TestLoginHandlersRejectWrongMethod(t *testing.T) {
	for _, h := range []http.HandlerFunc{AdminLoginHandler, MobileLoginHandler, MobileRegisterHandler} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/api/login", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for GET, got %d", rec.Code)
		}
	}
}

// A malformed JSON body is rejected with 400 before any credential lookup, so
// the handler never reaches the database.
func TestAdminLoginRejectsBadBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader("{not-json"))
	req.RemoteAddr = "203.0.113.7:1234"
	rec := httptest.NewRecorder()
	AdminLoginHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d", rec.Code)
	}
}

// Registration input validation runs before the database is touched, so these
// rejections are exercised without a DB. Each case uses a distinct client IP so
// the per-IP register limiter never trips across cases.
func TestMobileRegisterValidation(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		body string
		want string
	}{
		{"short username", "198.51.100.1", `{"username":"ab","password":"secret","invite_code":"X"}`, "用户名长度需为 3-32 个字符"},
		{"short password", "198.51.100.2", `{"username":"alice","password":"123","invite_code":"X"}`, "密码至少 6 位"},
		{"bad email", "198.51.100.3", `{"username":"alice","password":"secret","email":"nope","invite_code":"X"}`, "邮箱格式不正确"},
		{"missing invite", "198.51.100.4", `{"username":"alice","password":"secret"}`, "请输入邀请码"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/mobile/register", strings.NewReader(c.body))
			req.Header.Set("X-Forwarded-For", c.ip)
			rec := httptest.NewRecorder()
			MobileRegisterHandler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Fatalf("expected message %q, got %s", c.want, rec.Body.String())
			}
		})
	}
}

// The register limiter blocks a single IP after maxing out its hourly quota,
// returning 429 with Retry-After rather than processing the body.
func TestMobileRegisterRateLimited(t *testing.T) {
	const ip = "192.0.2.55"
	// registerLimiter allows 5 per hour; the 6th request from one IP is blocked.
	// Use a body that would otherwise fail validation so no DB call is reached
	// even on the allowed calls.
	body := `{"username":"ab"}`
	var lastCode int
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/mobile/register", strings.NewReader(body))
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		MobileRegisterHandler(rec, req)
		lastCode = rec.Code
		if i == 5 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429 on the 6th attempt, got %d", rec.Code)
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Fatal("expected Retry-After header on rate-limited response")
			}
		}
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected final attempt to be rate limited, got %d", lastCode)
	}
}
