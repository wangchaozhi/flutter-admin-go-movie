package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// POST /api/admin/videos
func AdminCreateVideoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		CategoryID  int    `json:"category_id"`
		IsVip       bool   `json:"is_vip"`
		IsFree      bool   `json:"is_free"`
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

	key := fmt.Sprintf("originals/%d/source.mp4", videoID)
	_, err = store.ObjectClient().PutObject(
		context.Background(),
		store.VideoBucket(),
		key,
		file,
		header.Size,
		minio.PutObjectOptions{ContentType: "video/mp4"},
	)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "upload failed: " + err.Error()})
		return
	}

	now := time.Now()
	store.DB().Model(&v).Updates(map[string]interface{}{
		"original_key": key,
		"size":         header.Size,
		"status":       "uploaded",
		"updated_at":   now,
	})

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"video_id":     videoID,
		"original_key": key,
		"size":         header.Size,
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
	previousStatus := v.Status
	mergeExisting := v.Status == "ready" || v.Status == "offline"

	task := &store.VideoTranscodeTask{
		VideoID: videoID,
		Status:  "pending",
	}
	if err := store.DB().Create(task).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	store.DB().Model(&v).Updates(map[string]interface{}{"status": "transcoding", "updated_at": time.Now()})

	if err := EnqueueTranscode(r.Context(), videoID, task.ID, qualities, mergeExisting, previousStatus); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "enqueue failed: " + err.Error()})
		return
	}

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"task_id": task.ID,
		"status":  "pending",
	}})
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

	var task store.VideoTranscodeTask
	if err := store.DB().Where("video_id = ?", videoID).Order("id desc").First(&task).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "task not found"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: task})
}

// GET /api/admin/videos
func AdminListVideosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var videos []store.Video
	store.DB().Order("id desc").Find(&videos)
	attachTranscodedQualities(r.Context(), videos)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: videos})
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
	v.TranscodedQualities = transcodedQualityNames(r.Context(), v)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: v})
}

func attachTranscodedQualities(ctx context.Context, videos []store.Video) {
	for i := range videos {
		videos[i].TranscodedQualities = transcodedQualityNames(ctx, videos[i])
	}
}

func transcodedQualityNames(ctx context.Context, v store.Video) []string {
	key := strings.TrimSpace(v.HLSMasterKey)
	if key == "" {
		return nil
	}
	raw, err := readMinioText(ctx, key)
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
		Title       *string `json:"title"`
		Description *string `json:"description"`
		CategoryID  *int    `json:"category_id"`
		IsVip       *bool   `json:"is_vip"`
		IsFree      *bool   `json:"is_free"`
		Status      *string `json:"status"`
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
	deleteVideoObjects(r.Context(), id)
	store.DB().Where("video_id = ?", id).Delete(&store.VideoTranscodeTask{})
	store.DB().Delete(&store.Video{}, id)
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
				log.Printf("deleteVideoObjects %s: %v", result.ObjectName, result.Err)
			}
		}
	}
}

func parseVideoID(path, prefix, suffix string) (int64, error) {
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimSuffix(s, suffix)
	return strconv.ParseInt(s, 10, 64)
}
