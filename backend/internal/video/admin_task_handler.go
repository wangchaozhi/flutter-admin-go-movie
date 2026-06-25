package video

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

// transcodeTaskItem is one per-quality entry in the admin video task list. It
// carries the latest transcode task for a quality plus whether that quality is
// currently referenced by the playable master playlist.
type transcodeTaskItem struct {
	store.VideoTranscodeTask
	Transcoded bool `json:"transcoded"`
}

type adminTranscodeHistoryItem struct {
	ID             int64      `json:"id"`
	VideoID        int64      `json:"video_id"`
	VideoTitle     string     `json:"video_title"`
	BatchID        int64      `json:"batch_id"`
	Quality        string     `json:"quality"`
	PreviousStatus string     `json:"previous_status"`
	Status         string     `json:"status"`
	StatusMessage  string     `json:"status_message"`
	Progress       int        `json:"progress"`
	Attempt        int        `json:"attempt"`
	ErrorMessage   string     `json:"error_message"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// AdminTranscodeHistoryHandler serves global transcode task history for admin.
//
//	GET /api/admin/video/transcode-tasks
func AdminTranscodeHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	reconcileActiveHistoryTasks(r.Context())

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	quality := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("quality")))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))

	query := store.DB().
		Table("video_transcode_tasks AS t").
		Select(`t.id, t.video_id, COALESCE(v.title, '') AS video_title, t.batch_id, t.quality,
			t.previous_status, t.status, t.status_message, t.progress, t.attempt, t.error_message,
			t.started_at, t.finished_at, t.created_at`).
		Joins("LEFT JOIN videos AS v ON v.id = t.video_id").
		Order("t.created_at DESC, t.id DESC").
		Limit(300)
	if status != "" && status != "all" {
		query = query.Where("t.status = ?", status)
	}
	if quality != "" && quality != "all" {
		query = query.Where("t.quality = ?", quality)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"CAST(t.video_id AS TEXT) LIKE ? OR COALESCE(v.title, '') ILIKE ? OR t.quality ILIKE ? OR t.status_message ILIKE ? OR t.error_message ILIKE ?",
			like,
			like,
			like,
			like,
			like,
		)
	}

	var result []adminTranscodeHistoryItem
	if err := query.Scan(&result).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: result})
}

// AdminTranscodeHistoryByIDHandler deletes one transcode task history record.
//
//	DELETE /api/admin/video/transcode-tasks/{id}
func AdminTranscodeHistoryByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	idText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/video/transcode-tasks/"), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid task id"})
		return
	}

	var task store.VideoTranscodeTask
	if err := store.DB().First(&task, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "task not found"})
		return
	}
	if isActiveTranscodeStatus(task.Status) {
		common.WriteJSON(w, http.StatusConflict, common.APIResponse{Code: 409, Msg: "进行中的转码任务不能删除"})
		return
	}
	if err := store.DB().Delete(&store.VideoTranscodeTask{}, id).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func reconcileActiveHistoryTasks(ctx context.Context) {
	var videoIDs []int64
	if err := store.DB().Model(&store.VideoTranscodeTask{}).
		Where("status IN ?", activeTranscodeStatuses()).
		Distinct("video_id").
		Limit(100).
		Pluck("video_id", &videoIDs).Error; err != nil {
		return
	}
	for _, videoID := range videoIDs {
		reconcileStaleTranscodeTasks(ctx, videoID)
	}
}

// AdminVideoTasksHandler serves the per-quality transcode task list and the
// per-quality variant deletion for a video.
//
//	GET    /api/admin/videos/{id}/tasks                   -> list latest task per quality
//	DELETE /api/admin/videos/{id}/tasks/{quality}         -> remove that quality variant
//	DELETE /api/admin/videos/{id}/tasks/{quality}/cancel  -> cancel active transcode for that quality
func AdminVideoTasksHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/videos/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[1] != "tasks" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid path"})
		return
	}
	videoID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}

	switch {
	case r.Method == http.MethodGet && len(parts) == 2:
		listVideoTranscodeTasks(w, r, videoID)
	case r.Method == http.MethodDelete && len(parts) == 3:
		deleteVideoTranscodeQuality(w, r, videoID, parts[2])
	case r.Method == http.MethodDelete && len(parts) == 4 && parts[3] == "cancel":
		cancelVideoTranscodeQuality(w, r, videoID, parts[2])
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func listVideoTranscodeTasks(w http.ResponseWriter, r *http.Request, videoID int64) {
	var v store.Video
	if err := store.DB().First(&v, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}
	reconcileStaleTranscodeTasks(r.Context(), videoID)

	var tasks []store.VideoTranscodeTask
	if err := store.DB().Where("video_id = ?", videoID).Order("id desc").Find(&tasks).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	transcoded := make(map[string]bool)
	for _, name := range transcodedQualityNames(r.Context(), v) {
		transcoded[name] = true
	}
	latest := displayTranscodeTasksByQuality(tasks, transcoded)

	items := make([]transcodeTaskItem, 0, len(latest))
	for _, task := range latest {
		items = append(items, transcodeTaskItem{VideoTranscodeTask: task, Transcoded: transcoded[task.Quality]})
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: items})
}

func displayTranscodeTasksByQuality(tasks []store.VideoTranscodeTask, transcoded map[string]bool) []store.VideoTranscodeTask {
	latestByQuality := make(map[string]store.VideoTranscodeTask, len(tasks))
	latestSuccessByQuality := make(map[string]store.VideoTranscodeTask, len(tasks))
	for _, task := range tasks {
		if task.Quality == "" {
			continue
		}
		if _, ok := latestSuccessByQuality[task.Quality]; !ok && task.Status == "success" {
			latestSuccessByQuality[task.Quality] = task
		}
		if _, seen := latestByQuality[task.Quality]; seen {
			continue
		}
		latestByQuality[task.Quality] = task
	}

	latest := make([]store.VideoTranscodeTask, 0, len(latestByQuality))
	for quality, task := range latestByQuality {
		if task.Status == "canceled" && transcoded[quality] {
			if previousSuccess, ok := latestSuccessByQuality[quality]; ok {
				task = previousSuccess
			} else {
				task.Status = "success"
				task.StatusMessage = "完成"
				task.Progress = 100
				task.ErrorMessage = ""
			}
		}
		latest = append(latest, task)
	}
	sort.SliceStable(latest, func(i, j int) bool {
		left := qualityHeight(latest[i].Quality)
		right := qualityHeight(latest[j].Quality)
		if left == right {
			return latest[i].Quality < latest[j].Quality
		}
		return left < right
	})
	return latest
}

func deleteVideoTranscodeQuality(w http.ResponseWriter, r *http.Request, videoID int64, quality string) {
	quality = strings.ToLower(strings.TrimSpace(quality))
	if qualityHeight(quality) <= 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid quality"})
		return
	}
	if err := store.DB().First(&store.Video{}, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}
	reconcileStaleTranscodeTasks(r.Context(), videoID)

	var active int64
	store.DB().Model(&store.VideoTranscodeTask{}).
		Where("video_id = ? AND quality = ? AND status IN ?", videoID, quality, activeTranscodeStatuses()).
		Count(&active)
	if active > 0 {
		common.WriteJSON(w, http.StatusConflict, common.APIResponse{Code: 409, Msg: "该清晰度正在转码中，无法删除"})
		return
	}

	if err := removeTranscodeQualityOutput(r.Context(), videoID, quality); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	store.DB().Where("video_id = ? AND quality = ?", videoID, quality).Delete(&store.VideoTranscodeTask{})
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func cancelVideoTranscodeQuality(w http.ResponseWriter, r *http.Request, videoID int64, quality string) {
	quality = strings.ToLower(strings.TrimSpace(quality))
	if qualityHeight(quality) <= 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid quality"})
		return
	}
	if err := store.DB().First(&store.Video{}, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}
	count, err := cancelTranscodeTasks(r.Context(), videoID, quality)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{"canceled": count}})
}

// removeTranscodeQualityOutput drops one quality from the master playlist and
// deletes its HLS objects. When the last quality is removed the whole HLS tree
// is cleared and the video falls back to the "uploaded" state so it can be
// transcoded again.
func removeTranscodeQualityOutput(ctx context.Context, videoID int64, quality string) error {
	masterKey := fmt.Sprintf("hls/%d/master.m3u8", videoID)
	raw, err := readMinioText(ctx, masterKey)
	if err != nil {
		// no playable output for this video; nothing to remove from storage
		return nil
	}

	remaining := make(map[string]masterPlaylistEntry)
	var removed *masterPlaylistEntry
	for _, entry := range parseMasterPlaylistEntries(raw) {
		if entry.name == quality {
			e := entry
			removed = &e
			continue
		}
		remaining[entry.name] = entry
	}
	if removed == nil {
		// quality is not part of the master playlist; nothing to remove
		return nil
	}

	// remove the deleted quality's segments/playlist objects
	if dir := strings.Trim(path.Dir(masterURIPath(removed.uri)), "/"); dir != "" && dir != "." {
		removeObjectsByPrefix(ctx, fmt.Sprintf("hls/%d/%s/", videoID, dir))
	}

	if len(remaining) > 0 {
		master := renderMasterPlaylist(remaining)
		return putText(ctx, masterKey, master, "application/vnd.apple.mpegurl")
	}

	// removed the last quality: clear the HLS tree and reset the video status
	removeObjectsByPrefix(ctx, fmt.Sprintf("hls/%d/", videoID))
	var v store.Video
	if err := store.DB().First(&v, videoID).Error; err == nil {
		status := "uploaded"
		if strings.TrimSpace(v.OriginalKey) == "" {
			status = "uploading"
		}
		store.DB().Model(&store.Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
			"status":         status,
			"hls_master_key": "",
			"updated_at":     time.Now(),
		})
	}
	return nil
}
