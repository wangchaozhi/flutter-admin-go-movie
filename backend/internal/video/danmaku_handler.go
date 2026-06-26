package video

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

const (
	maxDanmakuLength = 100
	// maxDanmakuPerVideo caps how many bullets one playback fetch returns so a
	// hugely-commented video can't flood the client. Newest are preferred.
	maxDanmakuPerVideo  = 2000
	defaultDanmakuColor = 0xFFFFFF
)

// danmakuLimiter throttles bullet submissions per user. Bullets are short and
// high-frequency by nature, so the window is more permissive than comments.
var danmakuLimiter = common.NewLimiter("danmaku_post", 20, time.Minute)

// GET  /api/videos/{id}/danmaku — public, returns bullets ordered by position.
// POST /api/videos/{id}/danmaku — create one bullet (mobile auth).
func AppVideoDanmakuHandler(w http.ResponseWriter, r *http.Request) {
	videoID, err := parseDanmakuVideoID(r.URL.Path)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		listVideoDanmaku(w, r, videoID)
	case http.MethodPost:
		createVideoDanmaku(w, r, videoID)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func listVideoDanmaku(w http.ResponseWriter, r *http.Request, videoID int64) {
	var items []store.VideoDanmaku
	store.DB().Where("video_id = ?", videoID).
		Order("time_ms asc").
		Limit(maxDanmakuPerVideo).
		Find(&items)
	if items == nil {
		items = []store.VideoDanmaku{}
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"items": items,
		"total": len(items),
	}})
}

func createVideoDanmaku(w http.ResponseWriter, r *http.Request, videoID int64) {
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	if allowed, retry := danmakuLimiter.Allow(strconv.Itoa(userID)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		common.WriteJSON(w, http.StatusTooManyRequests, common.APIResponse{Code: 429, Msg: "弹幕太频繁，请稍后再试"})
		return
	}
	var req struct {
		Content string `json:"content"`
		TimeMS  int    `json:"time_ms"`
		Color   int    `json:"color"`
		Mode    int    `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "弹幕内容不能为空"})
		return
	}
	if len([]rune(req.Content)) > maxDanmakuLength {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "弹幕太长"})
		return
	}
	if req.TimeMS < 0 {
		req.TimeMS = 0
	}
	// Clamp color to a 24-bit RGB value; fall back to white for the default 0.
	if req.Color <= 0 || req.Color > 0xFFFFFF {
		req.Color = defaultDanmakuColor
	}
	if req.Mode < 0 || req.Mode > 2 {
		req.Mode = 0
	}

	// Guard against attaching a bullet to a non-existent video.
	if err := store.DB().Select("id").First(&store.Video{}, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}

	bullet := store.VideoDanmaku{
		VideoID:   videoID,
		UserID:    int64(userID),
		Content:   req.Content,
		TimeMS:    req.TimeMS,
		Color:     req.Color,
		Mode:      req.Mode,
		CreatedAt: time.Now(),
	}
	if err := store.DB().Create(&bullet).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: bullet})
}

func parseDanmakuVideoID(path string) (int64, error) {
	rest := strings.TrimPrefix(path, "/api/videos/")
	rest = strings.TrimSuffix(rest, "/danmaku")
	return strconv.ParseInt(strings.Trim(rest, "/"), 10, 64)
}
