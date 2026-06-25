package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"https://admin.example.com", "http://localhost:5173"}

	if !originAllowed("https://admin.example.com", allowed) {
		t.Fatal("expected exact origin to be allowed")
	}
	if originAllowed("https://evil.example.com", allowed) {
		t.Fatal("unlisted origin must not be allowed")
	}
	if originAllowed("", allowed) {
		t.Fatal("empty origin must not be allowed")
	}
}

// requirePerm must reject unauthenticated writes before reaching the handler,
// regardless of the permission string, so the UI is never the only gate.
func TestRequirePermBlocksUnauthenticated(t *testing.T) {
	called := false
	guarded := requirePerm(map[string]string{http.MethodPost: "video:create"},
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/videos", nil)
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler must not run for an unauthenticated request")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// A method not listed in the permission map still requires a valid admin
// session: an anonymous GET is rejected, not silently passed through.
func TestRequirePermUnlistedMethodStillRequiresAuth(t *testing.T) {
	called := false
	guarded := requirePerm(map[string]string{http.MethodPost: "video:create"},
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/videos", nil)
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler must not run for an unauthenticated GET")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous GET, got %d", rec.Code)
	}
}
