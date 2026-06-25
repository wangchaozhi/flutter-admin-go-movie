package video

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"flutter-admin-go/internal/ai"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/store"
)

// POST /api/admin/videos/{id}/ai-metadata
func AdminGenerateAIMetadataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	videoID, err := parseVideoID(r.URL.Path, "/api/admin/videos/", "/ai-metadata")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	var req struct {
		OverwriteDescription bool `json:"overwrite_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}

	var v store.Video
	if err := store.DB().First(&v, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}
	categoryName := categoryNameForVideo(v.CategoryID)
	provider, err := ai.NewProvider(config.Load().AI)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ai.ErrDisabled) {
			status = http.StatusServiceUnavailable
		}
		common.WriteJSON(w, status, common.APIResponse{Code: status, Msg: err.Error()})
		return
	}

	now := time.Now()
	generated, err := provider.GenerateVideoMetadata(r.Context(), ai.VideoMetadataInput{
		Title:       v.Title,
		Description: v.Description,
		Category:    categoryName,
		Actors:      []string(v.Actors),
		Directors:   []string(v.Directors),
		Genres:      []string(v.Genres),
		Region:      v.Region,
		ReleaseYear: v.ReleaseYear,
		Language:    v.Language,
		Duration:    v.Duration,
		Width:       v.SourceWidth,
		Height:      v.SourceHeight,
		IsVIP:       v.IsVip,
		IsFree:      v.IsFree,
	})
	if err != nil {
		saveAIMetadataFailure(v.ID, provider.Name(), provider.Model(), err, now)
		common.WriteJSON(w, http.StatusBadGateway, common.APIResponse{Code: 502, Msg: err.Error()})
		return
	}

	meta := store.VideoAIMetadata{
		VideoID:     v.ID,
		Provider:    provider.Name(),
		Model:       provider.Model(),
		Status:      "ready",
		Synopsis:    generated.Synopsis,
		Highlights:  store.StringArray(generated.Highlights),
		Tags:        store.StringArray(generated.Tags),
		GeneratedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.DB().Save(&meta).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if req.OverwriteDescription || strings.TrimSpace(v.Description) == "" {
		store.DB().Model(&store.Video{}).
			Where("id = ?", v.ID).
			Updates(map[string]interface{}{"description": generated.Synopsis, "updated_at": now})
		v.Description = generated.Synopsis
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: meta})
}

func categoryNameForVideo(categoryID int) string {
	if categoryID <= 0 {
		return ""
	}
	var cat store.Category
	if store.DB().First(&cat, categoryID).Error == nil {
		return cat.Name
	}
	return ""
}

func saveAIMetadataFailure(videoID int64, provider, model string, runErr error, now time.Time) {
	errorMessage := strings.TrimSpace(runErr.Error())
	var existing store.VideoAIMetadata
	if err := store.DB().First(&existing, "video_id = ?", videoID).Error; err == nil {
		updates := map[string]interface{}{
			"provider":      provider,
			"model":         model,
			"error_message": errorMessage,
			"updated_at":    now,
		}
		if existing.Status != "ready" {
			updates["status"] = "failed"
		}
		_ = store.DB().Model(&store.VideoAIMetadata{}).Where("video_id = ?", videoID).Updates(updates).Error
		return
	}
	meta := store.VideoAIMetadata{
		VideoID:      videoID,
		Provider:     provider,
		Model:        model,
		Status:       "failed",
		ErrorMessage: errorMessage,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = store.DB().Save(&meta).Error
}

func loadVideoAIMetadata(videoID int64) *store.VideoAIMetadata {
	var meta store.VideoAIMetadata
	if err := store.DB().First(&meta, "video_id = ?", videoID).Error; err != nil {
		return nil
	}
	if meta.Status != "ready" {
		return nil
	}
	return &meta
}
