package video

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"

	"gorm.io/gorm"
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
	// When the caller is authenticated, flag which bullets they have already
	// liked so the player can show a filled heart without an extra round-trip.
	if userID, ok := parseMobileAuth(r); ok && len(items) > 0 {
		ids := make([]int64, len(items))
		for i, it := range items {
			ids[i] = it.ID
		}
		var likedIDs []int64
		store.DB().Model(&store.DanmakuLike{}).
			Where("user_id = ? AND danmaku_id IN ?", userID, ids).
			Pluck("danmaku_id", &likedIDs)
		liked := make(map[int64]struct{}, len(likedIDs))
		for _, id := range likedIDs {
			liked[id] = struct{}{}
		}
		for i := range items {
			if _, ok := liked[items[i].ID]; ok {
				items[i].Liked = true
			}
		}
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

// AppDanmakuByIDHandler handles bullet-scoped actions (mobile auth):
//   DELETE /api/mobile/danmaku/{id}        — delete the caller's own bullet
//   POST   /api/mobile/danmaku/{id}/like   — toggle a like on any bullet
func AppDanmakuByIDHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/mobile/danmaku/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}

	if len(parts) >= 2 && parts[1] == "like" {
		toggleDanmakuLike(w, r, id, userID)
		return
	}

	if r.Method != http.MethodDelete {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	result := store.DB().Where("id = ? AND user_id = ?", id, userID).Delete(&store.VideoDanmaku{})
	if result.Error != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "弹幕不存在或无权删除"})
		return
	}
	// Best-effort cleanup of orphaned likes for the deleted bullet.
	store.DB().Where("danmaku_id = ?", id).Delete(&store.DanmakuLike{})
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// toggleDanmakuLike likes the bullet if the user hasn't liked it yet, otherwise
// unlikes it. The denormalised like_count is kept in sync inside the same
// transaction. Returns the new {liked, like_count}.
func toggleDanmakuLike(w http.ResponseWriter, r *http.Request, danmakuID int64, userID int) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var liked bool
	var likeCount int
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		var bullet store.VideoDanmaku
		if err := tx.First(&bullet, danmakuID).Error; err != nil {
			return err
		}
		like := store.DanmakuLike{DanmakuID: danmakuID, UserID: int64(userID)}
		res := tx.Where("danmaku_id = ? AND user_id = ?", danmakuID, userID).Delete(&store.DanmakuLike{})
		if res.Error != nil {
			return res.Error
		}
		delta := 1
		if res.RowsAffected > 0 {
			// Existing like removed -> unlike.
			liked = false
			delta = -1
		} else {
			if err := tx.Create(&like).Error; err != nil {
				return err
			}
			liked = true
		}
		// Clamp at 0 so a stale count can never go negative.
		if err := tx.Model(&store.VideoDanmaku{}).Where("id = ?", danmakuID).
			Update("like_count", gorm.Expr("GREATEST(like_count + ?, 0)", delta)).Error; err != nil {
			return err
		}
		var refreshed store.VideoDanmaku
		if err := tx.Select("like_count").First(&refreshed, danmakuID).Error; err != nil {
			return err
		}
		likeCount = refreshed.LikeCount
		return nil
	})
	if err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "弹幕不存在"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"liked":      liked,
		"like_count": likeCount,
	}})
}
