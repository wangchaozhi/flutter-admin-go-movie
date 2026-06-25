package common

import (
	"net/http"
	"strconv"
)

// Pagination holds the resolved paging window for a list request.
type Pagination struct {
	Page    int
	PerPage int
	Offset  int
}

// HasPagination reports whether the request opted into pagination by sending a
// page or per_page query param. List handlers stay backward compatible: without
// these params they return the full array, with them they return a paged object.
func HasPagination(r *http.Request) bool {
	q := r.URL.Query()
	return q.Has("page") || q.Has("per_page")
}

// ParsePagination reads page/per_page from the query string, clamping per_page
// to [1, maxPerPage] and defaulting to defaultPerPage when absent or invalid.
func ParsePagination(r *http.Request, defaultPerPage, maxPerPage int) Pagination {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return Pagination{Page: page, PerPage: perPage, Offset: (page - 1) * perPage}
}

// PageResponse builds the standard paged payload: {items, total, page, per_page}.
// items should always be a non-nil slice so the JSON is [] rather than null.
func PageResponse(items interface{}, total int64, p Pagination) map[string]interface{} {
	return map[string]interface{}{
		"items":    items,
		"total":    total,
		"page":     p.Page,
		"per_page": p.PerPage,
	}
}
