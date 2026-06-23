package video

import (
	"bytes"
	"context"
	"errors"
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

type nonRetryableTranscodeError struct {
	err error
}

func (e nonRetryableTranscodeError) Error() string {
	return e.err.Error()
}

func (e nonRetryableTranscodeError) Unwrap() error {
	return e.err
}

const transcodeQueuedStartWait = 30 * time.Second
const staleTranscodeTaskAge = transcodeTaskTimeout + 30*time.Minute
const staleQueuedTaskAge = 10 * time.Minute
const sourceCacheMaxAge = 7 * 24 * time.Hour
const sourceCachePruneInterval = time.Hour

var (
	transcodeEncoderOnce sync.Once
	cachedEncoder        transcodeEncoder
	videoTranscodeLocks  sync.Map
	sourceCacheLocks     sync.Map
	sourceCachePruneMu   sync.Mutex
	sourceCachePrunedAt  time.Time
)

func HandleTranscodeTask(ctx context.Context, t *asynq.Task) error {
	p, err := ParseTranscodePayload(t)
	if err != nil {
		return err
	}

	requestedQualities := p.Qualities
	mergeMaster := p.MergeExisting
	if p.Quality != "" {
		requestedQualities = []string{p.Quality}
		mergeMaster = true
	}

	taskState, shouldRun, err := waitForRunnableTranscodeTask(ctx, p.TaskID)
	if taskState.PreviousStatus != "" {
		p.PreviousStatus = taskState.PreviousStatus
	}
	if err != nil {
		return handleTranscodeTaskError(ctx, p, err)
	}
	if !shouldRun {
		return nil
	}

	attempt := 1
	if retried, ok := asynq.GetRetryCount(ctx); ok {
		attempt = retried + 1
	}
	now := time.Now()
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"status":         "processing",
		"status_message": "准备转码",
		"progress":       1,
		"attempt":        attempt,
		"error_message":  "",
		"started_at":     now,
		"finished_at":    nil,
	})

	if err := runTranscodeForTask(ctx, p.VideoID, p.TaskID, p.BatchID, requestedQualities, mergeMaster); err != nil {
		return handleTranscodeTaskError(ctx, p, err)
	}

	fin := time.Now()
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"status":         "success",
		"status_message": "完成",
		"progress":       100,
		"finished_at":    fin,
	})
	finalizeTranscodeBatch(ctx, p.VideoID, p.TaskID, p.PreviousStatus)
	return nil
}

func waitForRunnableTranscodeTask(ctx context.Context, taskID int64) (store.VideoTranscodeTask, bool, error) {
	deadline := time.Now().Add(transcodeQueuedStartWait)
	for {
		var task store.VideoTranscodeTask
		if err := store.DB().First(&task, taskID).Error; err != nil {
			return task, false, err
		}
		switch task.Status {
		case "queued":
			if time.Now().After(deadline) {
				return task, false, fmt.Errorf("transcode task stayed queued")
			}
			timer := time.NewTimer(200 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return task, false, ctx.Err()
			case <-timer.C:
			}
		case "pending", "processing":
			return task, true, nil
		default:
			log.Printf("skip transcode task: task_id=%d status=%s", task.ID, task.Status)
			return task, false, nil
		}
	}
}

func handleTranscodeTaskError(ctx context.Context, p *TranscodePayload, err error) error {
	returnErr := transcodeTaskReturnError(err)
	errMsg := transcodeErrorMessage(err)
	fin := time.Now()
	if transcodeTaskWillRetry(ctx, returnErr) {
		store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
			"status":         "pending",
			"status_message": "等待重试",
			"progress":       0,
			"error_message":  errMsg,
			"finished_at":    nil,
		})
		return returnErr
	}
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"status":         "failed",
		"status_message": "失败",
		"progress":       100,
		"error_message":  errMsg,
		"finished_at":    fin,
	})
	finalizeTranscodeBatch(ctx, p.VideoID, p.TaskID, p.PreviousStatus)
	return returnErr
}

func runTranscodeForTask(ctx context.Context, videoID, taskID, batchID int64, requestedQualities []string, mergeExisting bool) error {
	lock := videoTranscodeLock(videoID)
	log.Printf("transcode video lock wait: video_id=%d task_id=%d", videoID, taskID)
	lock.Lock()
	defer lock.Unlock()

	log.Printf("transcode video lock acquired: video_id=%d task_id=%d", videoID, taskID)
	return withVideoTranscodeAdvisoryLock(ctx, videoID, taskID, func() error {
		return runTranscode(ctx, videoID, taskID, batchID, requestedQualities, mergeExisting)
	})
}

func videoTranscodeLock(videoID int64) *sync.Mutex {
	lock, _ := videoTranscodeLocks.LoadOrStore(videoID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func transcodeTaskReturnError(err error) error {
	if isNonRetryableTranscodeError(err) {
		return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
	}
	return err
}

func transcodeTaskWillRetry(ctx context.Context, err error) bool {
	if errors.Is(err, asynq.SkipRetry) {
		return false
	}
	retried, ok := asynq.GetRetryCount(ctx)
	if !ok {
		return false
	}
	maxRetry, ok := asynq.GetMaxRetry(ctx)
	if !ok {
		return false
	}
	return retried < maxRetry
}

func transcodeErrorMessage(err error) string {
	var nonRetryable nonRetryableTranscodeError
	if errors.As(err, &nonRetryable) {
		return nonRetryable.err.Error()
	}
	return err.Error()
}

func markNonRetryableTranscodeError(err error) error {
	return nonRetryableTranscodeError{err: err}
}

func isNonRetryableTranscodeError(err error) bool {
	var nonRetryable nonRetryableTranscodeError
	return errors.As(err, &nonRetryable)
}

func withVideoTranscodeAdvisoryLock(ctx context.Context, videoID, taskID int64, fn func() error) error {
	sqlDB, err := store.DB().DB()
	if err != nil {
		return fmt.Errorf("get postgres db: %w", err)
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("get postgres conn: %w", err)
	}
	defer conn.Close()

	key := videoTranscodeAdvisoryKey(videoID)
	log.Printf("transcode db lock wait: video_id=%d task_id=%d key=%d", videoID, taskID, key)
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		return fmt.Errorf("acquire transcode db lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", key); err != nil {
			log.Printf("release transcode db lock failed: video_id=%d task_id=%d key=%d: %v", videoID, taskID, key, err)
		}
	}()

	log.Printf("transcode db lock acquired: video_id=%d task_id=%d key=%d", videoID, taskID, key)
	return fn()
}

func videoTranscodeAdvisoryKey(videoID int64) int64 {
	const namespace int64 = 0x5452
	return (namespace << 48) | (videoID & 0x0000ffffffffffff)
}

func finalizeTranscodeBatch(ctx context.Context, videoID, taskID int64, previousStatus string) {
	var current store.VideoTranscodeTask
	if err := store.DB().First(&current, taskID).Error; err != nil {
		return
	}
	if previousStatus == "" {
		previousStatus = current.PreviousStatus
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

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if hasSuccess {
		refreshTranscodeVideoMetadata(ctx, videoID, updates)
	}

	store.DB().Model(&store.Video{}).Where("id = ?", videoID).Updates(updates)
	if hasSuccess {
		pruneUnreferencedHLSVersions(ctx, videoID)
	}
}

func reconcileStaleTranscodeTasks(ctx context.Context, videoID int64) {
	now := time.Now()
	processingCutoff := now.Add(-staleTranscodeTaskAge)
	queuedCutoff := now.Add(-staleQueuedTaskAge)
	var tasks []store.VideoTranscodeTask
	err := store.DB().
		Where("video_id = ? AND ((status = ? AND started_at < ?) OR (status = ? AND created_at < ?))", videoID, "processing", processingCutoff, "queued", queuedCutoff).
		Find(&tasks).Error
	if err != nil || len(tasks) == 0 {
		return
	}

	for _, task := range tasks {
		message := "任务超时，已自动恢复"
		if task.Status == "queued" {
			message = "任务入队超时，已自动失败"
		}
		store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"status":         "failed",
			"status_message": message,
			"progress":       100,
			"error_message":  message,
			"finished_at":    now,
		})
		finalizeTranscodeBatch(ctx, task.VideoID, task.ID, task.PreviousStatus)
	}
}

func runTranscode(ctx context.Context, videoID, taskID, batchID int64, requestedQualities []string, mergeExisting bool) error {
	updateTranscodeTaskProgress(taskID, "准备源文件", 5)
	tmpRoot := strings.TrimSpace(config.Load().Worker.TranscodeTempDir)
	if tmpRoot != "" {
		if err := os.MkdirAll(tmpRoot, 0755); err != nil {
			return fmt.Errorf("create transcode temp root: %w", err)
		}
	}
	tmpDir, err := os.MkdirTemp(tmpRoot, fmt.Sprintf("transcode_%d_", videoID))
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	srcKey := fmt.Sprintf("originals/%d/source.mp4", videoID)
	srcPath, err := cachedSourcePath(ctx, videoID, srcKey, tmpRoot)
	if err != nil {
		return err
	}

	updateTranscodeTaskProgress(taskID, "检查清晰度", 10)
	qualities, err := selectRequestedTranscodeQualities(probeSourceSize(srcPath), requestedQualities)
	if err != nil {
		return err
	}
	updateTranscodeTaskProgress(taskID, "选择编码器", 15)
	encoder := selectedTranscodeEncoder(ctx)
	log.Printf("transcode encoder selected: %s", encoder.name)

	outputVersion := transcodeOutputVersion(batchID, taskID)
	completedQualities := make([]transcodeQuality, 0, len(qualities))
	for _, q := range qualities {
		updateTranscodeTaskProgress(taskID, "转码 "+q.name, 20)
		outDir := filepath.Join(tmpDir, q.name)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return err
		}
		segPattern := filepath.Join(outDir, "seg_%03d.ts")
		m3u8Out := filepath.Join(outDir, "index.m3u8")

		args := buildTranscodeArgs(encoder, q, srcPath, segPattern, m3u8Out)
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			persistCompletedMaster(ctx, videoID, completedQualities, mergeExisting, outputVersion)
			return fmt.Errorf("ffmpeg %s: %w\n%s", q.name, err, stderr.String())
		}

		updateTranscodeTaskProgress(taskID, "上传 "+q.name, 75)
		if err := uploadDir(ctx, outDir, transcodeQualityObjectPrefix(videoID, outputVersion, q.name)); err != nil {
			persistCompletedMaster(ctx, videoID, completedQualities, mergeExisting, outputVersion)
			return fmt.Errorf("upload %s hls: %w", q.name, err)
		}
		completedQualities = append(completedQualities, q)
	}

	updateTranscodeTaskProgress(taskID, "更新播放列表", 90)
	if err := putMasterPlaylist(ctx, videoID, completedQualities, mergeExisting, outputVersion); err != nil {
		return fmt.Errorf("upload master.m3u8: %w", err)
	}

	updateTranscodeTaskProgress(taskID, "等待批次完成", 95)
	log.Printf("transcode done: video_id=%d qualities=%s", videoID, strings.Join(transcodeQualityNames(completedQualities), ","))
	return nil
}

func updateTranscodeTaskProgress(taskID int64, message string, progress int) {
	if taskID <= 0 {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	store.DB().Model(&store.VideoTranscodeTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status_message": message,
		"progress":       progress,
	})
}

func refreshTranscodeVideoMetadata(ctx context.Context, videoID int64, updates map[string]interface{}) {
	updates["hls_master_key"] = fmt.Sprintf("hls/%d/master.m3u8", videoID)

	v := store.Video{ID: videoID, OriginalKey: fmt.Sprintf("originals/%d/source.mp4", videoID)}
	_ = store.DB().First(&v, videoID).Error

	srcPath, sourceSize, duration, err := probeCachedVideoSource(ctx, v)
	if err != nil {
		log.Printf("refresh transcode metadata source probe failed for video %d: %v", videoID, err)
		return
	}
	if duration > 0 {
		updates["duration"] = duration
	}
	if hasSourceSize(sourceSize) {
		updates["source_width"] = sourceSize.width
		updates["source_height"] = sourceSize.height
	}

	coverKey := fmt.Sprintf("covers/%d/cover.jpg", videoID)
	if err := extractAndUploadCover(ctx, srcPath, coverKey); err != nil {
		log.Printf("cover extraction failed for video %d: %v", videoID, err)
		return
	}
	updates["cover_key"] = coverKey
}

func ensureSourceVideoMetadata(ctx context.Context, v *store.Video) sourceVideoSize {
	sourceSize := sourceSizeFromVideo(*v)
	if hasSourceSize(sourceSize) {
		return sourceSize
	}

	_, sourceSize, duration, err := probeCachedVideoSource(ctx, *v)
	if err != nil {
		log.Printf("source metadata probe failed for video %d: %v", v.ID, err)
		return sourceSizeFromVideo(*v)
	}

	updates := map[string]interface{}{}
	if hasSourceSize(sourceSize) {
		updates["source_width"] = sourceSize.width
		updates["source_height"] = sourceSize.height
		v.SourceWidth = sourceSize.width
		v.SourceHeight = sourceSize.height
	}
	if duration > 0 && v.Duration == 0 {
		updates["duration"] = duration
		v.Duration = duration
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		store.DB().Model(&store.Video{}).Where("id = ?", v.ID).Updates(updates)
	}
	return sourceSize
}

func probeCachedVideoSource(ctx context.Context, v store.Video) (string, sourceVideoSize, int, error) {
	srcKey := sourceKeyForVideo(v)
	srcPath, err := cachedSourcePath(ctx, v.ID, srcKey, strings.TrimSpace(config.Load().Worker.TranscodeTempDir))
	if err != nil {
		return "", sourceVideoSize{}, 0, err
	}
	sourceSize := probeSourceSize(srcPath)
	duration := probeDuration(srcPath)
	return srcPath, sourceSize, duration, nil
}

func sourceKeyForVideo(v store.Video) string {
	if key := strings.TrimSpace(v.OriginalKey); key != "" {
		return key
	}
	return fmt.Sprintf("originals/%d/source.mp4", v.ID)
}

func sourceSizeFromVideo(v store.Video) sourceVideoSize {
	return sourceVideoSize{width: v.SourceWidth, height: v.SourceHeight}
}

func hasSourceSize(sourceSize sourceVideoSize) bool {
	return sourceSize.width > 0 && sourceSize.height > 0
}

func cachedSourcePath(ctx context.Context, videoID int64, srcKey, tmpRoot string) (string, error) {
	info, err := store.ObjectClient().StatObject(ctx, store.VideoBucket(), srcKey, minio.StatObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("stat source: %w", err)
	}

	videoCacheDir, cachePath := sourceCachePath(videoID, tmpRoot, info)
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
	maybePruneSourceCacheRoot(tmpRoot)
	log.Printf("transcode source cached: video_id=%d path=%s", videoID, cachePath)
	return cachePath, nil
}

func cacheUploadedSource(ctx context.Context, videoID int64, srcKey string, size int64, etag string, src io.Reader) (string, error) {
	info := minio.ObjectInfo{Size: size, ETag: etag}
	if strings.TrimSpace(info.ETag) == "" {
		stat, err := store.ObjectClient().StatObject(ctx, store.VideoBucket(), srcKey, minio.StatObjectOptions{})
		if err != nil {
			return "", fmt.Errorf("stat uploaded source: %w", err)
		}
		info = stat
	}

	videoCacheDir, cachePath := sourceCachePath(videoID, strings.TrimSpace(config.Load().Worker.TranscodeTempDir), info)
	lock := sourceCacheLock(cachePath)
	lock.Lock()
	defer lock.Unlock()

	if sourceCacheFileUsable(cachePath, info.Size) {
		log.Printf("transcode source cache hit after upload: video_id=%d path=%s", videoID, cachePath)
		return cachePath, nil
	}
	if err := os.MkdirAll(videoCacheDir, 0755); err != nil {
		return "", fmt.Errorf("create source cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(videoCacheDir, ".source_upload_*.mp4")
	if err != nil {
		return "", fmt.Errorf("create source cache temp: %w", err)
	}
	tmpPath := tmp.Name()
	written, copyErr := io.Copy(tmp, src)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("copy uploaded source cache: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close uploaded source cache temp: %w", closeErr)
	}
	defer os.Remove(tmpPath)

	if info.Size > 0 && written != info.Size {
		return "", fmt.Errorf("uploaded source cache size mismatch: copied %d, want %d", written, info.Size)
	}
	if !sourceCacheFileUsable(tmpPath, info.Size) {
		return "", fmt.Errorf("uploaded source cache size mismatch")
	}
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("replace source cache: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", fmt.Errorf("move source cache: %w", err)
	}
	pruneSourceCache(videoCacheDir, filepath.Base(cachePath))
	maybePruneSourceCacheRoot(strings.TrimSpace(config.Load().Worker.TranscodeTempDir))
	log.Printf("transcode source cached from upload: video_id=%d path=%s", videoID, cachePath)
	return cachePath, nil
}

func sourceCachePath(videoID int64, tmpRoot string, info minio.ObjectInfo) (string, string) {
	cacheRoot := sourceCacheRoot(tmpRoot)
	videoCacheDir := filepath.Join(cacheRoot, strconv.FormatInt(videoID, 10))
	return videoCacheDir, filepath.Join(videoCacheDir, sourceCacheFileName(info))
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

func maybePruneSourceCacheRoot(tmpRoot string) {
	sourceCachePruneMu.Lock()
	if time.Since(sourceCachePrunedAt) < sourceCachePruneInterval {
		sourceCachePruneMu.Unlock()
		return
	}
	sourceCachePrunedAt = time.Now()
	sourceCachePruneMu.Unlock()

	root := sourceCacheRoot(tmpRoot)
	cutoff := time.Now().Add(-sourceCacheMaxAge)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			log.Printf("remove stale source cache %s failed: %v", path, err)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		log.Printf("prune source cache root %s failed: %v", root, err)
	}
	removeEmptySourceCacheDirs(root)
}

func removeEmptySourceCacheDirs(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		children, err := os.ReadDir(dir)
		if err == nil && len(children) == 0 {
			_ = os.Remove(dir)
		}
	}
}

func persistCompletedMaster(ctx context.Context, videoID int64, qualities []transcodeQuality, mergeExisting bool, outputVersion string) {
	if len(qualities) == 0 {
		return
	}
	if err := putMasterPlaylist(ctx, videoID, qualities, mergeExisting, outputVersion); err != nil {
		log.Printf("partial master playlist update failed for video %d: %v", videoID, err)
	}
}

func putMasterPlaylist(ctx context.Context, videoID int64, qualities []transcodeQuality, mergeExisting bool, outputVersion string) error {
	masterKey := fmt.Sprintf("hls/%d/master.m3u8", videoID)
	existingMaster := ""
	if mergeExisting {
		if raw, err := readMinioText(ctx, masterKey); err == nil {
			existingMaster = raw
		} else {
			log.Printf("read existing master playlist failed for video %d: %v", videoID, err)
		}
	}
	masterContent := buildVersionedMasterPlaylist(existingMaster, qualities, outputVersion)
	return putText(ctx, masterKey, masterContent, "application/vnd.apple.mpegurl")
}

func buildMasterPlaylist(existing string, qualities []transcodeQuality) string {
	return buildVersionedMasterPlaylist(existing, qualities, "")
}

func buildVersionedMasterPlaylist(existing string, qualities []transcodeQuality, outputVersion string) string {
	entriesByName := make(map[string]masterPlaylistEntry)
	for _, entry := range parseMasterPlaylistEntries(existing) {
		entriesByName[entry.name] = entry
	}
	for _, q := range qualities {
		entriesByName[q.name] = masterPlaylistEntry{
			name:       q.name,
			bandwidth:  q.bandwidth,
			resolution: q.res,
			uri:        transcodeQualityMasterURI(outputVersion, q.name),
		}
	}
	return renderMasterPlaylist(entriesByName)
}

// renderMasterPlaylist writes a master.m3u8 from the given entries, sorted by
// quality height ascending.
func renderMasterPlaylist(entriesByName map[string]masterPlaylistEntry) string {
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

func transcodeOutputVersion(batchID, taskID int64) string {
	if batchID > 0 {
		return strconv.FormatInt(batchID, 10)
	}
	if taskID > 0 {
		return "task-" + strconv.FormatInt(taskID, 10)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func transcodeQualityObjectPrefix(videoID int64, outputVersion, quality string) string {
	if strings.TrimSpace(outputVersion) == "" {
		return fmt.Sprintf("hls/%d/%s", videoID, quality)
	}
	return fmt.Sprintf("hls/%d/versions/%s/%s", videoID, outputVersion, quality)
}

func transcodeQualityMasterURI(outputVersion, quality string) string {
	if strings.TrimSpace(outputVersion) == "" {
		return fmt.Sprintf("%s/index.m3u8", quality)
	}
	return fmt.Sprintf("versions/%s/%s/index.m3u8", outputVersion, quality)
}

func pruneUnreferencedHLSVersions(ctx context.Context, videoID int64) {
	raw, err := readMinioText(ctx, fmt.Sprintf("hls/%d/master.m3u8", videoID))
	if err != nil {
		return
	}
	keep := referencedHLSVersions(raw)
	prefix := fmt.Sprintf("hls/%d/versions/", videoID)
	objectCh := store.ObjectClient().ListObjects(ctx, store.VideoBucket(), minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	removeCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(removeCh)
		for obj := range objectCh {
			if obj.Err != nil {
				log.Printf("list HLS version object failed for video %d: %v", videoID, obj.Err)
				continue
			}
			version := hlsVersionFromObjectKey(videoID, obj.Key)
			if version == "" || keep[version] {
				continue
			}
			removeCh <- obj
		}
	}()
	for result := range store.ObjectClient().RemoveObjects(ctx, store.VideoBucket(), removeCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			log.Printf("delete stale HLS version object %s failed: %v", result.ObjectName, result.Err)
		}
	}
}

func referencedHLSVersions(master string) map[string]bool {
	keep := map[string]bool{}
	for _, entry := range parseMasterPlaylistEntries(master) {
		uriPath := strings.Trim(masterURIPath(entry.uri), "/")
		parts := strings.Split(uriPath, "/")
		if len(parts) >= 3 && parts[0] == "versions" && parts[1] != "" {
			keep[parts[1]] = true
		}
	}
	return keep
}

func hlsVersionFromObjectKey(videoID int64, key string) string {
	prefix := fmt.Sprintf("hls/%d/versions/", videoID)
	rest := strings.TrimPrefix(key, prefix)
	if rest == key {
		return ""
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
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
	availableSet := make(map[string]bool, len(available))
	for _, q := range available {
		availableSet[q.name] = true
	}
	requestedSet := make(map[string]bool, len(requested))
	for _, name := range requested {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if normalized == "" {
			continue
		}
		if !availableSet[normalized] {
			return nil, unavailableTranscodeQualityError(available)
		}
		requestedSet[normalized] = true
	}
	selected := make([]transcodeQuality, 0, len(available))
	for _, q := range available {
		if requestedSet[q.name] {
			selected = append(selected, q)
		}
	}
	if len(selected) == 0 {
		return nil, unavailableTranscodeQualityError(available)
	}
	return selected, nil
}

func unavailableTranscodeQualityError(available []transcodeQuality) error {
	return markNonRetryableTranscodeError(fmt.Errorf("selected transcode qualities are not available for source; available: %s", strings.Join(transcodeQualityNames(available), ", ")))
}

func transcodeQualityNames(qualities []transcodeQuality) []string {
	names := make([]string, 0, len(qualities))
	for _, q := range qualities {
		if q.name != "" {
			names = append(names, q.name)
		}
	}
	return names
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
