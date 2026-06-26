package video

import (
	"net/http"
	"time"

	"flutter-admin-go/internal/cache"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"

	"gorm.io/gorm"
)

const (
	homeRailLimit = 12
	// homeCacheKey/TTL cache the public rails (identical for every visitor) for
	// a short window so the home page does not re-aggregate play counts on every
	// request. The per-user "continue" rail is never cached.
	homeCacheKey = "cache:home:rails:v1"
	homeCacheTTL = 30 * time.Second
)

// homeRails is the public, visitor-agnostic portion of the landing page.
type homeRails struct {
	Popular []AppVideoItem `json:"popular"`
	Latest  []AppVideoItem `json:"latest"`
	Vip     []AppVideoItem `json:"vip"`
}

// AppHomeHandler returns aggregated rails for the app landing page: most played
// (popular), newest, VIP picks, and — when the caller is authenticated — a
// personalised "continue watching" rail. Public rails are cached briefly; the
// continue rail is computed per request. Public — auth is optional.
//
// GET /api/home
func AppHomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}

	ctx := r.Context()
	var rails homeRails
	if !cache.GetJSON(ctx, homeCacheKey, &rails) {
		rails = homeRails{
			Latest: readyVideoItems(store.DB().
				Where("status = ?", "ready").
				Order("id desc").
				Limit(homeRailLimit)),
			Vip: readyVideoItems(store.DB().
				Where("status = ? AND is_vip = ? AND is_free = ?", "ready", true, false).
				Order("id desc").
				Limit(homeRailLimit)),
			Popular: popularVideoItems(homeRailLimit),
		}
		cache.SetJSON(ctx, homeCacheKey, rails, homeCacheTTL)
	}

	continueRail := []AppWatchHistoryItem{}
	if userID, ok := parseMobileAuth(r); ok {
		continueRail = continueWatchingItems(int64(userID), homeRailLimit)
	}

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"popular":  rails.Popular,
		"latest":   rails.Latest,
		"vip":      rails.Vip,
		"continue": continueRail,
	}})
}

// continueWatchingItems returns the user's recently-watched, not-yet-finished
// videos (newest first), so the app can offer one-tap resume. Finished plays
// (≥95% watched) are excluded, as are videos no longer "ready".
func continueWatchingItems(userID int64, limit int) []AppWatchHistoryItem {
	var records []store.VideoPlayRecord
	store.DB().
		Where("user_id = ? AND duration > 0 AND position > 0 AND position * 100 < duration * 95", userID).
		Order("updated_at desc").
		Limit(limit).
		Find(&records)
	if len(records) == 0 {
		return []AppWatchHistoryItem{}
	}
	videoIDs := make([]int64, 0, len(records))
	for _, rec := range records {
		videoIDs = append(videoIDs, rec.VideoID)
	}
	videoItems := appVideoItemsByID(videoIDs)
	items := make([]AppWatchHistoryItem, 0, len(records))
	for _, rec := range records {
		item, ok := videoItems[rec.VideoID]
		if !ok || item.Status != "ready" {
			continue
		}
		progress := rec.Position * 100 / rec.Duration
		if progress > 100 {
			progress = 100
		}
		items = append(items, AppWatchHistoryItem{
			AppVideoItem: item,
			Position:     rec.Position,
			Progress:     progress,
			UpdatedAt:    rec.UpdatedAt,
		})
	}
	return items
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
