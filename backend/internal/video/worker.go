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
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"flutter-admin-go/internal/config"
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

type transcodeEncoder struct {
	name        string
	vaapiDevice string
}

type masterPlaylistEntry struct {
	name       string
	bandwidth  string
	resolution string
	uri        string
}

var (
	transcodeEncoderOnce sync.Once
	cachedEncoder        transcodeEncoder
	videoTranscodeLocks  sync.Map
	sourceCacheLocks     sync.Map
)

func HandleTranscodeTask(ctx context.Context, t *asynq.Task) error {
	p, err := ParseTranscodePayload(t)
	if err != nil {
		return err
	}

	now := time.Now()
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"status":        "processing",
		"error_message": "",
		"started_at":    now,
		"finished_at":   nil,
	})

	requestedQualities := p.Qualities
	mergeMaster := p.MergeExisting
	if p.Quality != "" {
		requestedQualities = []string{p.Quality}
		mergeMaster = true
	}

	duration, err := runTranscodeForTask(ctx, p.VideoID, p.TaskID, requestedQualities, mergeMaster)
	if err != nil {
		errMsg := err.Error()
		fin := time.Now()
		store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errMsg,
			"finished_at":   fin,
		})
		finalizeTranscodeBatch(p.VideoID, p.TaskID, p.PreviousStatus)
		return err
	}

	masterKey := fmt.Sprintf("hls/%d/master.m3u8", p.VideoID)
	coverKey := fmt.Sprintf("covers/%d/cover.jpg", p.VideoID)
	fin := time.Now()
	store.DB().Model(&store.Video{}).Where("id = ?", p.VideoID).Updates(map[string]interface{}{
		"hls_master_key": masterKey,
		"cover_key":      coverKey,
		"duration":       duration,
		"updated_at":     fin,
	})
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"status":      "success",
		"finished_at": fin,
	})
	finalizeTranscodeBatch(p.VideoID, p.TaskID, p.PreviousStatus)
	return nil
}

func runTranscodeForTask(ctx context.Context, videoID, taskID int64, requestedQualities []string, mergeExisting bool) (int, error) {
	lock := videoTranscodeLock(videoID)
	log.Printf("transcode video lock wait: video_id=%d task_id=%d", videoID, taskID)
	lock.Lock()
	defer lock.Unlock()

	log.Printf("transcode video lock acquired: video_id=%d task_id=%d", videoID, taskID)
	return runTranscode(ctx, videoID, requestedQualities, mergeExisting)
}

func videoTranscodeLock(videoID int64) *sync.Mutex {
	lock, _ := videoTranscodeLocks.LoadOrStore(videoID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func finalizeTranscodeBatch(videoID, taskID int64, previousStatus string) {
	var current store.VideoTranscodeTask
	if err := store.DB().First(&current, taskID).Error; err != nil {
		return
	}

	tasks := []store.VideoTranscodeTask{current}
	if current.BatchID != 0 {
		if err := store.DB().Where("video_id = ? AND batch_id = ?", videoID, current.BatchID).Find(&tasks).Error; err != nil {
			return
		}
	}

	hasSuccess := false
	for _, task := range tasks {
		switch task.Status {
		case "pending", "processing":
			return
		case "success":
			hasSuccess = true
		}
	}

	status := "failed"
	if hasSuccess {
		status = "ready"
		if previousStatus == "offline" {
			status = "offline"
		}
	} else if previousStatus == "ready" || previousStatus == "offline" {
		status = previousStatus
	}

	store.DB().Model(&store.Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	})
}

func runTranscode(ctx context.Context, videoID int64, requestedQualities []string, mergeExisting bool) (int, error) {
	tmpRoot := strings.TrimSpace(config.Load().Worker.TranscodeTempDir)
	if tmpRoot != "" {
		if err := os.MkdirAll(tmpRoot, 0755); err != nil {
			return 0, fmt.Errorf("create transcode temp root: %w", err)
		}
	}
	tmpDir, err := os.MkdirTemp(tmpRoot, fmt.Sprintf("transcode_%d_", videoID))
	if err != nil {
		return 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	srcKey := fmt.Sprintf("originals/%d/source.mp4", videoID)
	srcPath, err := cachedSourcePath(ctx, videoID, srcKey, tmpRoot)
	if err != nil {
		return 0, err
	}

	qualities, err := selectRequestedTranscodeQualities(probeSourceSize(srcPath), requestedQualities)
	if err != nil {
		return 0, err
	}
	encoder := selectedTranscodeEncoder(ctx)
	log.Printf("transcode encoder selected: %s", encoder.name)

	completedQualities := make([]transcodeQuality, 0, len(qualities))
	for _, q := range qualities {
		outDir := filepath.Join(tmpDir, q.name)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return 0, err
		}
		segPattern := filepath.Join(outDir, "seg_%03d.ts")
		m3u8Out := filepath.Join(outDir, "index.m3u8")

		args := buildTranscodeArgs(encoder, q, srcPath, segPattern, m3u8Out)
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			persistCompletedMaster(ctx, videoID, completedQualities, mergeExisting)
			return 0, fmt.Errorf("ffmpeg %s: %w\n%s", q.name, err, stderr.String())
		}

		if err := uploadDir(ctx, outDir, fmt.Sprintf("hls/%d/%s", videoID, q.name)); err != nil {
			persistCompletedMaster(ctx, videoID, completedQualities, mergeExisting)
			return 0, fmt.Errorf("upload %s hls: %w", q.name, err)
		}
		completedQualities = append(completedQualities, q)
	}

	if err := putMasterPlaylist(ctx, videoID, completedQualities, mergeExisting); err != nil {
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

func cachedSourcePath(ctx context.Context, videoID int64, srcKey, tmpRoot string) (string, error) {
	info, err := store.ObjectClient().StatObject(ctx, store.VideoBucket(), srcKey, minio.StatObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("stat source: %w", err)
	}

	cacheRoot := sourceCacheRoot(tmpRoot)
	videoCacheDir := filepath.Join(cacheRoot, strconv.FormatInt(videoID, 10))
	cachePath := filepath.Join(videoCacheDir, sourceCacheFileName(info))
	lock := sourceCacheLock(cachePath)
	lock.Lock()
	defer lock.Unlock()

	if sourceCacheFileUsable(cachePath, info.Size) {
		log.Printf("transcode source cache hit: video_id=%d path=%s", videoID, cachePath)
		return cachePath, nil
	}

	if err := os.MkdirAll(videoCacheDir, 0755); err != nil {
		return "", fmt.Errorf("create source cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(videoCacheDir, ".source_*.mp4")
	if err != nil {
		return "", fmt.Errorf("create source cache temp: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close source cache temp: %w", err)
	}
	defer os.Remove(tmpPath)

	if err := downloadFromMinio(ctx, srcKey, tmpPath); err != nil {
		return "", fmt.Errorf("download source: %w", err)
	}
	if !sourceCacheFileUsable(tmpPath, info.Size) {
		return "", fmt.Errorf("downloaded source cache size mismatch")
	}
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("replace source cache: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", fmt.Errorf("move source cache: %w", err)
	}
	pruneSourceCache(videoCacheDir, filepath.Base(cachePath))
	log.Printf("transcode source cached: video_id=%d path=%s", videoID, cachePath)
	return cachePath, nil
}

func sourceCacheRoot(tmpRoot string) string {
	tmpRoot = strings.TrimSpace(tmpRoot)
	if tmpRoot != "" {
		return filepath.Join(tmpRoot, "source-cache")
	}
	return filepath.Join(os.TempDir(), "flutter-admin-go-transcode-source-cache")
}

func sourceCacheLock(cachePath string) *sync.Mutex {
	lock, _ := sourceCacheLocks.LoadOrStore(cachePath, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func sourceCacheFileName(info minio.ObjectInfo) string {
	token := strings.Trim(info.ETag, `"`)
	if token == "" {
		token = fmt.Sprintf("%d_%d", info.Size, info.LastModified.UnixNano())
	}
	return sanitizeCacheToken(token) + ".mp4"
}

func sanitizeCacheToken(token string) string {
	var b strings.Builder
	for _, r := range token {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	value := strings.Trim(b.String(), "_")
	if value == "" {
		return "source"
	}
	return value
}

func sourceCacheFileUsable(path string, wantSize int64) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return wantSize <= 0 || info.Size() == wantSize
}

func pruneSourceCache(videoCacheDir, keepName string) {
	entries, err := os.ReadDir(videoCacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == keepName {
			continue
		}
		_ = os.Remove(filepath.Join(videoCacheDir, entry.Name()))
	}
}

func persistCompletedMaster(ctx context.Context, videoID int64, qualities []transcodeQuality, mergeExisting bool) {
	if len(qualities) == 0 {
		return
	}
	if err := putMasterPlaylist(ctx, videoID, qualities, mergeExisting); err != nil {
		log.Printf("partial master playlist update failed for video %d: %v", videoID, err)
	}
}

func putMasterPlaylist(ctx context.Context, videoID int64, qualities []transcodeQuality, mergeExisting bool) error {
	masterKey := fmt.Sprintf("hls/%d/master.m3u8", videoID)
	existingMaster := ""
	if mergeExisting {
		if raw, err := readMinioText(ctx, masterKey); err == nil {
			existingMaster = raw
		} else {
			log.Printf("read existing master playlist failed for video %d: %v", videoID, err)
		}
	}
	masterContent := buildMasterPlaylist(existingMaster, qualities)
	return putText(ctx, masterKey, masterContent, "application/vnd.apple.mpegurl")
}

func buildMasterPlaylist(existing string, qualities []transcodeQuality) string {
	entriesByName := make(map[string]masterPlaylistEntry)
	for _, entry := range parseMasterPlaylistEntries(existing) {
		entriesByName[entry.name] = entry
	}
	for _, q := range qualities {
		entriesByName[q.name] = masterPlaylistEntry{
			name:       q.name,
			bandwidth:  q.bandwidth,
			resolution: q.res,
			uri:        fmt.Sprintf("%s/index.m3u8", q.name),
		}
	}

	entries := make([]masterPlaylistEntry, 0, len(entriesByName))
	for _, entry := range entriesByName {
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left := qualityHeight(entries[i].name)
		right := qualityHeight(entries[j].name)
		if left == right {
			return entries[i].name < entries[j].name
		}
		if left == 0 {
			return false
		}
		if right == 0 {
			return true
		}
		return left < right
	})

	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n\n")
	for _, entry := range entries {
		b.WriteString("#EXT-X-STREAM-INF")
		attrs := make([]string, 0, 2)
		if entry.bandwidth != "" {
			attrs = append(attrs, "BANDWIDTH="+entry.bandwidth)
		}
		if entry.resolution != "" {
			attrs = append(attrs, "RESOLUTION="+entry.resolution)
		}
		if len(attrs) > 0 {
			b.WriteString(":")
			b.WriteString(strings.Join(attrs, ","))
		}
		b.WriteString("\n")
		b.WriteString(entry.uri)
		b.WriteString("\n\n")
	}
	return b.String()
}

func parseMasterPlaylistEntries(raw string) []masterPlaylistEntry {
	var entries []masterPlaylistEntry
	bandwidth := ""
	resolution := ""
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			bandwidth = parseMasterAttribute(line, "BANDWIDTH")
			resolution = parseMasterAttribute(line, "RESOLUTION")
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasSuffix(masterURIPath(line), ".m3u8") {
			continue
		}
		name := qualityNameFromMasterURI(line)
		if name == "" {
			bandwidth = ""
			resolution = ""
			continue
		}
		entries = append(entries, masterPlaylistEntry{
			name:       name,
			bandwidth:  bandwidth,
			resolution: resolution,
			uri:        line,
		})
		bandwidth = ""
		resolution = ""
	}
	return entries
}

func parseMasterAttribute(line, key string) string {
	_, attrs, ok := strings.Cut(line, ":")
	if !ok {
		return ""
	}
	prefix := key + "="
	for _, part := range strings.Split(attrs, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

func qualityNameFromMasterURI(uri string) string {
	path := strings.Trim(masterURIPath(uri), "/")
	path = strings.TrimSuffix(path, "/index.m3u8")
	path = strings.TrimSuffix(path, "index.m3u8")
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func masterURIPath(uri string) string {
	path := strings.TrimSpace(uri)
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	return strings.ReplaceAll(path, "\\", "/")
}

func qualityHeight(name string) int {
	if !strings.HasSuffix(name, "p") {
		return 0
	}
	height, err := strconv.Atoi(strings.TrimSuffix(name, "p"))
	if err != nil {
		return 0
	}
	return height
}

func selectRequestedTranscodeQualities(sourceSize sourceVideoSize, requested []string) ([]transcodeQuality, error) {
	available := selectTranscodeQualities(sourceSize)
	if len(requested) == 0 {
		return available, nil
	}
	requestedSet := make(map[string]bool, len(requested))
	for _, name := range requested {
		requestedSet[strings.ToLower(strings.TrimSpace(name))] = true
	}
	selected := make([]transcodeQuality, 0, len(available))
	for _, q := range available {
		if requestedSet[q.name] {
			selected = append(selected, q)
		}
	}
	if len(selected) == 0 {
		availableNames := make([]string, 0, len(available))
		for _, q := range available {
			availableNames = append(availableNames, q.name)
		}
		return nil, fmt.Errorf("selected transcode qualities are not available for source; available: %s", strings.Join(availableNames, ", "))
	}
	return selected, nil
}

func selectedTranscodeEncoder(ctx context.Context) transcodeEncoder {
	transcodeEncoderOnce.Do(func() {
		requested := strings.ToLower(strings.TrimSpace(config.Load().Worker.TranscodeVideoEncoder))
		if requested == "" {
			requested = "auto"
		}
		cachedEncoder = chooseTranscodeEncoder(ctx, requested)
	})
	return cachedEncoder
}

func chooseTranscodeEncoder(ctx context.Context, requested string) transcodeEncoder {
	supported, err := ffmpegEncoders(ctx)
	if err != nil {
		log.Printf("ffmpeg encoder detection failed, falling back to libx264: %v", err)
		return transcodeEncoder{name: "libx264"}
	}

	if requested != "auto" {
		encoder := newTranscodeEncoder(requested)
		if encoder.name == "" {
			log.Printf("unknown transcode encoder %q, falling back to libx264", requested)
			return transcodeEncoder{name: "libx264"}
		}
		if isUsableTranscodeEncoder(ctx, encoder, supported) {
			return encoder
		}
		log.Printf("configured transcode encoder %q is not usable, falling back to libx264", requested)
		return transcodeEncoder{name: "libx264"}
	}

	for _, candidate := range autoTranscodeEncoderCandidates() {
		encoder := newTranscodeEncoder(candidate)
		if isUsableTranscodeEncoder(ctx, encoder, supported) {
			return encoder
		}
	}
	return transcodeEncoder{name: "libx264"}
}

func ffmpegEncoders(ctx context.Context) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", err, string(out))
	}
	supported := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			supported[fields[1]] = true
		}
	}
	return supported, nil
}

func autoTranscodeEncoderCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"h264_videotoolbox", "libx264"}
	case "windows":
		return []string{"h264_nvenc", "h264_qsv", "h264_amf", "libx264"}
	default:
		return []string{"h264_nvenc", "h264_qsv", "h264_vaapi", "h264_amf", "libx264"}
	}
}

func newTranscodeEncoder(name string) transcodeEncoder {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "libx264", "h264_nvenc", "h264_qsv", "h264_videotoolbox", "h264_amf":
		return transcodeEncoder{name: strings.ToLower(strings.TrimSpace(name))}
	case "h264_vaapi":
		return transcodeEncoder{name: "h264_vaapi", vaapiDevice: detectVAAPIDevice()}
	default:
		return transcodeEncoder{}
	}
}

func detectVAAPIDevice() string {
	for _, device := range []string{"/dev/dri/renderD128", "/dev/dri/card0"} {
		if _, err := os.Stat(device); err == nil {
			return device
		}
	}
	return ""
}

func isUsableTranscodeEncoder(ctx context.Context, encoder transcodeEncoder, supported map[string]bool) bool {
	if encoder.name == "" || !supported[encoder.name] {
		return false
	}
	args := buildEncoderProbeArgs(encoder)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("transcode encoder %s probe failed: %v\n%s", encoder.name, err, stderr.String())
		return false
	}
	return true
}

func buildEncoderProbeArgs(encoder transcodeEncoder) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}
	args = append(args, encoderDeviceArgs(encoder)...)
	args = append(args,
		"-f", "lavfi",
		"-i", "testsrc2=size=256x144:rate=1",
		"-t", "1",
		"-vf", encoderScaleFilter(encoder, "256:144"),
	)
	args = append(args, encoderVideoArgs(encoder, "500k")...)
	args = append(args, encoderGOPArgs(encoder)...)
	args = append(args, "-an", "-f", "null", "-")
	return args
}

func buildTranscodeArgs(encoder transcodeEncoder, q transcodeQuality, srcPath, segPattern, m3u8Out string) []string {
	args := []string{}
	args = append(args, encoderDeviceArgs(encoder)...)
	args = append(args,
		"-i", srcPath,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-sn",
		"-vf", encoderScaleFilter(encoder, q.scale),
	)
	args = append(args, encoderVideoArgs(encoder, q.videoBit)...)
	args = append(args, encoderGOPArgs(encoder)...)
	args = append(args,
		"-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", q.audioBit,
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", segPattern,
		m3u8Out,
	)
	return args
}

func encoderDeviceArgs(encoder transcodeEncoder) []string {
	if encoder.name == "h264_vaapi" && encoder.vaapiDevice != "" {
		return []string{"-vaapi_device", encoder.vaapiDevice}
	}
	return nil
}

func encoderScaleFilter(encoder transcodeEncoder, scale string) string {
	if encoder.name == "h264_vaapi" {
		return "scale=" + scale + ",format=nv12,hwupload"
	}
	return "scale=" + scale + ",format=yuv420p"
}

func encoderGOPArgs(encoder transcodeEncoder) []string {
	args := []string{
		"-g", "180",
		"-force_key_frames", "expr:gte(t,n_forced*6)",
	}
	if encoder.name == "libx264" {
		args = append(args, "-keyint_min", "180", "-sc_threshold", "0")
	}
	return args
}

func encoderVideoArgs(encoder transcodeEncoder, bitrate string) []string {
	switch encoder.name {
	case "h264_nvenc":
		return []string{"-c:v", "h264_nvenc", "-preset", "fast", "-b:v", bitrate}
	case "h264_qsv":
		return []string{"-c:v", "h264_qsv", "-preset", "veryfast", "-b:v", bitrate}
	case "h264_vaapi":
		return []string{"-c:v", "h264_vaapi", "-b:v", bitrate}
	case "h264_videotoolbox":
		return []string{"-c:v", "h264_videotoolbox", "-b:v", bitrate}
	case "h264_amf":
		return []string{"-c:v", "h264_amf", "-quality", "speed", "-b:v", bitrate}
	default:
		return []string{"-c:v", "libx264", "-preset", "veryfast", "-b:v", bitrate}
	}
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
