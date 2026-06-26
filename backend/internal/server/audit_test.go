package server

import (
	"net/http"
	"testing"
)

func TestAuditableRequest(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/admin/videos", true},
		{http.MethodPut, "/api/admin/users/1", true},
		{http.MethodDelete, "/api/admin/orders/5", true},
		{http.MethodGet, "/api/admin/videos", false},   // reads are not audited
		{http.MethodPost, "/api/mobile/login", false},  // non-admin
		{http.MethodPost, "/api/videos/1/comments", false},
		{http.MethodOptions, "/api/admin/videos", false},
	}
	for _, c := range cases {
		r, _ := http.NewRequest(c.method, c.path, nil)
		if got := auditableRequest(r); got != c.want {
			t.Errorf("auditableRequest(%s %s) = %v, want %v", c.method, c.path, got, c.want)
		}
	}
}
