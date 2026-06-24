package video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/store"

	"github.com/minio/minio-go/v7"
)

type ffprobeStreamInfo struct {
	Index       int               `json:"index"`
	CodecName   string            `json:"codec_name"`
	CodecType   string            `json:"codec_type"`
	Tags        map[string]string `json:"tags"`
	Disposition map[string]int    `json:"disposition"`
}

type ffprobeStreamsResult struct {
	Streams []ffprobeStreamInfo `json:"streams"`
}

func ensureVideoMediaTracks(ctx context.Context, videoID int64, srcKey, srcPath string) ([]store.VideoMediaTrack, error) {
	info, err := store.ObjectClient().StatObject(ctx, store.VideoBucket(), srcKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("stat source for media tracks: %w", err)
	}
	return ensureVideoMediaTracksForSource(ctx, videoID, srcKey, srcPath, info)
}

func ensureVideoMediaTracksForSource(ctx context.Context, videoID int64, srcKey, srcPath string, info minio.ObjectInfo) ([]store.VideoMediaTrack, error) {
	sourceETag := mediaSourceETag(info)
	var existing []store.VideoMediaTrack
	if err := store.DB().
		Where("video_id = ? AND source_key = ? AND source_etag = ? AND source_size = ?", videoID, srcKey, sourceETag, info.Size).
		Order("track_type asc, stream_position asc").
		Find(&existing).Error; err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return readyMediaTracks(existing), nil
	}

	if err := store.DB().Where("video_id = ?", videoID).Delete(&store.VideoMediaTrack{}).Error; err != nil {
		return nil, err
	}
	removeObjectsByPrefix(ctx, fmt.Sprintf("hls/%d/tracks/", videoID))
	if err := store.DB().Create(&store.VideoMediaTrack{
		VideoID:        videoID,
		SourceKey:      srcKey,
		SourceETag:     sourceETag,
		SourceSize:     info.Size,
		TrackType:      "source",
		StreamIndex:    -1,
		StreamPosition: 0,
		Status:         "ready",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	streams, err := probeMediaStreams(srcPath)
	if err != nil {
		return nil, err
	}

	tmpRoot := strings.TrimSpace(config.Load().Worker.TranscodeTempDir)
	if tmpRoot != "" {
		if err := os.MkdirAll(tmpRoot, 0755); err != nil {
			return nil, fmt.Errorf("create media track temp root: %w", err)
		}
	}
	tmpDir, err := os.MkdirTemp(tmpRoot, fmt.Sprintf("tracks_%d_", videoID))
	if err != nil {
		return nil, fmt.Errorf("create media track temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	sourceVersion := mediaSourceVersion(info)
	audioPosition := 0
	subtitlePosition := 0
	created := make([]store.VideoMediaTrack, 0)
	for _, stream := range streams {
		switch stream.CodecType {
		case "audio":
			track := newMediaTrack(videoID, srcKey, sourceETag, info.Size, "audio", stream, audioPosition)
			audioPosition++
			track.ObjectKey = fmt.Sprintf("hls/%d/tracks/%s/audio/a%d/index.m3u8", videoID, sourceVersion, track.StreamPosition)
			track.Status = "processing"
			if err := store.DB().Create(&track).Error; err != nil {
				return nil, err
			}
			if err := extractAndUploadAudioTrack(ctx, srcPath, tmpDir, track); err != nil {
				markMediaTrackFailed(track.ID, err)
				log.Printf("audio track extraction failed: video_id=%d stream=%d: %v", videoID, track.StreamIndex, err)
			} else {
				markMediaTrackReady(track.ID)
				track.Status = "ready"
				created = append(created, track)
			}
		case "subtitle":
			track := newMediaTrack(videoID, srcKey, sourceETag, info.Size, "subtitle", stream, subtitlePosition)
			subtitlePosition++
			if !isTextSubtitleCodec(track.CodecName) {
				track.Status = "unsupported"
				track.ErrorMessage = "unsupported subtitle codec: " + track.CodecName
				if err := store.DB().Create(&track).Error; err != nil {
					return nil, err
				}
				continue
			}
			track.ObjectKey = fmt.Sprintf("hls/%d/tracks/%s/subtitles/s%d.vtt", videoID, sourceVersion, track.StreamPosition)
			track.Status = "processing"
			if err := store.DB().Create(&track).Error; err != nil {
				return nil, err
			}
			if err := extractAndUploadSubtitleTrack(ctx, srcPath, tmpDir, track); err != nil {
				markMediaTrackFailed(track.ID, err)
				log.Printf("subtitle track extraction failed: video_id=%d stream=%d: %v", videoID, track.StreamIndex, err)
			} else {
				markMediaTrackReady(track.ID)
				track.Status = "ready"
				created = append(created, track)
			}
		}
	}
	sortMediaTracks(created)
	return created, nil
}

func probeMediaStreams(srcPath string) ([]ffprobeStreamInfo, error) {
	out, err := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "stream=index,codec_name,codec_type:stream_tags=language,title:stream_disposition=default,forced",
		"-of", "json",
		srcPath,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe media streams: %w", err)
	}
	var result ffprobeStreamsResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse ffprobe media streams: %w", err)
	}
	return result.Streams, nil
}

func newMediaTrack(videoID int64, srcKey, sourceETag string, sourceSize int64, trackType string, stream ffprobeStreamInfo, position int) store.VideoMediaTrack {
	now := time.Now()
	tags := stream.Tags
	if tags == nil {
		tags = map[string]string{}
	}
	disposition := stream.Disposition
	if disposition == nil {
		disposition = map[string]int{}
	}
	return store.VideoMediaTrack{
		VideoID:        videoID,
		SourceKey:      srcKey,
		SourceETag:     sourceETag,
		SourceSize:     sourceSize,
		TrackType:      trackType,
		StreamIndex:    stream.Index,
		StreamPosition: position,
		CodecName:      strings.ToLower(strings.TrimSpace(stream.CodecName)),
		Language:       strings.ToLower(strings.TrimSpace(tags["language"])),
		Title:          strings.TrimSpace(tags["title"]),
		IsDefault:      disposition["default"] == 1,
		IsForced:       disposition["forced"] == 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func extractAndUploadAudioTrack(ctx context.Context, srcPath, tmpRoot string, track store.VideoMediaTrack) error {
	outDir := filepath.Join(tmpRoot, fmt.Sprintf("audio_%d", track.StreamPosition))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	segPattern := filepath.Join(outDir, "seg_%03d.ts")
	indexPath := filepath.Join(outDir, "index.m3u8")
	args := []string{
		"-y",
		"-i", srcPath,
		"-map", fmt.Sprintf("0:%d", track.StreamIndex),
		"-vn",
		"-sn",
		"-c:a", "aac",
		"-ac", "2",
		"-ar", "48000",
		"-b:a", "128k",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", segPattern,
		indexPath,
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg audio stream %d: %w\n%s", track.StreamIndex, err, stderr.String())
	}
	return uploadDir(ctx, outDir, strings.TrimSuffix(track.ObjectKey, "/index.m3u8"))
}

func extractAndUploadSubtitleTrack(ctx context.Context, srcPath, tmpRoot string, track store.VideoMediaTrack) error {
	outPath := filepath.Join(tmpRoot, fmt.Sprintf("subtitle_%d.vtt", track.StreamPosition))
	args := []string{
		"-y",
		"-i", srcPath,
		"-map", fmt.Sprintf("0:%d", track.StreamIndex),
		"-c:s", "webvtt",
		"-f", "webvtt",
		outPath,
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg subtitle stream %d: %w\n%s", track.StreamIndex, err, stderr.String())
	}
	info, err := os.Stat(outPath)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("empty subtitle output")
	}
	return putFile(ctx, track.ObjectKey, outPath, "text/vtt")
}

func putFile(ctx context.Context, key, filePath, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	_, err = store.ObjectClient().PutObject(ctx, store.VideoBucket(), key, f, info.Size(), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func markMediaTrackReady(id int64) {
	store.DB().Model(&store.VideoMediaTrack{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        "ready",
		"error_message": "",
		"updated_at":    time.Now(),
	})
}

func markMediaTrackFailed(id int64, err error) {
	store.DB().Model(&store.VideoMediaTrack{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": err.Error(),
		"updated_at":    time.Now(),
	})
}

func readyMediaTracks(tracks []store.VideoMediaTrack) []store.VideoMediaTrack {
	ready := make([]store.VideoMediaTrack, 0, len(tracks))
	for _, track := range tracks {
		if track.Status == "ready" && strings.TrimSpace(track.ObjectKey) != "" {
			ready = append(ready, track)
		}
	}
	sortMediaTracks(ready)
	return ready
}

func sortMediaTracks(tracks []store.VideoMediaTrack) {
	sort.SliceStable(tracks, func(i, j int) bool {
		if tracks[i].TrackType == tracks[j].TrackType {
			return tracks[i].StreamPosition < tracks[j].StreamPosition
		}
		return tracks[i].TrackType < tracks[j].TrackType
	})
}

func isTextSubtitleCodec(codec string) bool {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "subrip", "ass", "ssa", "webvtt", "mov_text", "text":
		return true
	default:
		return false
	}
}

func mediaSourceETag(info minio.ObjectInfo) string {
	if etag := strings.Trim(strings.TrimSpace(info.ETag), `"`); etag != "" {
		return etag
	}
	return mediaSourceVersion(info)
}

func mediaSourceVersion(info minio.ObjectInfo) string {
	value := mediaSourceETagValue(info)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	version := strings.Trim(b.String(), "._-")
	if version == "" {
		return "source"
	}
	return version
}

func mediaSourceETagValue(info minio.ObjectInfo) string {
	if etag := strings.Trim(strings.TrimSpace(info.ETag), `"`); etag != "" {
		return etag
	}
	if !info.LastModified.IsZero() {
		return fmt.Sprintf("%d_%d", info.Size, info.LastModified.UnixNano())
	}
	return fmt.Sprintf("%d", info.Size)
}
