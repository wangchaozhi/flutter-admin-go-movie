package video

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const janitorTestMaster = `#EXTM3U
#EXT-X-VERSION:3

#EXT-X-STREAM-INF:BANDWIDTH=1000000,RESOLUTION=640x360
versions/1782252932400535500/360p/index.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720
versions/1782252932400535500/720p/index.m3u8
`

func TestReferencedHLSDirPrefixes(t *testing.T) {
	keep := referencedHLSDirPrefixes(1, janitorTestMaster)
	want := []string{
		"hls/1/versions/1782252932400535500/360p/",
		"hls/1/versions/1782252932400535500/720p/",
	}
	for _, prefix := range want {
		if !keep[prefix] {
			t.Fatalf("expected %q to be kept, keep=%v", prefix, keep)
		}
	}
	if len(keep) != len(want) {
		t.Fatalf("unexpected kept set size: got %d want %d (%v)", len(keep), len(want), keep)
	}
}

func TestHLSObjectReferenced(t *testing.T) {
	keep := referencedHLSDirPrefixes(1, janitorTestMaster)

	referenced := []string{
		"hls/1/versions/1782252932400535500/360p/index.m3u8",
		"hls/1/versions/1782252932400535500/360p/seg_000.ts",
		"hls/1/versions/1782252932400535500/720p/seg_010.ts",
	}
	for _, key := range referenced {
		if !hlsObjectReferenced(key, keep) {
			t.Fatalf("expected %q to be referenced", key)
		}
	}

	// Old flat layout and superseded version dirs must be reapable.
	orphans := []string{
		"hls/1/360p/seg_000.ts",
		"hls/1/480p/index.m3u8",
		"hls/1/versions/1782252628769074000/480p/seg_001.ts",
	}
	for _, key := range orphans {
		if hlsObjectReferenced(key, keep) {
			t.Fatalf("expected %q to be unreferenced", key)
		}
	}
}

func TestPruneOrphanTempDirs(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-2 * orphanCacheAge)

	staleTranscode := filepath.Join(root, "transcode_3_2587783449")
	staleTracks := filepath.Join(root, "tracks_4_123")
	freshTranscode := filepath.Join(root, "transcode_5_fresh")
	unrelated := filepath.Join(root, "source-cache")
	for _, dir := range []string{staleTranscode, staleTracks, freshTranscode, unrelated} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustChtimes(t, staleTranscode, old)
	mustChtimes(t, staleTracks, old)

	pruneOrphanTempDirs(root)

	assertRemoved(t, staleTranscode)
	assertRemoved(t, staleTracks)
	assertExists(t, freshTranscode) // too recent: could be an in-progress job
	assertExists(t, unrelated)      // not a transcode temp dir
}

func TestPruneOrphanSourceCaches(t *testing.T) {
	tmpRoot := t.TempDir()
	root := sourceCacheRoot(tmpRoot)

	live := filepath.Join(root, "1")
	deleted := filepath.Join(root, "3")
	junk := filepath.Join(root, "not-a-number")
	for _, dir := range []string{live, deleted, junk} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	pruneOrphanSourceCaches(tmpRoot, map[int64]bool{1: true})

	assertExists(t, live)
	assertRemoved(t, deleted)
	assertExists(t, junk) // non-numeric dirs are ignored, never deleted
}

func mustChtimes(t *testing.T, path string, ts time.Time) {
	t.Helper()
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
}

func assertRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, err=%v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist, err=%v", path, err)
	}
}
