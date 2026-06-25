package video

import (
	"net/http"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"

	"gorm.io/gorm"
)

const homeRailLimit = 12

// AppHomeHandler returns aggregated rails for the app landing page: most played
// (popular), newest, and VIP picks. Public — no auth required.
//
// GET /api/home
func AppHomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}

	latest := readyVideoItems(store.DB().
		Where("status = ?", "ready").
		Order("id desc").
		Limit(homeRailLimit))

	vip := readyVideoItems(store.DB().
		Where("status = ? AND is_vip = ? AND is_free = ?", "ready", true, false).
		Order("id desc").
		Limit(homeRailLimit))

	popular := popularVideoItems(homeRailLimit)

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"popular": popular,
		"latest":  latest,
		"vip":     vip,
	}})
}

// readyVideoItems runs the given (already-scoped) query and decorates the
// resulting videos with cover URL and category name.
func readyVideoItems(query *gorm.DB) []AppVideoItem {
	var videos []store.Video
	query.Find(&videos)
	return buildAppVideoItems(videos)
}

// popularVideoItems ranks videos by play-record count, keeping only those still
// "ready" and preserving the popularity order.
func popularVideoItems(limit int) []AppVideoItem {
	type row struct {
		VideoID int64
	}
	var rows []row
	store.DB().Model(&store.VideoPlayRecord{}).
		Select("video_id").
		Group("video_id").
		Order("count(*) desc").
		Limit(limit).
		Scan(&rows)
	if len(rows) == 0 {
		return []AppVideoItem{}
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.VideoID)
	}
	var videos []store.Video
	store.DB().Where("id IN ? AND status = ?", ids, "ready").Find(&videos)
	itemByID := make(map[int64]AppVideoItem, len(videos))
	for _, item := range buildAppVideoItems(videos) {
		itemByID[item.Video.ID] = item
	}
	ordered := make([]AppVideoItem, 0, len(rows))
	for _, r := range rows {
		if item, ok := itemByID[r.VideoID]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

// buildAppVideoItems decorates videos with cover URL and category name,
// resolving only the categories referenced by this slice (not the whole table)
// and preserving input order. Always returns a non-nil slice.
func buildAppVideoItems(videos []store.Video) []AppVideoItem {
	items := make([]AppVideoItem, 0, len(videos))
	if len(videos) == 0 {
		return items
	}
	catIDSet := map[int]struct{}{}
	for _, v := range videos {
		if v.CategoryID > 0 {
			catIDSet[v.CategoryID] = struct{}{}
		}
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
	for _, v := range videos {
		items = append(items, AppVideoItem{
			Video:        v,
			CoverURL:     coverURL(v),
			CategoryName: catNames[v.CategoryID],
		})
	}
	return items
}
