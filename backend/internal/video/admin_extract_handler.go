package video

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

type adminExtractHistoryItem struct {
	ID            int64      `json:"id"`
	VideoID       int64      `json:"video_id"`
	VideoTitle    string     `json:"video_title"`
	SourceKey     string     `json:"source_key"`
	Status        string     `json:"status"`
	StatusMessage string     `json:"status_message"`
	AudioCount    int        `json:"audio_count"`
	SubtitleCount int        `json:"subtitle_count"`
	ReadyCount    int        `json:"ready_count"`
	FailedCount   int        `json:"failed_count"`
	ErrorMessage  string     `json:"error_message"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AdminExtractHistoryHandler serves global audio/subtitle extraction task
// history for admin.
//
//	GET /api/admin/video/extract-tasks
func AdminExtractHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	reconcileStaleExtractTasks()

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))

	dataQuery := store.DB().
		Table("video_extract_track_tasks AS t").
		Select(`t.id, t.video_id, COALESCE(v.title, '') AS video_title, t.source_key,
			t.status, t.status_message, t.audio_count, t.subtitle_count, t.ready_count,
			t.failed_count, t.error_message, t.started_at, t.finished_at, t.created_at`).
		Joins("LEFT JOIN videos AS v ON v.id = t.video_id").
		Order("t.created_at DESC, t.id DESC")
	countQuery := store.DB().
		Table("video_extract_track_tasks AS t").
		Joins("LEFT JOIN videos AS v ON v.id = t.video_id")
	if status == "active" {
		dataQuery = dataQuery.Where("t.status = ?", "processing")
		countQuery = countQuery.Where("t.status = ?", "processing")
	} else if status != "" && status != "all" {
		dataQuery = dataQuery.Where("t.status = ?", status)
		countQuery = countQuery.Where("t.status = ?", status)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		const cond = "CAST(t.video_id AS TEXT) LIKE ? OR COALESCE(v.title, '') ILIKE ? OR t.source_key ILIKE ? OR t.status_message ILIKE ? OR t.error_message ILIKE ?"
		dataQuery = dataQuery.Where(cond, like, like, like, like, like)
		countQuery = countQuery.Where(cond, like, like, like, like, like)
	}

	if !common.HasPagination(r) {
		result := make([]adminExtractHistoryItem, 0)
		if err := dataQuery.Limit(300).Scan(&result).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: result})
		return
	}
	p := common.ParsePagination(r, 20, 100)
	var total int64
	countQuery.Count(&total)
	result := make([]adminExtractHistoryItem, 0)
	if err := dataQuery.Offset(p.Offset).Limit(p.PerPage).Scan(&result).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: common.PageResponse(result, total, p)})
}

// AdminExtractHistoryByIDHandler deletes one extraction task history record.
//
//	DELETE /api/admin/video/extract-tasks/{id}
func AdminExtractHistoryByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	idText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/video/extract-tasks/"), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid task id"})
		return
	}

	var task store.VideoExtractTrackTask
	if err := store.DB().First(&task, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "task not found"})
		return
	}
	if isActiveExtractStatus(task.Status) {
		common.WriteJSON(w, http.StatusConflict, common.APIResponse{Code: 409, Msg: "进行中的提取任务不能删除"})
		return
	}
	if err := store.DB().Delete(&store.VideoExtractTrackTask{}, id).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// reconcileStaleExtractTasks fails out extraction history rows that have been
// stuck in "processing" longer than staleExtractingAge, which only happens if
// the worker died between starting and finishing. It mirrors the janitor's
// reapStaleExtractingVideos so the UI never shows a perpetually "进行中" run.
func reconcileStaleExtractTasks() {
	cutoff := time.Now().Add(-staleExtractingAge)
	now := time.Now()
	store.DB().Model(&store.VideoExtractTrackTask{}).
		Where("status = ? AND created_at < ?", extractStatusProcessing, cutoff).
		Updates(map[string]interface{}{
			"status":         extractStatusFailed,
			"status_message": "提取超时，任务可能已中断",
			"error_message":  "extraction timed out",
			"finished_at":    now,
		})
}
