package server

import "testing"

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
