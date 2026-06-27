package video

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The notification feed only serves GET; other verbs are rejected before any
// database access.
func TestAppNotificationsRejectsWrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/mobile/notifications", nil)
	rec := httptest.NewRecorder()
	AppNotificationsHandler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

// Listing notifications without a valid mobile token is rejected with 401 before
// touching the database.
func TestAppNotificationsRequiresAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/mobile/notifications", nil)
	rec := httptest.NewRecorder()
	AppNotificationsHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

// The notification sub-actions (unread-count / read / {id}/read) all require a
// valid mobile token, enforced before any database access.
func TestAppNotificationByPathRequiresAuth(t *testing.T) {
	cases := []struct {
		method string
		target string
	}{
		{http.MethodGet, "/api/mobile/notifications/unread-count"},
		{http.MethodPost, "/api/mobile/notifications/read"},
		{http.MethodPost, "/api/mobile/notifications/9/read"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.target, nil)
		rec := httptest.NewRecorder()
		AppNotificationByPathHandler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s without token, got %d", c.method, c.target, rec.Code)
		}
	}
}
