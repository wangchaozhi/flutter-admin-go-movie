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
	maxCommentLength = 1000
	commentPageSize  = 20
	// maxRepliesPerReview caps how many replies are returned inline per top-level
	// review so a hugely-discussed review can't bloat one page. reply_count still
	// reports the true total.
	maxRepliesPerReview = 50
)

// commentLimiter throttles comment/rating submissions per user to curb spam.
var commentLimiter = common.NewLimiter("comment_post", 10, time.Minute)

// AppCommentItem is a comment enriched with the author's display name so the app
// does not have to resolve users separately. For a top-level review, Replies and
// ReplyCount carry its thread; for a reply, ReplyToNickname names the @user.
type AppCommentItem struct {
	store.VideoComment
	Nickname        string           `json:"nickname"`
	Username        string           `json:"username"`
	ReplyToNickname string           `json:"reply_to_nickname,omitempty"`
	Replies         []AppCommentItem `json:"replies,omitempty"`
	ReplyCount      int              `json:"reply_count"`
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

	// Top-level reviews drive pagination; replies hang off them.
	var total int64
	store.DB().Model(&store.VideoComment{}).
		Where("video_id = ? AND parent_id IS NULL", videoID).Count(&total)

	var summary commentSummary
	store.DB().Model(&store.VideoComment{}).
		Select("count(*) as count, count(*) filter (where rating > 0) as rating_count, coalesce(avg(rating) filter (where rating > 0), 0) as average_rating").
		Where("video_id = ? AND parent_id IS NULL", videoID).
		Scan(&summary)

	var reviews []store.VideoComment
	store.DB().Where("video_id = ? AND parent_id IS NULL", videoID).
		Order("id desc").
		Offset((page - 1) * commentPageSize).
		Limit(commentPageSize).
		Find(&reviews)

	// Fetch all replies for the reviews on this page in one query.
	reviewIDs := make([]int64, len(reviews))
	for i, c := range reviews {
		reviewIDs[i] = c.ID
	}
	var replies []store.VideoComment
	if len(reviewIDs) > 0 {
		store.DB().Where("parent_id IN ?", reviewIDs).
			Order("id asc").Find(&replies)
	}

	// Flag which comments the caller has already liked (authenticated only).
	all := append(append([]store.VideoComment{}, reviews...), replies...)
	markLikedByUser(r, all)

	// Resolve display names for authors and @reply targets in one pass.
	nameByID := resolveCommentNames(all)
	decorate := func(c store.VideoComment) AppCommentItem {
		item := AppCommentItem{VideoComment: c, Nickname: nameByID[c.UserID].Nickname, Username: nameByID[c.UserID].Username}
		if c.ReplyToUserID != nil {
			item.ReplyToNickname = displayName(nameByID[*c.ReplyToUserID])
		}
		return item
	}

	repliesByRoot := map[int64][]AppCommentItem{}
	for _, rep := range replies {
		if rep.ParentID == nil {
			continue
		}
		root := *rep.ParentID
		if len(repliesByRoot[root]) >= maxRepliesPerReview {
			continue
		}
		repliesByRoot[root] = append(repliesByRoot[root], decorate(rep))
	}

	items := make([]AppCommentItem, len(reviews))
	for i, rev := range reviews {
		item := decorate(rev)
		item.Replies = repliesByRoot[rev.ID]
		item.ReplyCount = len(repliesByRoot[rev.ID])
		items[i] = item
	}

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
	if allowed, retry := commentLimiter.Allow(strconv.Itoa(userID)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		common.WriteJSON(w, http.StatusTooManyRequests, common.APIResponse{Code: 429, Msg: "评论太频繁，请稍后再试"})
		return
	}
	var req struct {
		Content       string `json:"content"`
		Rating        int    `json:"rating"`
		ParentID      int64  `json:"parent_id"`
		ReplyToUserID int64  `json:"reply_to_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if len([]rune(req.Content)) > maxCommentLength {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "comment too long"})
		return
	}

	// Guard against commenting on a non-existent video.
	if err := store.DB().Select("id").First(&store.Video{}, videoID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
		return
	}

	if req.ParentID > 0 {
		createReply(w, videoID, int64(userID), req.ParentID, req.ReplyToUserID, req.Content)
		return
	}

	// Top-level review: rating must be valid and content or rating is required.
	if req.Rating < 0 || req.Rating > 5 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "rating must be between 0 and 5"})
		return
	}
	if req.Content == "" && req.Rating == 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "comment or rating required"})
		return
	}

	now := time.Now()
	var comment store.VideoComment
	// A user may post only ONE top-level review per video, but can edit it.
	// Upsert onto the partial unique index (video_id, user_id) WHERE parent_id IS
	// NULL so re-posting updates the existing review instead of stacking rows.
	if err := store.DB().Raw(`
		INSERT INTO video_comments (video_id, user_id, content, rating, parent_id, like_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, 0, ?, ?)
		ON CONFLICT (video_id, user_id) WHERE parent_id IS NULL
		DO UPDATE SET content = EXCLUDED.content, rating = EXCLUDED.rating, updated_at = EXCLUDED.updated_at
		RETURNING id, video_id, user_id, content, rating, parent_id, reply_to_user_id, like_count, created_at, updated_at
	`, videoID, userID, req.Content, req.Rating, now, now).Scan(&comment).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: decorateComments([]store.VideoComment{comment})[0]})
}

// createReply inserts a reply under a review and notifies the user being
// replied to. Threads are kept flat (two-level): replying to a reply attaches
// to the same root review and records the replied-to author as @user.
func createReply(w http.ResponseWriter, videoID, userID, parentID, replyToUserID int64, content string) {
	if content == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "回复内容不能为空"})
		return
	}
	var parent store.VideoComment
	if err := store.DB().First(&parent, parentID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "评论不存在"})
		return
	}
	if parent.VideoID != videoID {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "评论与影片不匹配"})
		return
	}

	// Flatten to two levels: the root is the parent itself if it's a review,
	// otherwise the parent's root. The notification target is the author we are
	// directly replying to.
	rootID := parent.ID
	target := parent.UserID
	if parent.ParentID != nil {
		rootID = *parent.ParentID
		target = parent.UserID
	}
	if replyToUserID > 0 {
		target = replyToUserID
	}

	now := time.Now()
	reply := store.VideoComment{
		VideoID:       videoID,
		UserID:        userID,
		Content:       content,
		Rating:        0,
		ParentID:      &rootID,
		ReplyToUserID: &target,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := store.DB().Create(&reply).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	notifyUser(target, userID, "reply", videoID, &reply.ID, &rootID, content)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: decorateComments([]store.VideoComment{reply})[0]})
}

// AppCommentByIDHandler handles comment-scoped actions (mobile auth):
//
//	PUT    /api/mobile/comments/{id}        — edit the caller's own comment
//	DELETE /api/mobile/comments/{id}        — delete the caller's own comment
//	POST   /api/mobile/comments/{id}/like   — toggle a like on any comment
func AppCommentByIDHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/mobile/comments/")
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
		toggleCommentLike(w, r, id, int64(userID))
		return
	}

	switch r.Method {
	case http.MethodPut:
		editComment(w, r, id, int64(userID))
	case http.MethodDelete:
		deleteOwnComment(w, id, int64(userID))
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

// editComment updates the content of the caller's own comment, plus the rating
// when it is a top-level review.
func editComment(w http.ResponseWriter, r *http.Request, id, userID int64) {
	var req struct {
		Content string `json:"content"`
		Rating  int    `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if len([]rune(req.Content)) > maxCommentLength {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "comment too long"})
		return
	}
	if req.Rating < 0 || req.Rating > 5 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "rating must be between 0 and 5"})
		return
	}

	var comment store.VideoComment
	if err := store.DB().Where("id = ? AND user_id = ?", id, userID).First(&comment).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "评论不存在或无权修改"})
		return
	}
	updates := map[string]interface{}{"content": req.Content, "updated_at": time.Now()}
	if comment.ParentID == nil {
		// Only top-level reviews carry a rating; replies must have content.
		if req.Content == "" && req.Rating == 0 {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "comment or rating required"})
			return
		}
		updates["rating"] = req.Rating
	} else if req.Content == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "回复内容不能为空"})
		return
	}
	if err := store.DB().Model(&store.VideoComment{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	store.DB().First(&comment, id)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: decorateComments([]store.VideoComment{comment})[0]})
}

// deleteOwnComment removes the caller's own comment and cleans up its likes and,
// for a review, its replies.
func deleteOwnComment(w http.ResponseWriter, id, userID int64) {
	result := store.DB().Where("id = ? AND user_id = ?", id, userID).Delete(&store.VideoComment{})
	if result.Error != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "comment not found"})
		return
	}
	// Best-effort cleanup of the deleted comment's likes and any replies under it.
	store.DB().Where("comment_id = ?", id).Delete(&store.CommentLike{})
	store.DB().Where("parent_id = ?", id).Delete(&store.VideoComment{})
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// toggleCommentLike likes the comment if the user hasn't liked it yet, otherwise
// unlikes it. The denormalised like_count is kept in sync inside the same
// transaction. A like (not an unlike) notifies the comment author. Mirrors
// toggleDanmakuLike. Returns the new {liked, like_count}.
func toggleCommentLike(w http.ResponseWriter, r *http.Request, commentID, userID int64) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var liked bool
	var likeCount int
	var comment store.VideoComment
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&comment, commentID).Error; err != nil {
			return err
		}
		like := store.CommentLike{CommentID: commentID, UserID: userID}
		res := tx.Where("comment_id = ? AND user_id = ?", commentID, userID).Delete(&store.CommentLike{})
		if res.Error != nil {
			return res.Error
		}
		delta := 1
		if res.RowsAffected > 0 {
			liked = false
			delta = -1
		} else {
			if err := tx.Create(&like).Error; err != nil {
				return err
			}
			liked = true
		}
		if err := tx.Model(&store.VideoComment{}).Where("id = ?", commentID).
			Update("like_count", gorm.Expr("GREATEST(like_count + ?, 0)", delta)).Error; err != nil {
			return err
		}
		var refreshed store.VideoComment
		if err := tx.Select("like_count").First(&refreshed, commentID).Error; err != nil {
			return err
		}
		likeCount = refreshed.LikeCount
		return nil
	})
	if err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "评论不存在"})
		return
	}
	if liked {
		root := comment.ParentID
		if root == nil {
			root = &comment.ID
		}
		notifyUser(comment.UserID, userID, "like", comment.VideoID, &comment.ID, root, comment.Content)
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"liked":      liked,
		"like_count": likeCount,
	}})
}

// notifyUser records an in-app notification, best-effort. Self-actions and
// missing recipients are skipped, and a failed insert never blocks the action
// that triggered it.
func notifyUser(recipient, actor int64, typ string, videoID int64, commentID, rootID *int64, snippet string) {
	if recipient == 0 || recipient == actor {
		return
	}
	if len([]rune(snippet)) > 140 {
		snippet = string([]rune(snippet)[:140])
	}
	store.DB().Create(&store.UserNotification{
		UserID:        recipient,
		ActorID:       actor,
		Type:          typ,
		VideoID:       videoID,
		CommentID:     commentID,
		RootCommentID: rootID,
		Content:       snippet,
		CreatedAt:     time.Now(),
	})
}

type commentUser struct {
	ID       int64
	Username string
	Nickname string
}

func displayName(u commentUser) string {
	if strings.TrimSpace(u.Nickname) != "" {
		return strings.TrimSpace(u.Nickname)
	}
	if strings.TrimSpace(u.Username) != "" {
		return strings.TrimSpace(u.Username)
	}
	return "用户"
}

// resolveCommentNames batch-loads display names for every author and @reply
// target referenced by the given comments in one query.
func resolveCommentNames(comments []store.VideoComment) map[int64]commentUser {
	idSet := map[int64]struct{}{}
	for _, c := range comments {
		idSet[c.UserID] = struct{}{}
		if c.ReplyToUserID != nil {
			idSet[*c.ReplyToUserID] = struct{}{}
		}
	}
	out := make(map[int64]commentUser, len(idSet))
	if len(idSet) == 0 {
		return out
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var users []commentUser
	store.DB().Model(&store.MobileUser{}).Select("id, username, nickname").Where("id IN ?", ids).Scan(&users)
	for _, u := range users {
		out[u.ID] = u
	}
	return out
}

// markLikedByUser flags, in place, which of the comments the requesting user has
// already liked. No-op for anonymous requests.
func markLikedByUser(r *http.Request, comments []store.VideoComment) {
	userID, ok := parseMobileAuth(r)
	if !ok || len(comments) == 0 {
		return
	}
	ids := make([]int64, len(comments))
	for i, c := range comments {
		ids[i] = c.ID
	}
	var likedIDs []int64
	store.DB().Model(&store.CommentLike{}).
		Where("user_id = ? AND comment_id IN ?", userID, ids).
		Pluck("comment_id", &likedIDs)
	liked := make(map[int64]struct{}, len(likedIDs))
	for _, id := range likedIDs {
		liked[id] = struct{}{}
	}
	for i := range comments {
		if _, ok := liked[comments[i].ID]; ok {
			comments[i].Liked = true
		}
	}
}

// decorateComments resolves author display names for a batch of comments in one
// query to avoid N+1 lookups.
func decorateComments(comments []store.VideoComment) []AppCommentItem {
	items := make([]AppCommentItem, len(comments))
	if len(comments) == 0 {
		return items
	}
	nameByID := resolveCommentNames(comments)
	for i, c := range comments {
		u := nameByID[c.UserID]
		item := AppCommentItem{VideoComment: c, Nickname: u.Nickname, Username: u.Username}
		if c.ReplyToUserID != nil {
			item.ReplyToNickname = displayName(nameByID[*c.ReplyToUserID])
		}
		items[i] = item
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
	// Best-effort cleanup of the deleted comment's likes and replies.
	store.DB().Where("comment_id = ?", id).Delete(&store.CommentLike{})
	store.DB().Where("parent_id = ?", id).Delete(&store.VideoComment{})
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func parseCommentVideoID(path string) (int64, error) {
	rest := strings.TrimPrefix(path, "/api/videos/")
	rest = strings.TrimSuffix(rest, "/comments")
	return strconv.ParseInt(strings.Trim(rest, "/"), 10, 64)
}
