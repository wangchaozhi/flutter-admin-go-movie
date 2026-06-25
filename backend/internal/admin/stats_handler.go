package admin

import (
	"net/http"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

type countByKey struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type revenueByCurrency struct {
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amount_cents"`
}

type topVideo struct {
	VideoID int64  `json:"video_id"`
	Title   string `json:"title"`
	Plays   int64  `json:"plays"`
}

type dashboardStats struct {
	Videos struct {
		Total    int64        `json:"total"`
		VIP      int64        `json:"vip"`
		Free     int64        `json:"free"`
		ByStatus []countByKey `json:"by_status"`
	} `json:"videos"`
	Categories struct {
		Total int64 `json:"total"`
	} `json:"categories"`
	Users struct {
		Total  int64 `json:"total"`
		VIP    int64 `json:"vip"`
		Banned int64 `json:"banned"`
	} `json:"users"`
	Orders struct {
		Total    int64        `json:"total"`
		ByStatus []countByKey `json:"by_status"`
	} `json:"orders"`
	Revenue   []revenueByCurrency `json:"revenue"`
	TopVideos []topVideo          `json:"top_videos"`
}

// StatsHandler returns aggregate counts powering the admin dashboard: video,
// category, user and order tallies, paid revenue by currency, and the most
// watched videos. Read-only; available to any authenticated admin.
func StatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}

	db := store.DB()
	var stats dashboardStats

	db.Model(&store.Video{}).Count(&stats.Videos.Total)
	db.Model(&store.Video{}).Where("is_vip = ? AND is_free = ?", true, false).Count(&stats.Videos.VIP)
	db.Model(&store.Video{}).Where("is_free = ?", true).Count(&stats.Videos.Free)
	db.Model(&store.Video{}).Select("status as key, count(*) as count").Group("status").Scan(&stats.Videos.ByStatus)

	db.Model(&store.Category{}).Count(&stats.Categories.Total)

	now := time.Now()
	db.Model(&store.MobileUser{}).Count(&stats.Users.Total)
	db.Model(&store.MobileUser{}).Where("vip_until IS NOT NULL AND vip_until > ?", now).Count(&stats.Users.VIP)
	db.Model(&store.MobileUser{}).Where("status = ?", "banned").Count(&stats.Users.Banned)

	db.Model(&store.Order{}).Count(&stats.Orders.Total)
	db.Model(&store.Order{}).Select("status as key, count(*) as count").Group("status").Scan(&stats.Orders.ByStatus)

	db.Model(&store.Order{}).
		Select("currency, sum(amount_cents) as amount_cents").
		Where("status = ?", "paid").
		Group("currency").
		Order("amount_cents desc").
		Scan(&stats.Revenue)

	stats.TopVideos = topWatchedVideos(5)

	if stats.Revenue == nil {
		stats.Revenue = []revenueByCurrency{}
	}
	if stats.TopVideos == nil {
		stats.TopVideos = []topVideo{}
	}

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: stats})
}

// topWatchedVideos returns the videos with the most play records, resolving each
// title in a single follow-up query.
func topWatchedVideos(limit int) []topVideo {
	var rows []topVideo
	store.DB().Model(&store.VideoPlayRecord{}).
		Select("video_id, count(*) as plays").
		Group("video_id").
		Order("plays desc").
		Limit(limit).
		Scan(&rows)
	if len(rows) == 0 {
		return rows
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.VideoID)
	}
	titles := map[int64]string{}
	var videos []store.Video
	store.DB().Select("id, title").Where("id IN ?", ids).Find(&videos)
	for _, v := range videos {
		titles[v.ID] = v.Title
	}
	for i := range rows {
		rows[i].Title = titles[rows[i].VideoID]
	}
	return rows
}
