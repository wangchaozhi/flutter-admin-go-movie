package video

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/store"

	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"
)

type sourceVideoSize struct {
	width  int
	height int
}

type transcodeQuality struct {
	name      string
	height    int
	scale     string
	videoBit  string
	audioBit  string
	bandwidth string
	res       string
}

func HandleTranscodeTask(ctx context.Context, t *asynq.Task) error {
	p, err := ParseTranscodePayload(t)
	if err != nil {
		return err
	}

	now := time.Now()
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"status":     "processing",
		"started_at": now,
	})

	duration, err := runTranscode(ctx, p.VideoID)
	if err != nil {
		errMsg := err.Error()
		fin := time.Now()
		store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errMsg,
			"finished_at":   fin,
		})
		store.DB().Model(&store.Video{}).Where("id = ?", p.VideoID).Updates(map[string]interface{}{
			"status":     "failed",
			"updated_at": fin,
		})
		return err
	}

	masterKey := fmt.Sprintf("hls/%d/master.m3u8", p.VideoID)
	coverKey := fmt.Sprintf("covers/%d/cover.jpg", p.VideoID)
	fin := time.Now()
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"status":      "success",
		"finished_at": fin,
	})
	store.DB().Model(&store.Video{}).Where("id = ?", p.VideoID).Updates(map[string]interface{}{
		"status":         "ready",
		"hls_master_key": masterKey,
		"cover_key":      coverKey,
		"duration":       duration,
		"updated_at":     fin,
	})
	return nil
}

func runTranscode(ctx context.Context, videoID int64) (int, error) {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("transcode_%d_", videoID))
	if err != nil {
		return 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "source.mp4")
	srcKey := fmt.Sprintf("originals/%d/source.mp4", videoID)
	if err := downloadFromMinio(ctx, srcKey, srcPath); err != nil {
		return 0, fmt.Errorf("download source: %w", err)
	}

	qualities := selectTranscodeQualities(probeSourceSize(srcPath))

	for _, q := range qualities {
		outDir := filepath.Join(tmpDir, q.name)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return 0, err
		}
		segPattern := filepath.Join(outDir, "seg_%03d.ts")
		m3u8Out := filepath.Join(outDir, "index.m3u8")

		args := []string{
			"-i", srcPath,
			"-vf", "scale=" + q.scale,
			"-c:v", "libx264", "-preset", "veryfast", "-b:v", q.videoBit,
			"-g", "180",
			"-keyint_min", "180",
			"-force_key_frames", "expr:gte(t,n_forced*6)",
			"-sc_threshold", "0",
			"-c:a", "aac", "-b:a", q.audioBit,
			"-hls_time", "6",
			"-hls_playlist_type", "vod",
			"-hls_flags", "independent_segments",
			"-hls_segment_filename", segPattern,
			m3u8Out,
		}
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return 0, fmt.Errorf("ffmpeg %s: %w\n%s", q.name, err, stderr.String())
		}

		if err := uploadDir(ctx, outDir, fmt.Sprintf("hls/%d/%s", videoID, q.name)); err != nil {
			return 0, fmt.Errorf("upload %s hls: %w", q.name, err)
		}
	}

	masterContent := "#EXTM3U\n#EXT-X-VERSION:3\n\n"
	for _, q := range qualities {
		masterContent += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%s\n%s/index.m3u8\n\n", q.bandwidth, q.res, q.name)
	}
	masterKey := fmt.Sprintf("hls/%d/master.m3u8", videoID)
	if err := putText(ctx, masterKey, masterContent, "application/vnd.apple.mpegurl"); err != nil {
		return 0, fmt.Errorf("upload master.m3u8: %w", err)
	}

	duration := probeDuration(srcPath)

	coverKey := fmt.Sprintf("covers/%d/cover.jpg", videoID)
	if err := extractAndUploadCover(ctx, srcPath, coverKey); err != nil {
		log.Printf("cover extraction failed for video %d: %v", videoID, err)
	}

	log.Printf("transcode done: video_id=%d duration=%ds", videoID, duration)
	return duration, nil
}

func selectTranscodeQualities(sourceSize sourceVideoSize) []transcodeQuality {
	all := []transcodeQuality{
		{"360p", 360, "-2:360", "800k", "96k", "1000000", "640x360"},
		{"480p", 480, "-2:480", "1200k", "96k", "1400000", "854x480"},
		{"720p", 720, "-2:720", "2500k", "128k", "2800000", "1280x720"},
		{"1080p", 1080, "-2:1080", "5000k", "128k", "5500000", "1920x1080"},
	}
	if sourceSize.height <= 0 || sourceSize.width <= 0 {
		return all
	}

	selected := make([]transcodeQuality, 0, len(all))
	for _, q := range all {
		if q.height <= sourceSize.height+16 {
			selected = append(selected, withResolution(q, sourceSize))
		}
	}
	if len(selected) > 0 {
		return selected
	}

	lowest := all[0]
	targetHeight := evenHeight(sourceSize.height)
	lowest.name = fmt.Sprintf("%dp", targetHeight)
	lowest.height = targetHeight
	lowest.scale = fmt.Sprintf("-2:%d", targetHeight)
	lowest = withResolution(lowest, sourceSize)
	return []transcodeQuality{lowest}
}

func withResolution(q transcodeQuality, sourceSize sourceVideoSize) transcodeQuality {
	width := evenWidth(sourceSize.width * q.height / sourceSize.height)
	if width < 2 {
		width = 2
	}
	q.res = fmt.Sprintf("%dx%d", width, q.height)
	return q
}

func evenWidth(width int) int {
	if width < 2 {
		return 2
	}
	if width%2 == 1 {
		return width + 1
	}
	return width
}

func evenHeight(height int) int {
	if height < 2 {
		return 2
	}
	if height%2 == 1 {
		return height - 1
	}
	return height
}

// probeDuration returns video duration in seconds using ffprobe.
func probeDuration(srcPath string) int {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		srcPath,
	).Output()
	if err != nil {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return int(f)
}

func probeSourceSize(srcPath string) sourceVideoSize {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		srcPath,
	).Output()
	if err != nil {
		return sourceVideoSize{}
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "x")
	if len(parts) != 2 {
		return sourceVideoSize{}
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return sourceVideoSize{}
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return sourceVideoSize{}
	}
	return sourceVideoSize{width: width, height: height}
}

// extractAndUploadCover grabs a frame at 5 s and uploads it to MinIO.
func extractAndUploadCover(ctx context.Context, srcPath, coverKey string) error {
	tmp, err := os.CreateTemp("", "cover_*.jpg")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", "00:00:05",
		"-i", srcPath,
		"-vframes", "1",
		"-y", tmp.Name(),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg cover: %w\n%s", err, stderr.String())
	}

	f, err := os.Open(tmp.Name())
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	_, err = store.ObjectClient().PutObject(ctx, store.VideoBucket(), coverKey, f, info.Size(),
		minio.PutObjectOptions{ContentType: "image/jpeg"})
	return err
}

func downloadFromMinio(ctx context.Context, key, dst string) error {
	obj, err := store.ObjectClient().GetObject(ctx, store.VideoBucket(), key, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, obj)
	return err
}

func uploadDir(ctx context.Context, localDir, minioPrefix string) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(localDir, path)
		key := minioPrefix + "/" + rel

		ct := "video/mp2t"
		if filepath.Ext(path) == ".m3u8" {
			ct = "application/vnd.apple.mpegurl"
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		// pass -1 so the SDK uses chunked/multipart upload; avoids Content-Length mismatch
		_, err = store.ObjectClient().PutObject(ctx, store.VideoBucket(), key, f, -1, minio.PutObjectOptions{ContentType: ct})
		return err
	})
}

func putText(ctx context.Context, key, content, contentType string) error {
	r := bytes.NewReader([]byte(content))
	_, err := store.ObjectClient().PutObject(ctx, store.VideoBucket(), key, r, int64(len(content)), minio.PutObjectOptions{ContentType: contentType})
	return err
}
