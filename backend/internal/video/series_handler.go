package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"

	"github.com/minio/minio-go/v7"
)

// seriesEpisode is the trimmed episode view returned in series listings. It is a
// video row reduced to what the app needs to render and launch playback.
type seriesEpisode struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	EpisodeNumber int    `json:"episode_number"`
	Duration      int    `json:"duration"`
	Status        string `json:"status"`
	IsVip         bool   `json:"is_vip"`
	IsFree        bool   `json:"is_free"`
	CoverURL      string `json:"cover_url"`
}

// ===========================================================================
// Admin
// ===========================================================================

// GET  /api/admin/series        — list (optional ?q= over title, ?category_id=)
// POST /api/admin/series        — create (series:create, enforced in router)
func AdminSeriesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		adminListSeries(w, r)
	case http.MethodPost:
		adminCreateSeries(w, r)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func adminListSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	keyword := strings.TrimSpace(q.Get("q"))
	categoryID, _ := strconv.Atoi(q.Get("category_id"))

	db := store.DB().Model(&store.Series{})
	if keyword != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	if categoryID > 0 {
		db = db.Where("category_id = ?", categoryID)
	}
	var list []store.Series
	db.Order("id desc").Find(&list)
	decorateSeries(list)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: list})
}

type seriesUpsertRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	CategoryID  int      `json:"category_id"`
	Region      string   `json:"region"`
	ReleaseYear int      `json:"release_year"`
	Genres      []string `json:"genres"`
	IsVip       bool     `json:"is_vip"`
	Status      string   `json:"status"`
}

func adminCreateSeries(w http.ResponseWriter, r *http.Request) {
	var req seriesUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "title required"})
		return
	}
	s := &store.Series{
		Title:       trimCatalogText(req.Title, 255),
		Description: req.Description,
		CategoryID:  req.CategoryID,
		Region:      trimCatalogText(req.Region, 128),
		ReleaseYear: normalizeReleaseYear(req.ReleaseYear),
		Genres:      store.StringArray(normalizeCatalogNames(req.Genres, 12, 24)),
		IsVip:       req.IsVip,
		Status:      normalizeSeriesStatus(req.Status),
	}
	if err := store.DB().Create(s).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: s})
}

// AdminSeriesByIDHandler dispatches everything under /api/admin/series/{id}. The
// route is mounted behind a plain admin-auth guard; per-action button
// permissions are enforced here (mirroring the videos sub-handler in router.go).
func AdminSeriesByIDHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/admin/series/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid series id"})
		return
	}

	// /api/admin/series/{id}/cover
	if len(parts) == 2 && parts[1] == "cover" {
		if !admin.EnsurePermission(w, r, "series:edit") {
			return
		}
		adminUploadSeriesCover(w, r, id)
		return
	}

	// /api/admin/series/{id}/episodes[/{videoId}]
	if len(parts) >= 2 && parts[1] == "episodes" {
		adminSeriesEpisodes(w, r, id, parts)
		return
	}

	switch r.Method {
	case http.MethodGet:
		adminGetSeries(w, r, id)
	case http.MethodPut:
		if !admin.EnsurePermission(w, r, "series:edit") {
			return
		}
		adminUpdateSeries(w, r, id)
	case http.MethodDelete:
		if !admin.EnsurePermission(w, r, "series:delete") {
			return
		}
		adminDeleteSeries(w, r, id)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func adminGetSeries(w http.ResponseWriter, r *http.Request, id int64) {
	var s store.Series
	if err := store.DB().First(&s, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	list := []store.Series{s}
	decorateSeries(list)
	episodes := loadSeriesEpisodes(id, false)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"series":   list[0],
		"episodes": episodes,
	}})
}

func adminUpdateSeries(w http.ResponseWriter, r *http.Request, id int64) {
	var req struct {
		Title       *string   `json:"title"`
		Description *string   `json:"description"`
		CategoryID  *int      `json:"category_id"`
		Region      *string   `json:"region"`
		ReleaseYear *int      `json:"release_year"`
		Genres      *[]string `json:"genres"`
		IsVip       *bool     `json:"is_vip"`
		Status      *string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if req.Title != nil {
		updates["title"] = trimCatalogText(*req.Title, 255)
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Region != nil {
		updates["region"] = trimCatalogText(*req.Region, 128)
	}
	if req.ReleaseYear != nil {
		updates["release_year"] = normalizeReleaseYear(*req.ReleaseYear)
	}
	if req.Genres != nil {
		updates["genres"] = store.StringArray(normalizeCatalogNames(*req.Genres, 12, 24))
	}
	if req.IsVip != nil {
		updates["is_vip"] = *req.IsVip
	}
	if req.Status != nil {
		updates["status"] = normalizeSeriesStatus(*req.Status)
	}
	if err := store.DB().Model(&store.Series{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// adminDeleteSeries removes the series and detaches its episodes (resetting
// series_id to 0). The episode videos themselves are kept — only the grouping is
// removed, so no media is lost.
func adminDeleteSeries(w http.ResponseWriter, r *http.Request, id int64) {
	if err := store.DB().Model(&store.Video{}).Where("series_id = ?", id).
		Updates(map[string]interface{}{"series_id": 0, "episode_number": 0, "updated_at": time.Now()}).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if err := store.DB().Delete(&store.Series{}, id).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// adminSeriesEpisodes manages the episode membership of a series.
//
//	GET    /api/admin/series/{id}/episodes            — list episodes
//	POST   /api/admin/series/{id}/episodes            — assign a video (series:edit)
//	DELETE /api/admin/series/{id}/episodes/{videoId}  — detach a video (series:edit)
func adminSeriesEpisodes(w http.ResponseWriter, r *http.Request, seriesID int64, parts []string) {
	// parts == [id, "episodes", maybe videoId]
	switch r.Method {
	case http.MethodGet:
		episodes := loadSeriesEpisodes(seriesID, false)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: episodes})
	case http.MethodPost:
		if !admin.EnsurePermission(w, r, "series:edit") {
			return
		}
		var req struct {
			VideoID       int64 `json:"video_id"`
			EpisodeNumber int   `json:"episode_number"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}
		if req.VideoID <= 0 {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "video_id required"})
			return
		}
		// Ensure both the series and the video exist before linking them.
		if err := store.DB().Select("id").First(&store.Series{}, seriesID).Error; err != nil {
			common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "series not found"})
			return
		}
		var v store.Video
		if err := store.DB().Select("id", "series_id").First(&v, req.VideoID).Error; err != nil {
			common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
			return
		}
		if v.SeriesID != 0 && v.SeriesID != seriesID {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "该视频已属于其他剧集"})
			return
		}
		epNo := req.EpisodeNumber
		if epNo <= 0 {
			epNo = nextEpisodeNumber(seriesID)
		}
		if err := store.DB().Model(&store.Video{}).Where("id = ?", req.VideoID).
			Updates(map[string]interface{}{"series_id": seriesID, "episode_number": epNo, "updated_at": time.Now()}).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
	case http.MethodDelete:
		if !admin.EnsurePermission(w, r, "series:edit") {
			return
		}
		if len(parts) < 3 {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "video id required"})
			return
		}
		videoID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
			return
		}
		if err := store.DB().Model(&store.Video{}).Where("id = ? AND series_id = ?", videoID, seriesID).
			Updates(map[string]interface{}{"series_id": 0, "episode_number": 0, "updated_at": time.Now()}).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func adminUploadSeriesCover(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var s store.Series
	if err := store.DB().First(&s, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "series not found"})
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
	key := fmt.Sprintf("covers/series/%d/cover.%s", id, ext)
	if _, err := store.ObjectClient().PutObject(
		context.Background(), store.VideoBucket(), key, file, header.Size,
		minio.PutObjectOptions{ContentType: contentType},
	); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "upload failed: " + err.Error()})
		return
	}
	store.DB().Model(&s).Updates(map[string]interface{}{"cover_key": key, "updated_at": time.Now()})
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]string{"cover_key": key}})
}

// ===========================================================================
// App (public)
// ===========================================================================

// GET /api/series — public list of published series.
func AppListSeriesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	q := r.URL.Query()
	keyword := strings.TrimSpace(q.Get("q"))
	categoryID, _ := strconv.Atoi(q.Get("category_id"))

	db := store.DB().Model(&store.Series{}).Where("status <> ?", "offline")
	if keyword != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	if categoryID > 0 {
		db = db.Where("category_id = ?", categoryID)
	}
	var list []store.Series
	db.Order("id desc").Find(&list)
	decorateSeries(list)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"items": list,
		"total": len(list),
	}})
}

// GET /api/series/{id} — series detail with its ready episodes (app-facing).
func AppGetSeriesHandler(w http.ResponseWriter, r *http.Request, id int64) {
	var s store.Series
	if err := store.DB().First(&s, id).Error; err != nil || s.Status == "offline" {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	list := []store.Series{s}
	decorateSeries(list)
	episodes := loadSeriesEpisodes(id, true)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"series":   list[0],
		"episodes": episodes,
	}})
}

// GET /api/series/{id}/cover — serve the series poster (or fall back to nothing).
func AppSeriesCoverHandler(w http.ResponseWriter, r *http.Request, id int64) {
	var s store.Series
	if store.DB().First(&s, id).Error != nil || s.CoverKey == "" {
		http.NotFound(w, r)
		return
	}
	obj, err := store.ObjectClient().GetObject(r.Context(), store.VideoBucket(), s.CoverKey, minio.GetObjectOptions{})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer obj.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, obj)
}

// ===========================================================================
// Helpers
// ===========================================================================

// loadSeriesEpisodes returns a series' episodes ordered by episode number. When
// readyOnly is set (app-facing) it filters to playable videos.
func loadSeriesEpisodes(seriesID int64, readyOnly bool) []seriesEpisode {
	db := store.DB().Where("series_id = ?", seriesID)
	if readyOnly {
		db = db.Where("status = ?", "ready")
	}
	var videos []store.Video
	db.Order("episode_number asc, id asc").Find(&videos)
	episodes := make([]seriesEpisode, len(videos))
	for i, v := range videos {
		episodes[i] = seriesEpisode{
			ID:            v.ID,
			Title:         v.Title,
			EpisodeNumber: v.EpisodeNumber,
			Duration:      v.Duration,
			Status:        v.Status,
			IsVip:         v.IsVip,
			IsFree:        v.IsFree,
			CoverURL:      coverURL(v),
		}
	}
	return episodes
}

// decorateSeries fills episode counts, cover URLs and category names for a batch
// of series in a few queries (no N+1).
func decorateSeries(list []store.Series) {
	if len(list) == 0 {
		return
	}
	ids := make([]int64, len(list))
	catIDSet := map[int]struct{}{}
	for i, s := range list {
		ids[i] = s.ID
		if s.CategoryID > 0 {
			catIDSet[s.CategoryID] = struct{}{}
		}
	}

	// Episode counts + a fallback cover (first episode) per series, in one query.
	type epRow struct {
		SeriesID   int64
		Count      int
		FirstID    int64
		FirstCover string
	}
	var rows []epRow
	store.DB().Model(&store.Video{}).
		Select("series_id, COUNT(*) as count, MIN(id) as first_id").
		Where("series_id IN ?", ids).
		Group("series_id").
		Scan(&rows)
	countBySeries := make(map[int64]int, len(rows))
	firstIDBySeries := make(map[int64]int64, len(rows))
	for _, row := range rows {
		countBySeries[row.SeriesID] = row.Count
		firstIDBySeries[row.SeriesID] = row.FirstID
	}

	catNames := map[int]string{}
	if len(catIDSet) > 0 {
		catIDs := make([]int, 0, len(catIDSet))
		for id := range catIDSet {
			catIDs = append(catIDs, id)
		}
		var cats []store.Category
		store.DB().Where("id IN ?", catIDs).Find(&cats)
		for _, c := range cats {
			catNames[c.ID] = c.Name
		}
	}

	for i := range list {
		s := &list[i]
		s.EpisodeCount = countBySeries[s.ID]
		s.CategoryName = catNames[s.CategoryID]
		if s.CoverKey != "" {
			s.CoverURL = "/api/series/" + strconv.FormatInt(s.ID, 10) + "/cover"
		} else if firstID := firstIDBySeries[s.ID]; firstID > 0 {
			// Fall back to the first episode's cover so the series tile is never blank.
			s.CoverURL = "/api/videos/" + strconv.FormatInt(firstID, 10) + "/cover"
		}
	}
}

// nextEpisodeNumber returns one past the highest episode number in the series so
// a freshly-attached video lands at the end by default.
func nextEpisodeNumber(seriesID int64) int {
	var max *int
	store.DB().Model(&store.Video{}).Where("series_id = ?", seriesID).
		Select("MAX(episode_number)").Scan(&max)
	if max == nil {
		return 1
	}
	return *max + 1
}

func normalizeSeriesStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "completed":
		return "completed"
	case "offline":
		return "offline"
	default:
		return "ongoing"
	}
}
