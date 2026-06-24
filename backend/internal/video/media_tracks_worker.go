package video

import (
	"context"
	"log"
	"strings"
	"time"

	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/store"

	"github.com/hibiken/asynq"
)

// videoStatusExtracting marks a video whose source has been uploaded but whose
// audio/subtitle tracks are still being extracted in the background.
const videoStatusExtracting = "extracting"

// staleExtractingAge is how long a video may sit in "extracting" before the
// janitor assumes the worker crashed mid-extraction and releases it to
// "uploaded". Extraction is normally a few minutes even for long sources with
// several tracks, so a generous window avoids racing a slow job.
const staleExtractingAge = 2 * time.Hour

// HandleExtractTracksTask runs background extraction of the audio/subtitle
// tracks for an uploaded source. Extraction is best effort: per-track failures
// are recorded on the track rows, and the video is always released back to
// "uploaded" so it never gets stranded in "extracting".
func HandleExtractTracksTask(ctx context.Context, t *asynq.Task) error {
	p, err := ParseExtractTracksPayload(t)
	if err != nil {
		return err
	}
	runExtractTracks(ctx, p.VideoID, p.SourceKey)
	return nil
}

func runExtractTracks(ctx context.Context, videoID int64, srcKey string) {
	defer releaseExtractingVideo(videoID)

	var video store.Video
	if err := store.DB().First(&video, videoID).Error; err != nil {
		log.Printf("extract tracks: load video %d failed: %v", videoID, err)
		return
	}
	if strings.TrimSpace(srcKey) == "" {
		srcKey = sourceKeyForVideo(video)
	}

	tmpRoot := strings.TrimSpace(config.Load().Worker.TranscodeTempDir)
	srcPath, err := cachedSourcePath(ctx, videoID, srcKey, tmpRoot)
	if err != nil {
		log.Printf("extract tracks: cache source for video %d failed: %v", videoID, err)
		return
	}
	if _, err := ensureVideoMediaTracks(ctx, videoID, srcKey, srcPath); err != nil {
		log.Printf("extract tracks: ensure media tracks for video %d failed: %v", videoID, err)
	}
}

// releaseExtractingVideo advances a video from "extracting" to "uploaded".
// It is a no-op for any other status so it never clobbers a transcode that the
// admin may have started in the meantime.
func releaseExtractingVideo(videoID int64) {
	store.DB().Model(&store.Video{}).
		Where("id = ? AND status = ?", videoID, videoStatusExtracting).
		Updates(map[string]interface{}{"status": "uploaded", "updated_at": time.Now()})
}

// reapStaleExtractingVideos releases videos that have been stuck in "extracting"
// longer than staleExtractingAge, which only happens if the worker died between
// setting the status and finishing extraction.
func reapStaleExtractingVideos(ctx context.Context) {
	cutoff := time.Now().Add(-staleExtractingAge)
	var ids []int64
	if err := store.DB().WithContext(ctx).Model(&store.Video{}).
		Where("status = ? AND updated_at < ?", videoStatusExtracting, cutoff).
		Pluck("id", &ids).Error; err != nil {
		log.Printf("janitor: load stale extracting videos failed: %v", err)
		return
	}
	for _, id := range ids {
		store.DB().Model(&store.Video{}).
			Where("id = ? AND status = ?", id, videoStatusExtracting).
			Updates(map[string]interface{}{"status": "uploaded", "updated_at": time.Now()})
		log.Printf("janitor: released stale extracting video %d", id)
	}
}
