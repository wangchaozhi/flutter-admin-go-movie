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
	maxCommentLength = 1000
	commentPageSize  = 20
)

// AppCommentItem is a comment enriched with the author's display name so the app
// does not have to resolve users separately.
type AppCommentItem struct {
	store.VideoComment
	Nickname string `json:"nickname"`
	Username string `json:"username"`
}

// commentSummary carries the aggregate rating/comment counts for a video.
type commentSummary struct {
	Count         int64   `json:"count"`
	RatingCount   int64   `json:"rating_count"`
	AverageRating float64 `json:"average_rating"`
}

// GET  /api/videos/{id}/comments   — public, paginated, newest first + summary
// POST /api/videos/{id}/comments   — create (mobile auth)
func AppVideoCommentsHandler(w http.ResponseWriter, r *http.Request) {
	videoID, err := parseCommentVideoID(r.URL.Path)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		listVideoComments(w, r, videoID)
	case http.MethodPost:
		createVideoComment(w, r, videoID)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func listVideoComments(w http.ResponseWriter, r *http.Request, videoID int64) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	var total int64
	store.DB().Model(&store.VideoComment{}).Where("video_id = ?", videoID).Count(&total)

	var summary commentSummary
	store.DB().Model(&store.VideoComment{}).
		Select("count(*) as count, count(*) filter (where rating > 0) as rating_count, coalesce(avg(rating) filter (where rating > 0), 0) as average_rating").
		Where("video_id = ?", videoID).
		Scan(&summary)

	var comments []store.VideoComment
	store.DB().Where("video_id = ?", videoID).
		Order("id desc").
		Offset((page - 1) * commentPageSize).
		Limit(commentPageSize).
		Find(&comments)

	items := decorateComments(comments)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"items":    items,
		"total":    total,
		"page":     page,
		"per_page": commentPageSize,
		"summary":  summary,
	}})
}

func createVideoComment(w http.ResponseWriter, r *http.Request, videoID int64) {
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	var req struct {
		Content string `json:"content"`
		Rating  int    `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Rating < 0 || req.Rating > 5 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "rating must be between 0 and 5"})
		return
	}
	if req.Content == "" && req.Rating == 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "comment or rating required"})
		return
	}
	if len([]rune(req.Content)) > maxCommentLength {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "comment too long"})
		return
	}

	// Guard against commenting on a non-existent video.
	if err := store.DB().Select("id").First(&store.Video{}, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}

	now := time.Now()
	comment := store.VideoComment{
		VideoID:   videoID,
		UserID:    int64(userID),
		Content:   req.Content,
		Rating:    req.Rating,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.DB().Create(&comment).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	items := decorateComments([]store.VideoComment{comment})
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: items[0]})
}

// DELETE /api/mobile/comments/{id} — delete the caller's own comment.
func AppCommentByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/mobile/comments/"), 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	result := store.DB().Where("id = ? AND user_id = ?", id, userID).Delete(&store.VideoComment{})
	if result.Error != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "comment not found"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// decorateComments resolves author display names for a batch of comments in one
// query to avoid N+1 lookups.
func decorateComments(comments []store.VideoComment) []AppCommentItem {
	items := make([]AppCommentItem, len(comments))
	if len(comments) == 0 {
		return items
	}
	idSet := map[int64]struct{}{}
	for _, c := range comments {
		idSet[c.UserID] = struct{}{}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	type userRow struct {
		ID       int64
		Username string
		Nickname string
	}
	var users []userRow
	store.DB().Model(&store.MobileUser{}).Select("id, username, nickname").Where("id IN ?", ids).Scan(&users)
	nameByID := make(map[int64]userRow, len(users))
	for _, u := range users {
		nameByID[u.ID] = u
	}
	for i, c := range comments {
		u := nameByID[c.UserID]
		items[i] = AppCommentItem{VideoComment: c, Nickname: u.Nickname, Username: u.Username}
	}
	return items
}

// AdminCommentItem is a comment enriched with author and video title for the
// moderation list.
type AdminCommentItem struct {
	store.VideoComment
	Nickname   string `json:"nickname"`
	Username   string `json:"username"`
	VideoTitle string `json:"video_title"`
}

// GET /api/admin/comments — paginated moderation list, optional ?q= over content.
func AdminListCommentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	base := store.DB().Model(&store.VideoComment{})
	if keyword != "" {
		base = base.Where("content ILIKE ?", "%"+keyword+"%")
	}

	p := common.ParsePagination(r, commentPageSize, 100)
	var total int64
	base.Count(&total)

	var comments []store.VideoComment
	base.Order("id desc").Offset(p.Offset).Limit(p.PerPage).Find(&comments)

	items := make([]AdminCommentItem, 0, len(comments))
	if len(comments) > 0 {
		decorated := decorateComments(comments)
		videoIDSet := map[int64]struct{}{}
		for _, c := range comments {
			videoIDSet[c.VideoID] = struct{}{}
		}
		videoIDs := make([]int64, 0, len(videoIDSet))
		for id := range videoIDSet {
			videoIDs = append(videoIDs, id)
		}
		type videoRow struct {
			ID    int64
			Title string
		}
		var videos []videoRow
		store.DB().Model(&store.Video{}).Select("id, title").Where("id IN ?", videoIDs).Scan(&videos)
		titleByID := make(map[int64]string, len(videos))
		for _, v := range videos {
			titleByID[v.ID] = v.Title
		}
		for _, d := range decorated {
			items = append(items, AdminCommentItem{
				VideoComment: d.VideoComment,
				Nickname:     d.Nickname,
				Username:     d.Username,
				VideoTitle:   titleByID[d.VideoID],
			})
		}
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: common.PageResponse(items, total, p)})
}

// DELETE /api/admin/comments/{id} — remove any comment (moderation).
func AdminDeleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/admin/comments/"), 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	result := store.DB().Delete(&store.VideoComment{}, id)
	if result.Error != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "comment not found"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func parseCommentVideoID(path string) (int64, error) {
	rest := strings.TrimPrefix(path, "/api/videos/")
	rest = strings.TrimSuffix(rest, "/comments")
	return strconv.ParseInt(strings.Trim(rest, "/"), 10, 64)
}
