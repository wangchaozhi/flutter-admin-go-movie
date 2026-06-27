package video

import (
	"net/http"
	"strconv"
	"strings"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

const notificationPageSize = 20

// AppNotificationItem is a notification enriched with the actor's display name
// and the video title so the app can render it without extra lookups.
type AppNotificationItem struct {
	store.UserNotification
	ActorNickname string `json:"actor_nickname"`
	ActorUsername string `json:"actor_username"`
	VideoTitle    string `json:"video_title"`
}

// AppNotificationsHandler serves the caller's notification feed.
//
//	GET /api/mobile/notifications?page=  — paginated, newest first
func AppNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	var total, unread int64
	store.DB().Model(&store.UserNotification{}).Where("user_id = ?", userID).Count(&total)
	store.DB().Model(&store.UserNotification{}).Where("user_id = ? AND is_read = FALSE", userID).Count(&unread)

	var notes []store.UserNotification
	store.DB().Where("user_id = ?", userID).
		Order("id desc").
		Offset((page - 1) * notificationPageSize).
		Limit(notificationPageSize).
		Find(&notes)

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"items":    decorateNotifications(notes),
		"total":    total,
		"unread":   unread,
		"page":     page,
		"per_page": notificationPageSize,
	}})
}

// AppNotificationByPathHandler serves notification sub-actions:
//
//	GET  /api/mobile/notifications/unread-count — count of unread notifications
//	POST /api/mobile/notifications/read         — mark all as read
//	POST /api/mobile/notifications/{id}/read    — mark one as read
func AppNotificationByPathHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/mobile/notifications/"), "/")

	if rest == "unread-count" {
		if r.Method != http.MethodGet {
			common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
			return
		}
		var unread int64
		store.DB().Model(&store.UserNotification{}).Where("user_id = ? AND is_read = FALSE", userID).Count(&unread)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{"unread": unread}})
		return
	}

	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}

	q := store.DB().Model(&store.UserNotification{}).Where("user_id = ?", userID)
	if rest != "read" {
		// /{id}/read — mark a single notification read.
		id, err := strconv.ParseInt(strings.TrimSuffix(rest, "/read"), 10, 64)
		if err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
			return
		}
		q = q.Where("id = ?", id)
	}
	if err := q.Update("is_read", true).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// decorateNotifications resolves actor names and video titles for a batch in two
// queries to avoid N+1 lookups.
func decorateNotifications(notes []store.UserNotification) []AppNotificationItem {
	items := make([]AppNotificationItem, len(notes))
	if len(notes) == 0 {
		return items
	}
	actorSet := map[int64]struct{}{}
	videoSet := map[int64]struct{}{}
	for _, n := range notes {
		actorSet[n.ActorID] = struct{}{}
		videoSet[n.VideoID] = struct{}{}
	}
	actorIDs := make([]int64, 0, len(actorSet))
	for id := range actorSet {
		actorIDs = append(actorIDs, id)
	}
	videoIDs := make([]int64, 0, len(videoSet))
	for id := range videoSet {
		videoIDs = append(videoIDs, id)
	}

	var users []commentUser
	store.DB().Model(&store.MobileUser{}).Select("id, username, nickname").Where("id IN ?", actorIDs).Scan(&users)
	userByID := make(map[int64]commentUser, len(users))
	for _, u := range users {
		userByID[u.ID] = u
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

	for i, n := range notes {
		u := userByID[n.ActorID]
		items[i] = AppNotificationItem{
			UserNotification: n,
			ActorNickname:    u.Nickname,
			ActorUsername:    u.Username,
			VideoTitle:       titleByID[n.VideoID],
		}
	}
	return items
}
