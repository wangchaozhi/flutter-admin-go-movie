package video

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseCommentVideoID(t *testing.T) {
	id, err := parseCommentVideoID("/api/videos/42/comments")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected 42, got %d", id)
	}
	if _, err := parseCommentVideoID("/api/videos/not-a-number/comments"); err == nil {
		t.Fatalf("expected error for non-numeric id")
	}
}

// displayName prefers a (trimmed) nickname, falls back to the username, and
// finally to a generic label.
func TestDisplayName(t *testing.T) {
	cases := []struct {
		in   commentUser
		want string
	}{
		{commentUser{Nickname: "Neo", Username: "neo"}, "Neo"},
		{commentUser{Nickname: "  ", Username: "neo"}, "neo"},
		{commentUser{Nickname: " Neo "}, "Neo"},
		{commentUser{}, "用户"},
	}
	for _, c := range cases {
		if got := displayName(c.in); got != c.want {
			t.Errorf("displayName(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A non-numeric video id is rejected with 400 before any database access.
func TestAppVideoCommentsRejectsInvalidVideoID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/videos/not-a-number/comments", nil)
	rec := httptest.NewRecorder()
	AppVideoCommentsHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric video id, got %d", rec.Code)
	}
}

// Unsupported verbs on the comments collection are rejected before any DB hit.
func TestAppVideoCommentsRejectsWrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/videos/5/comments", nil)
	rec := httptest.NewRecorder()
	AppVideoCommentsHandler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for DELETE, got %d", rec.Code)
	}
}

// Posting a comment without a valid mobile token is rejected with 401 before
// touching the database.
func TestCreateVideoCommentRequiresAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/videos/5/comments", nil)
	rec := httptest.NewRecorder()
	AppVideoCommentsHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

// A non-numeric comment id is rejected with 400 before auth or any DB access.
func TestAppCommentByIDRejectsInvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/mobile/comments/not-a-number", nil)
	rec := httptest.NewRecorder()
	AppCommentByIDHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric id, got %d", rec.Code)
	}
}

// Without a valid mobile token the comment actions (edit/delete/like) are
// rejected with 401 before touching the database.
func TestAppCommentByIDRequiresAuth(t *testing.T) {
	cases := []struct {
		method string
		target string
	}{
		{http.MethodPut, "/api/mobile/comments/5"},
		{http.MethodDelete, "/api/mobile/comments/5"},
		{http.MethodPost, "/api/mobile/comments/5/like"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.target, nil)
		rec := httptest.NewRecorder()
		AppCommentByIDHandler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s without token, got %d", c.method, c.target, rec.Code)
		}
	}
}
