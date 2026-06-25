package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"

	"github.com/minio/minio-go/v7"
)

type transcodeRequest struct {
	Qualities []string `json:"qualities"`
}

type transcodeStatusResponse struct {
	store.VideoTranscodeTask
	QualityStatuses map[string]string `json:"quality_statuses,omitempty"`
	QualityMessages map[string]string `json:"quality_messages,omitempty"`
	QualityProgress map[string]int    `json:"quality_progress,omitempty"`
}

// POST /api/admin/videos
func AdminCreateVideoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		CategoryID  int      `json:"category_id"`
		Actors      []string `json:"actors"`
		Directors   []string `json:"directors"`
		Genres      []string `json:"genres"`
		Region      string   `json:"region"`
		ReleaseYear int      `json:"release_year"`
		Language    string   `json:"language"`
		IsVip       bool     `json:"is_vip"`
		IsFree      bool     `json:"is_free"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "title required"})
		return
	}
	v := &store.Video{
		Title:       req.Title,
		Description: req.Description,
		CategoryID:  req.CategoryID,
		Actors:      store.StringArray(normalizeCatalogNames(req.Actors, 16, 40)),
		Directors:   store.StringArray(normalizeCatalogNames(req.Directors, 8, 40)),
		Genres:      store.StringArray(normalizeCatalogNames(req.Genres, 12, 24)),
		Region:      trimCatalogText(req.Region, 128),
		ReleaseYear: normalizeReleaseYear(req.ReleaseYear),
		Language:    trimCatalogText(req.Language, 128),
		IsVip:       req.IsVip,
		IsFree:      req.IsFree,
		Status:      "uploading",
	}
	if err := store.DB().Create(v).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: v})
}

// POST /api/admin/videos/{id}/upload
func AdminUploadVideoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	videoID, err := parseVideoID(r.URL.Path, "/api/admin/videos/", "/upload")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}

	var v store.Video
	if err := store.DB().First(&v, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}

	if err := r.ParseMultipartForm(500 << 20); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "parse form failed"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "file required"})
		return
	}
	defer file.Close()

	key, contentType, err := uploadedVideoSourceInfo(videoID, header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	uploadInfo, err := store.ObjectClient().PutObject(
		r.Context(),
		store.VideoBucket(),
		key,
		file,
		header.Size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "upload failed: " + err.Error()})
		return
	}
	if !videoRecordExists(videoID) {
		_ = store.ObjectClient().RemoveObject(r.Context(), store.VideoBucket(), key, minio.RemoveObjectOptions{})
		common.WriteJSON(w, http.StatusGone, common.APIResponse{Code: 410, Msg: "video was deleted during upload"})
		return
	}
	removeObjectsByPrefix(r.Context(), fmt.Sprintf("hls/%d/", videoID))
	store.DB().Where("video_id = ?", videoID).Delete(&store.VideoMediaTrack{})
	if strings.TrimSpace(v.OriginalKey) != "" && v.OriginalKey != key {
		_ = store.ObjectClient().RemoveObject(r.Context(), store.VideoBucket(), v.OriginalKey, minio.RemoveObjectOptions{})
	}

	sourceSize := sourceVideoSize{}
	duration := 0
	srcPath := ""
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		log.Printf("seek uploaded video failed for video %d: %v", videoID, err)
	} else {
		cachedPath, err := cacheUploadedSource(r.Context(), videoID, key, header.Size, uploadInfo.ETag, file)
		if err != nil {
			log.Printf("cache uploaded video failed for video %d: %v", videoID, err)
		} else {
			srcPath = cachedPath
			sourceSize = probeSourceSize(srcPath)
			duration = probeDuration(srcPath)
		}
	}
	now := time.Now()
	result := store.DB().Model(&store.Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
		"original_key":         key,
		"hls_master_key":       "",
		"duration":             duration,
		"size":                 header.Size,
		"source_width":         sourceSize.width,
		"source_height":        sourceSize.height,
		"audio_track_count":    0,
		"subtitle_track_count": 0,
		"media_tracks_scanned": false,
		"status":               videoStatusExtracting,
		"updated_at":           now,
	})
	if result.Error != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		_ = store.ObjectClient().RemoveObject(r.Context(), store.VideoBucket(), key, minio.RemoveObjectOptions{})
		common.WriteJSON(w, http.StatusGone, common.APIResponse{Code: 410, Msg: "video was deleted during upload"})
		return
	}

	// Extract audio/subtitle tracks in the background so the upload request
	// returns immediately; the video shows as "extracting" until the worker
	// finishes and releases it to "uploaded".
	if err := EnqueueExtractTracks(r.Context(), videoID, key); err != nil {
		log.Printf("enqueue track extraction failed for video %d, running inline: %v", videoID, err)
		if srcPath != "" {
			if _, err := ensureVideoMediaTracks(r.Context(), videoID, key, srcPath); err != nil {
				log.Printf("prepare media tracks failed for video %d: %v", videoID, err)
			}
		}
		releaseExtractingVideo(videoID, key)
	}

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"video_id":      videoID,
		"original_key":  key,
		"size":          header.Size,
		"source_width":  sourceSize.width,
		"source_height": sourceSize.height,
		"duration":      duration,
	}})
}

// POST /api/admin/videos/{id}/cover
func AdminUploadCoverHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	videoID, err := parseVideoID(r.URL.Path, "/api/admin/videos/", "/cover")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}

	var v store.Video
	if err := store.DB().First(&v, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "parse form failed"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "file required"})
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	ext := "jpg"
	if strings.Contains(contentType, "webp") {
		ext = "webp"
	} else if strings.Contains(contentType, "png") {
		ext = "png"
	}
	key := fmt.Sprintf("covers/%d/cover.%s", videoID, ext)
	_, err = store.ObjectClient().PutObject(
		context.Background(),
		store.VideoBucket(),
		key,
		file,
		header.Size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "upload failed: " + err.Error()})
		return
	}
	store.DB().Model(&v).Updates(map[string]interface{}{"cover_key": key, "updated_at": time.Now()})
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]string{"cover_key": key}})
}

// POST /api/admin/videos/{id}/transcode
func AdminTranscodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	videoID, err := parseVideoID(r.URL.Path, "/api/admin/videos/", "/transcode")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}

	var v store.Video
	if err := store.DB().First(&v, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}
	if v.OriginalKey == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "video not uploaded yet"})
		return
	}
	reconcileStaleTranscodeTasks(r.Context(), videoID)
	var req transcodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	qualities, err := normalizeTranscodeQualityNames(req.Qualities)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	sourceSize := ensureSourceVideoMetadata(r.Context(), &v)
	previousStatus := v.Status
	mergeExisting := v.Status == "ready" || v.Status == "offline"

	requestedQualities := qualities
	if len(requestedQualities) == 0 {
		requestedQualities = transcodeQualityNames(selectTranscodeQualities(sourceSize))
	}
	selectedQualities, err := selectRequestedTranscodeQualities(sourceSize, requestedQualities)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	requestedQualities = transcodeQualityNames(selectedQualities)

	activeTasks, err := activeTranscodeTasks(videoID)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if len(activeTasks) > 0 {
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "transcode already running", Data: activeTranscodeResponse(activeTasks)})
		return
	}

	batchID := time.Now().UnixNano()
	tasks := make([]store.VideoTranscodeTask, 0, len(requestedQualities))
	for _, quality := range requestedQualities {
		tasks = append(tasks, store.VideoTranscodeTask{
			VideoID:        videoID,
			BatchID:        batchID,
			Quality:        quality,
			PreviousStatus: previousStatus,
			Status:         "queued",
			StatusMessage:  "等待入队",
		})
	}
	if err := store.DB().Create(&tasks).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	// Only take the video offline (status=transcoding) when there is no playable
	// output yet. For a re-transcode / "transcode remaining" on an already
	// ready/offline video (mergeExisting), the existing master playlist stays
	// valid until the worker atomically rewrites it, so keep the current status
	// so the app keeps showing/playing the video while the new qualities encode.
	if !mergeExisting {
		store.DB().Model(&v).Updates(map[string]interface{}{"status": "transcoding", "updated_at": time.Now()})
	}

	taskIDs := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
		if err := EnqueueTranscode(r.Context(), videoID, task.ID, task.BatchID, task.Quality, mergeExisting, previousStatus); err != nil {
			failQueuedTranscodeBatch(videoID, batchID, previousStatus, err)
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "enqueue failed: " + err.Error()})
			return
		}
	}
	if err := store.DB().Model(&store.VideoTranscodeTask{}).
		Where("video_id = ? AND batch_id = ? AND status = ?", videoID, batchID, "queued").
		Updates(map[string]interface{}{"status": "pending", "status_message": "等待转码", "progress": 0}).Error; err != nil {
		failQueuedTranscodeBatch(videoID, batchID, previousStatus, err)
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "mark pending failed: " + err.Error()})
		return
	}

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"batch_id": batchID,
		"task_ids": taskIDs,
		"status":   "pending",
	}})
}

// DELETE /api/admin/videos/{id}/transcode
func AdminCancelTranscodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	videoID, err := parseVideoID(r.URL.Path, "/api/admin/videos/", "/transcode")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	if !videoRecordExists(videoID) {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}
	count, err := cancelTranscodeTasks(r.Context(), videoID, "")
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{"canceled": count}})
}

func activeTranscodeTasks(videoID int64) ([]store.VideoTranscodeTask, error) {
	var tasks []store.VideoTranscodeTask
	err := store.DB().
		Where("video_id = ? AND status IN ?", videoID, activeTranscodeStatuses()).
		Order("id asc").
		Find(&tasks).Error
	return tasks, err
}

func activeTranscodeResponse(tasks []store.VideoTranscodeTask) map[string]interface{} {
	taskIDs := make([]int64, 0, len(tasks))
	qualities := make([]string, 0, len(tasks))
	status := "pending"
	batchID := int64(0)
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
		if task.Quality != "" {
			qualities = append(qualities, task.Quality)
		}
		if task.Status == "processing" {
			status = "processing"
		}
		if batchID == 0 {
			batchID = task.BatchID
		}
	}
	return map[string]interface{}{
		"batch_id":  batchID,
		"task_ids":  taskIDs,
		"qualities": qualities,
		"status":    status,
	}
}

func failQueuedTranscodeBatch(videoID, batchID int64, previousStatus string, enqueueErr error) {
	now := time.Now()
	message := "入队失败: " + enqueueErr.Error()
	store.DB().Model(&store.VideoTranscodeTask{}).
		Where("video_id = ? AND batch_id = ? AND status IN ?", videoID, batchID, []string{"queued", "pending"}).
		Updates(map[string]interface{}{
			"status":         "failed",
			"status_message": "入队失败",
			"error_message":  message,
			"finished_at":    now,
		})

	status := previousStatus
	if status == "" || status == "transcoding" {
		status = "failed"
	}
	store.DB().Model(&store.Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	})
}

func activeTranscodeStatuses() []string {
	return []string{"queued", "pending", "processing"}
}

func isActiveTranscodeStatus(status string) bool {
	switch status {
	case "queued", "pending", "processing":
		return true
	default:
		return false
	}
}

func cancelTranscodeTasks(ctx context.Context, videoID int64, quality string) (int64, error) {
	query := store.DB().Where("video_id = ? AND status IN ?", videoID, activeTranscodeStatuses())
	if strings.TrimSpace(quality) != "" {
		query = query.Where("quality = ?", strings.ToLower(strings.TrimSpace(quality)))
	}
	var tasks []store.VideoTranscodeTask
	if err := query.Order("id asc").Find(&tasks).Error; err != nil {
		return 0, err
	}
	if len(tasks) == 0 {
		return 0, nil
	}

	ids := make([]int64, 0, len(tasks))
	previousStatus := ""
	for _, task := range tasks {
		ids = append(ids, task.ID)
		if previousStatus == "" && strings.TrimSpace(task.PreviousStatus) != "" {
			previousStatus = task.PreviousStatus
		}
	}
	now := time.Now()
	if err := store.DB().Model(&store.VideoTranscodeTask{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"status":         "canceled",
			"status_message": "已取消",
			"error_message":  "",
			"finished_at":    now,
		}).Error; err != nil {
		return 0, err
	}

	var remaining int64
	if err := store.DB().Model(&store.VideoTranscodeTask{}).
		Where("video_id = ? AND status IN ?", videoID, activeTranscodeStatuses()).
		Count(&remaining).Error; err != nil {
		return 0, err
	}
	if remaining == 0 {
		store.DB().Model(&store.Video{}).
			Where("id = ? AND status = ?", videoID, "transcoding").
			Updates(map[string]interface{}{"status": restoredTranscodeStatus(previousStatus), "updated_at": now})
	}
	finalizeTranscodeBatch(ctx, videoID, tasks[0].ID, previousStatus)
	return int64(len(tasks)), nil
}

func restoredTranscodeStatus(previousStatus string) string {
	switch previousStatus {
	case "ready", "offline", "uploaded", "failed":
		return previousStatus
	default:
		return "uploaded"
	}
}

func videoRecordExists(videoID int64) bool {
	var count int64
	if err := store.DB().Model(&store.Video{}).Where("id = ?", videoID).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func uploadedVideoSourceInfo(videoID int64, filename, contentType string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".mp4"
	}
	switch ext {
	case ".mp4":
		if strings.TrimSpace(contentType) == "" {
			contentType = "video/mp4"
		}
	case ".mkv":
		contentType = "video/x-matroska"
	default:
		return "", "", fmt.Errorf("unsupported video source format %s", ext)
	}
	return fmt.Sprintf("originals/%d/source%s", videoID, ext), contentType, nil
}

func normalizeTranscodeQualityNames(input []string) ([]string, error) {
	allowed := map[string]bool{
		"360p":  true,
		"480p":  true,
		"720p":  true,
		"1080p": true,
	}
	result := make([]string, 0, len(input))
	seen := map[string]bool{}
	for _, raw := range input {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if !allowed[name] {
			return nil, fmt.Errorf("unsupported transcode quality %s", raw)
		}
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}
	if len(result) == len(allowed) {
		return nil, nil
	}
	return result, nil
}

// GET /api/admin/videos/{id}/transcode
func AdminTranscodeStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	videoID, err := parseVideoID(r.URL.Path, "/api/admin/videos/", "/transcode")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	reconcileStaleTranscodeTasks(r.Context(), videoID)

	var task store.VideoTranscodeTask
	if err := store.DB().Where("video_id = ?", videoID).Order("id desc").First(&task).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "task not found"})
		return
	}
	if task.BatchID == 0 {
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: task})
		return
	}

	var tasks []store.VideoTranscodeTask
	if err := store.DB().Where("video_id = ? AND batch_id = ?", videoID, task.BatchID).Order("id asc").Find(&tasks).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: aggregateTranscodeStatus(task, tasks)})
}

func aggregateTranscodeStatus(base store.VideoTranscodeTask, tasks []store.VideoTranscodeTask) transcodeStatusResponse {
	resp := transcodeStatusResponse{
		VideoTranscodeTask: base,
		QualityStatuses:    make(map[string]string, len(tasks)),
		QualityMessages:    make(map[string]string, len(tasks)),
		QualityProgress:    make(map[string]int, len(tasks)),
	}
	if len(tasks) == 0 {
		return resp
	}

	hasProcessing := false
	hasPending := false
	hasFailed := false
	hasCanceled := false
	totalProgress := 0
	resp.ErrorMessage = ""
	for _, task := range tasks {
		totalProgress += task.Progress
		if task.Quality != "" {
			resp.QualityStatuses[task.Quality] = task.Status
			resp.QualityMessages[task.Quality] = task.StatusMessage
			resp.QualityProgress[task.Quality] = task.Progress
		}
		switch task.Status {
		case "queued":
			hasPending = true
		case "processing":
			hasProcessing = true
		case "pending":
			hasPending = true
		case "failed":
			hasFailed = true
			if resp.ErrorMessage == "" {
				resp.ErrorMessage = task.ErrorMessage
				if task.Quality != "" && resp.ErrorMessage != "" {
					resp.ErrorMessage = task.Quality + ": " + resp.ErrorMessage
				}
			}
		case "canceled":
			hasCanceled = true
		}
	}

	switch {
	case hasProcessing:
		resp.Status = "processing"
	case hasPending:
		resp.Status = "pending"
	case hasFailed:
		resp.Status = "failed"
	case hasCanceled:
		resp.Status = "canceled"
	default:
		resp.Status = "success"
	}
	resp.Progress = totalProgress / len(tasks)
	resp.StatusMessage = aggregateTranscodeMessage(tasks, resp.Status)
	resp.BatchID = tasks[0].BatchID
	return resp
}

func aggregateTranscodeMessage(tasks []store.VideoTranscodeTask, status string) string {
	for _, task := range tasks {
		if task.Status == "processing" && task.StatusMessage != "" {
			return task.Quality + " " + task.StatusMessage
		}
	}
	for _, task := range tasks {
		if (task.Status == "queued" || task.Status == "pending") && task.StatusMessage != "" {
			return task.Quality + " " + task.StatusMessage
		}
	}
	for _, task := range tasks {
		if task.Status == "failed" && task.ErrorMessage != "" {
			if task.Quality != "" {
				return task.Quality + " 失败"
			}
			return task.ErrorMessage
		}
	}
	if status == "success" {
		return "完成"
	}
	if status == "canceled" {
		return "已取消"
	}
	return ""
}

// GET /api/admin/videos
func AdminListVideosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	if !common.HasPagination(r) {
		var videos []store.Video
		store.DB().Order("id desc").Find(&videos)
		attachVideoTranscodeMetadata(r.Context(), videos)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: videos})
		return
	}
	p := common.ParsePagination(r, 20, 100)
	var total int64
	store.DB().Model(&store.Video{}).Count(&total)
	var videos []store.Video
	store.DB().Order("id desc").Offset(p.Offset).Limit(p.PerPage).Find(&videos)
	attachVideoTranscodeMetadata(r.Context(), videos)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: common.PageResponse(videos, total, p)})
}

// GET /api/admin/videos/{id}
func AdminGetVideoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/admin/videos/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	var v store.Video
	if err := store.DB().First(&v, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	videos := []store.Video{v}
	attachVideoTranscodeMetadata(r.Context(), videos)
	v = videos[0]
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: v})
}

func attachVideoTranscodeMetadata(ctx context.Context, videos []store.Video) {
	transcoding := activeTranscodeVideoIDs(videos)
	for i := range videos {
		videos[i].TranscodedQualities = transcodedQualityNames(ctx, videos[i])
		videos[i].AvailableTranscodeQualities = availableTranscodeQualityNames(videos[i])
		videos[i].Transcoding = transcoding[videos[i].ID]
	}
}

// activeTranscodeVideoIDs returns, in one query, the set of videos that have an
// active transcode task. Used so the admin list can flag in-progress transcodes
// even for videos that stay "ready" during a merge re-transcode.
func activeTranscodeVideoIDs(videos []store.Video) map[int64]bool {
	result := make(map[int64]bool, len(videos))
	if len(videos) == 0 {
		return result
	}
	ids := make([]int64, 0, len(videos))
	for _, v := range videos {
		ids = append(ids, v.ID)
	}
	var activeIDs []int64
	store.DB().Model(&store.VideoTranscodeTask{}).
		Where("video_id IN ? AND status IN ?", ids, activeTranscodeStatuses()).
		Distinct("video_id").
		Pluck("video_id", &activeIDs)
	for _, id := range activeIDs {
		result[id] = true
	}
	return result
}

func transcodedQualityNames(ctx context.Context, v store.Video) []string {
	key := strings.TrimSpace(v.HLSMasterKey)
	if key == "" {
		key = fmt.Sprintf("hls/%d/master.m3u8", v.ID)
	}
	raw, err := readMinioText(ctx, key)
	if err != nil && key != fmt.Sprintf("hls/%d/master.m3u8", v.ID) {
		raw, err = readMinioText(ctx, fmt.Sprintf("hls/%d/master.m3u8", v.ID))
	}
	if err != nil {
		return nil
	}

	names := make([]string, 0)
	seen := make(map[string]bool)
	for _, entry := range parseMasterPlaylistEntries(raw) {
		if entry.name == "" || seen[entry.name] {
			continue
		}
		names = append(names, entry.name)
		seen[entry.name] = true
	}
	return names
}

func availableTranscodeQualityNames(v store.Video) []string {
	if sourceSize := sourceSizeFromVideo(v); hasSourceSize(sourceSize) {
		return transcodeQualityNames(selectTranscodeQualities(sourceSize))
	}
	return failedAvailableTranscodeQualityNames(v.ID)
}

func failedAvailableTranscodeQualityNames(videoID int64) []string {
	var task store.VideoTranscodeTask
	err := store.DB().
		Where("video_id = ? AND error_message LIKE ?", videoID, "selected transcode qualities are not available for source; available:%").
		Order("id desc").
		First(&task).Error
	if err != nil {
		return nil
	}
	return parseAvailableTranscodeQualityNames(task.ErrorMessage)
}

func parseAvailableTranscodeQualityNames(message string) []string {
	const marker = "available:"
	idx := strings.Index(message, marker)
	if idx < 0 {
		return nil
	}
	raw := message[idx+len(marker):]
	names := make([]string, 0)
	seen := make(map[string]bool)
	for _, item := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(item))
		if qualityHeight(name) <= 0 || seen[name] {
			continue
		}
		names = append(names, name)
		seen[name] = true
	}
	return names
}

func normalizeCatalogNames(values []string, maxItems, maxRunes int) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		item := trimCatalogText(value, maxRunes)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
		if len(result) >= maxItems {
			break
		}
	}
	return result
}

func trimCatalogText(value string, maxRunes int) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return text
}

func normalizeReleaseYear(year int) int {
	if year < 0 || year > time.Now().Year()+2 {
		return 0
	}
	return year
}

// PUT /api/admin/videos/{id}
func AdminUpdateVideoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/admin/videos/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	var req struct {
		Title       *string   `json:"title"`
		Description *string   `json:"description"`
		CategoryID  *int      `json:"category_id"`
		Actors      *[]string `json:"actors"`
		Directors   *[]string `json:"directors"`
		Genres      *[]string `json:"genres"`
		Region      *string   `json:"region"`
		ReleaseYear *int      `json:"release_year"`
		Language    *string   `json:"language"`
		IsVip       *bool     `json:"is_vip"`
		IsFree      *bool     `json:"is_free"`
		Status      *string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Actors != nil {
		updates["actors"] = store.StringArray(normalizeCatalogNames(*req.Actors, 16, 40))
	}
	if req.Directors != nil {
		updates["directors"] = store.StringArray(normalizeCatalogNames(*req.Directors, 8, 40))
	}
	if req.Genres != nil {
		updates["genres"] = store.StringArray(normalizeCatalogNames(*req.Genres, 12, 24))
	}
	if req.Region != nil {
		updates["region"] = trimCatalogText(*req.Region, 128)
	}
	if req.ReleaseYear != nil {
		updates["release_year"] = normalizeReleaseYear(*req.ReleaseYear)
	}
	if req.Language != nil {
		updates["language"] = trimCatalogText(*req.Language, 128)
	}
	if req.IsVip != nil {
		updates["is_vip"] = *req.IsVip
	}
	if req.IsFree != nil {
		updates["is_free"] = *req.IsFree
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	store.DB().Model(&store.Video{}).Where("id = ?", id).Updates(updates)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// DELETE /api/admin/videos/{id}
func AdminDeleteVideoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/admin/videos/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	var v store.Video
	if err := store.DB().First(&v, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	if _, err := cancelTranscodeTasks(r.Context(), id, ""); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if err := store.DB().Where("video_id = ?", id).Delete(&store.VideoMediaTrack{}).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if err := store.DB().Where("video_id = ?", id).Delete(&store.VideoTranscodeTask{}).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if err := store.DB().Where("video_id = ?", id).Delete(&store.VideoExtractTrackTask{}).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if err := store.DB().Delete(&store.Video{}, id).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	deleteVideoObjects(r.Context(), id)
	removeVideoSourceCache(id)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// deleteVideoObjects removes all MinIO objects for a video: original, HLS segments, cover.
func deleteVideoObjects(ctx context.Context, videoID int64) {
	prefixes := []string{
		fmt.Sprintf("originals/%d/", videoID),
		fmt.Sprintf("hls/%d/", videoID),
		fmt.Sprintf("covers/%d/", videoID),
	}
	for _, prefix := range prefixes {
		removeObjectsByPrefix(ctx, prefix)
	}
}

// removeObjectsByPrefix deletes every MinIO object under the given prefix.
func removeObjectsByPrefix(ctx context.Context, prefix string) {
	objectCh := store.ObjectClient().ListObjects(ctx, store.VideoBucket(), minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	removeCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(removeCh)
		for obj := range objectCh {
			if obj.Err == nil {
				removeCh <- obj
			}
		}
	}()
	for result := range store.ObjectClient().RemoveObjects(ctx, store.VideoBucket(), removeCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			log.Printf("removeObjectsByPrefix %s: %v", result.ObjectName, result.Err)
		}
	}
}

func parseVideoID(path, prefix, suffix string) (int64, error) {
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimSuffix(s, suffix)
	return strconv.ParseInt(s, 10, 64)
}
