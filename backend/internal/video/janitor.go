package video

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/store"

	"github.com/minio/minio-go/v7"
)

// janitorInterval is how often the background cleanup runs after its first pass.
const janitorInterval = time.Hour

// janitorStartupDelay lets the worker settle before the first sweep.
const janitorStartupDelay = time.Minute

// orphanCacheAge is the safety window before disk temp dirs and MinIO HLS
// objects are eligible for deletion. It must exceed a transcode's maximum
// lifetime so an in-progress job is never touched. staleTranscodeTaskAge is
// transcodeTaskTimeout + 30m, which already satisfies that.
const orphanCacheAge = staleTranscodeTaskAge

// StartJanitor launches a background goroutine on the worker that periodically
// removes orphaned transcode caches left by deleted videos and crashed jobs,
// both on local disk and in MinIO. It runs once shortly after start, then every
// janitorInterval, and stops when ctx is cancelled.
func StartJanitor(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(janitorStartupDelay):
		}
		runJanitorCycle(ctx)

		ticker := time.NewTicker(janitorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runJanitorCycle(ctx)
			}
		}
	}()
}

func runJanitorCycle(ctx context.Context) {
	existing, err := existingVideoIDs(ctx)
	if err != nil {
		log.Printf("janitor: load video ids failed, skipping cycle: %v", err)
		return
	}
	tmpRoot := strings.TrimSpace(config.Load().Worker.TranscodeTempDir)
	pruneOrphanTempDirs(tmpRoot)
	pruneOrphanSourceCaches(tmpRoot, existing)
	pruneOrphanMinioObjects(ctx, existing)
	reapStaleExtractingVideos(ctx)
}

// existingVideoIDs returns the set of video IDs that still exist in the database.
func existingVideoIDs(ctx context.Context) (map[int64]bool, error) {
	var ids []int64
	if err := store.DB().WithContext(ctx).Model(&store.Video{}).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// pruneOrphanTempDirs removes transcode_*/tracks_* working dirs left behind when
// the worker was killed mid-job. Live jobs clean up via defer; anything older
// than orphanCacheAge belongs to a job that has exceeded its maximum lifetime.
func pruneOrphanTempDirs(tmpRoot string) {
	root := tmpRoot
	if root == "" {
		root = os.TempDir()
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-orphanCacheAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "transcode_") && !strings.HasPrefix(name, "tracks_") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		dir := filepath.Join(root, name)
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("janitor: remove orphan temp dir %s failed: %v", dir, err)
			continue
		}
		log.Printf("janitor: removed orphan temp dir %s", dir)
	}
}

// pruneOrphanSourceCaches removes cached source files for videos that no longer
// exist in the database. Caches for live videos are left for reuse (and still
// age-pruned separately by maybePruneSourceCacheRoot).
func pruneOrphanSourceCaches(tmpRoot string, existing map[int64]bool) {
	root := sourceCacheRoot(tmpRoot)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || existing[id] {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("janitor: remove orphan source cache %s failed: %v", dir, err)
			continue
		}
		log.Printf("janitor: removed orphan source cache for deleted video %d", id)
	}
}

// pruneOrphanMinioObjects removes MinIO objects for deleted videos and prunes
// unreferenced HLS objects for videos that still exist.
func pruneOrphanMinioObjects(ctx context.Context, existing map[int64]bool) {
	for _, prefix := range []string{"originals/", "hls/", "covers/"} {
		for _, id := range listVideoIDDirs(ctx, prefix) {
			if existing[id] {
				if prefix == "hls/" {
					pruneUnreferencedHLSObjects(ctx, id)
				}
				continue
			}
			full := fmt.Sprintf("%s%d/", prefix, id)
			log.Printf("janitor: removing MinIO objects for deleted video: %s", full)
			removeObjectsByPrefix(ctx, full)
		}
	}
}

// listVideoIDDirs lists the immediate {id}/ subdirectories under a top-level
// prefix (e.g. "hls/") and returns the parsed video IDs.
func listVideoIDDirs(ctx context.Context, prefix string) []int64 {
	var ids []int64
	seen := map[int64]bool{}
	for obj := range store.ObjectClient().ListObjects(ctx, store.VideoBucket(), minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	}) {
		if obj.Err != nil {
			log.Printf("janitor: list %s failed: %v", prefix, obj.Err)
			continue
		}
		rest := strings.Trim(strings.TrimPrefix(obj.Key, prefix), "/")
		id, err := strconv.ParseInt(rest, 10, 64)
		if err != nil || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// pruneUnreferencedHLSObjects deletes objects under hls/{id}/ that the current
// master.m3u8 no longer points to (old flat-layout dirs and superseded version
// dirs), while preserving master.m3u8 and the tracks/ subtree (served from the
// DB, not the master). It is guarded against deleting an in-progress transcode:
// it skips videos with active tasks and only removes objects older than
// orphanCacheAge, so a freshly uploaded version is never reaped before its
// master entry is written.
func pruneUnreferencedHLSObjects(ctx context.Context, videoID int64) {
	if active, err := activeTranscodeTasks(videoID); err != nil || len(active) > 0 {
		return
	}

	masterKey := fmt.Sprintf("hls/%d/master.m3u8", videoID)
	raw, err := readMinioText(ctx, masterKey)
	if err != nil {
		// No readable master: don't risk deleting anything for this video.
		return
	}
	keepDirs := referencedHLSDirPrefixes(videoID, raw)
	base := fmt.Sprintf("hls/%d/", videoID)
	tracksPrefix := base + "tracks/"
	cutoff := time.Now().Add(-orphanCacheAge)

	objectCh := store.ObjectClient().ListObjects(ctx, store.VideoBucket(), minio.ListObjectsOptions{
		Prefix:    base,
		Recursive: true,
	})
	removeCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(removeCh)
		for obj := range objectCh {
			if obj.Err != nil {
				log.Printf("janitor: list hls object failed for video %d: %v", videoID, obj.Err)
				continue
			}
			if obj.Key == masterKey || strings.HasPrefix(obj.Key, tracksPrefix) {
				continue
			}
			if obj.LastModified.After(cutoff) || hlsObjectReferenced(obj.Key, keepDirs) {
				continue
			}
			removeCh <- obj
		}
	}()
	for result := range store.ObjectClient().RemoveObjects(ctx, store.VideoBucket(), removeCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			log.Printf("janitor: delete unreferenced hls object %s failed: %v", result.ObjectName, result.Err)
			continue
		}
		log.Printf("janitor: removed unreferenced hls object %s", result.ObjectName)
	}
}

// referencedHLSDirPrefixes returns the set of hls/{id}/<dir>/ prefixes that the
// master playlist actively references, for both versioned and flat layouts.
func referencedHLSDirPrefixes(videoID int64, master string) map[string]bool {
	base := fmt.Sprintf("hls/%d/", videoID)
	keep := map[string]bool{}
	for _, entry := range parseMasterPlaylistEntries(master) {
		uriPath := strings.Trim(masterURIPath(entry.uri), "/")
		if uriPath == "" {
			continue
		}
		dir := path.Dir(uriPath)
		if dir == "." || dir == "" {
			continue
		}
		keep[base+dir+"/"] = true
	}
	return keep
}

func hlsObjectReferenced(key string, keepDirs map[string]bool) bool {
	for prefix := range keepDirs {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
