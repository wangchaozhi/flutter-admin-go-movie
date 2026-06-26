package video

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeSeriesStatus(t *testing.T) {
	cases := map[string]string{
		"completed":   "completed",
		"offline":     "offline",
		"ongoing":     "ongoing",
		"":            "ongoing",   // empty defaults to ongoing
		"garbage":     "ongoing",   // unknown defaults to ongoing
		" completed ": "completed", // surrounding whitespace is trimmed
	}
	for in, want := range cases {
		if got := normalizeSeriesStatus(in); got != want {
			t.Errorf("normalizeSeriesStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

// A non-numeric series id is rejected with 400 before any database access.
func TestAdminSeriesByIDRejectsInvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/series/not-a-number", nil)
	rec := httptest.NewRecorder()
	AdminSeriesByIDHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric id, got %d", rec.Code)
	}
}

// The public series list rejects non-GET verbs before touching the database.
func TestAppListSeriesRejectsWrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/series", nil)
	rec := httptest.NewRecorder()
	AppListSeriesHandler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}
