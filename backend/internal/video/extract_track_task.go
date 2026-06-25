package video

import (
	"context"
	"errors"
	"log"
	"time"

	"flutter-admin-go/internal/store"
)

// Bookkeeping writes for a finished run must complete even when the run context
// was canceled mid-extraction, so they use the default DB context rather than
// the (possibly canceled) run context.

// errExtractTracksCanceled marks an extraction run that was abandoned because
// the video's source changed (re-upload/delete) while it was in flight. It maps
// to the "canceled" history status rather than "failed".
var errExtractTracksCanceled = errors.New("extract tracks canceled")

// Extraction history statuses.
const (
	extractStatusProcessing = "processing"
	extractStatusSuccess    = "success"
	extractStatusFailed     = "failed"
	extractStatusCanceled   = "canceled"
)

func isActiveExtractStatus(status string) bool {
	return status == extractStatusProcessing
}

// beginExtractTrackTask records the start of a background extraction run and
// returns the new history row id. A zero id means recording failed and the
// caller should simply skip the matching finish call.
func beginExtractTrackTask(videoID int64, srcKey string) int64 {
	now := time.Now()
	task := store.VideoExtractTrackTask{
		VideoID:   videoID,
		SourceKey: srcKey,
		Status:    extractStatusProcessing,
		StartedAt: &now,
		CreatedAt: now,
	}
	if err := store.DB().Create(&task).Error; err != nil {
		log.Printf("extract tracks: record history start for video %d failed: %v", videoID, err)
		return 0
	}
	return task.ID
}

// finishExtractTrackTask closes out a history row with its final status and the
// detected/extracted track counts. runErr nil means success; a cancellation
// sentinel or context cancellation maps to "canceled"; anything else "failed".
func finishExtractTrackTask(taskID, videoID int64, srcKey string, runErr error) {
	if taskID == 0 {
		return
	}

	status := extractStatusSuccess
	errMessage := ""
	switch {
	case runErr == nil:
		// success
	case errors.Is(runErr, errExtractTracksCanceled) || errors.Is(runErr, context.Canceled):
		status = extractStatusCanceled
	default:
		status = extractStatusFailed
		errMessage = runErr.Error()
	}

	counts := extractTrackCounts(videoID, srcKey)
	now := time.Now()
	updates := map[string]interface{}{
		"status":         status,
		"status_message": extractStatusMessage(status, counts),
		"audio_count":    counts.Audio,
		"subtitle_count": counts.Subtitle,
		"ready_count":    counts.Ready,
		"failed_count":   counts.Failed,
		"error_message":  errMessage,
		"finished_at":    now,
	}
	if err := store.DB().Model(&store.VideoExtractTrackTask{}).
		Where("id = ?", taskID).
		Updates(updates).Error; err != nil {
		log.Printf("extract tracks: record history finish for task %d failed: %v", taskID, err)
	}
}

type extractCounts struct {
	Audio    int
	Subtitle int
	Ready    int
	Failed   int
}

// extractTrackCounts tallies the media-track rows produced for this source so
// the history row reflects what was actually detected and extracted, including
// partial failures.
func extractTrackCounts(videoID int64, srcKey string) extractCounts {
	var counts extractCounts
	if err := store.DB().
		Model(&store.VideoMediaTrack{}).
		Where("video_id = ? AND source_key = ? AND track_type <> ?", videoID, srcKey, "source").
		Select(`
			COUNT(*) FILTER (WHERE track_type = 'audio') AS audio,
			COUNT(*) FILTER (WHERE track_type = 'subtitle') AS subtitle,
			COUNT(*) FILTER (WHERE status = 'ready') AS ready,
			COUNT(*) FILTER (WHERE status IN ('failed', 'unsupported')) AS failed`).
		Scan(&counts).Error; err != nil {
		log.Printf("extract tracks: count tracks for video %d failed: %v", videoID, err)
	}
	return counts
}

func extractStatusMessage(status string, counts extractCounts) string {
	switch status {
	case extractStatusSuccess:
		if counts.Failed > 0 {
			return "提取完成，部分轨道失败"
		}
		return "提取完成"
	case extractStatusCanceled:
		return "已取消"
	case extractStatusFailed:
		return "提取失败"
	default:
		return ""
	}
}
