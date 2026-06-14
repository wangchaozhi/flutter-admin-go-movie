package admin

import (
	"net/http"
	"testing"
)

func TestAdminTokenRoundTrip(t *testing.T) {
	token, err := BuildAdminToken("admin")
	if err != nil {
		t.Fatalf("BuildAdminToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	username, ok := CurrentAdminUsername(req)
	if !ok || username != "admin" {
		t.Fatalf("expected admin, got (%q, %v)", username, ok)
	}
}

func TestCurrentAdminUsernameRejectsForgedToken(t *testing.T) {
	// The old scheme accepted "admin-token:<username>" verbatim. Ensure that
	// (and any other unsigned junk) is now rejected.
	for _, raw := range []string{
		"admin-token:admin",
		"Bearer admin-token:admin",
		"Bearer not-a-jwt",
		"",
	} {
		req, _ := http.NewRequest(http.MethodGet, "/api/admin/users", nil)
		if raw != "" {
			req.Header.Set("Authorization", raw)
		}
		if _, ok := CurrentAdminUsername(req); ok {
			t.Fatalf("forged authorization %q was accepted", raw)
		}
	}
}
