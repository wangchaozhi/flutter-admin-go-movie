package video

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A non-numeric bullet id is rejected with 400 before auth or any DB access.
func TestAppDanmakuByIDRejectsInvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/mobile/danmaku/not-a-number", nil)
	rec := httptest.NewRecorder()
	AppDanmakuByIDHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric id, got %d", rec.Code)
	}
}

// Without a valid mobile token the bullet actions are rejected with 401 before
// touching the database.
func TestAppDanmakuByIDRequiresAuth(t *testing.T) {
	for _, target := range []string{"/api/mobile/danmaku/5", "/api/mobile/danmaku/5/like"} {
		req := httptest.NewRequest(http.MethodPost, target, nil)
		rec := httptest.NewRecorder()
		AppDanmakuByIDHandler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s without token, got %d", target, rec.Code)
		}
	}
}
