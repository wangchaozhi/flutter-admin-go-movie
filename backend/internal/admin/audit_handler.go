package admin

import (
	"net/http"
	"strings"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

// AuditLogsHandler returns the admin mutation audit trail, newest first, with an
// optional ?q= keyword over username/path. Read-only; open to any authenticated
// admin like the other GET endpoints.
//
// GET /api/admin/audit-logs
func AuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}

	base := store.DB().Model(&store.AuditLog{})
	if keyword := strings.TrimSpace(r.URL.Query().Get("q")); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("username ILIKE ? OR path ILIKE ?", like, like)
	}

	p := common.ParsePagination(r, 20, 100)
	var total int64
	base.Count(&total)

	var logs []store.AuditLog
	base.Order("id desc").Offset(p.Offset).Limit(p.PerPage).Find(&logs)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: common.PageResponse(logs, total, p)})
}
