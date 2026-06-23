package video

import (
	"strings"
	"testing"
)

func TestBuildTranscodeArgsLibx264(t *testing.T) {
	args := buildTranscodeArgs(testEncoder("libx264"), testQuality(), "source.mp4", "seg_%03d.ts", "index.m3u8")

	assertHasArgSequence(t, args, "-c:v", "libx264")
	assertHasArgSequence(t, args, "-map", "0:v:0")
	assertHasArgSequence(t, args, "-map", "0:a:0?")
	assertHasArgSequence(t, args, "-sn")
	assertHasArgSequence(t, args, "-vf", "scale=-2:720,format=yuv420p")
	assertHasArgSequence(t, args, "-preset", "veryfast")
	assertHasArgSequence(t, args, "-keyint_min", "180")
	assertHasArgSequence(t, args, "-sc_threshold", "0")
	assertHasArgSequence(t, args, "-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "128k")
	assertHasArgSequence(t, args, "-hls_flags", "independent_segments")
}

func TestBuildTranscodeArgsNVENC(t *testing.T) {
	args := buildTranscodeArgs(testEncoder("h264_nvenc"), testQuality(), "source.mp4", "seg_%03d.ts", "index.m3u8")

	assertHasArgSequence(t, args, "-c:v", "h264_nvenc")
	assertHasArgSequence(t, args, "-preset", "fast")
	assertHasArgSequence(t, args, "-force_key_frames", "expr:gte(t,n_forced*6)")
	assertMissingArg(t, args, "-keyint_min")
	assertMissingArg(t, args, "-sc_threshold")
}

func TestBuildTranscodeArgsVAAPI(t *testing.T) {
	args := buildTranscodeArgs(
		transcodeEncoder{name: "h264_vaapi", vaapiDevice: "/dev/dri/renderD128"},
		testQuality(),
		"source.mp4",
		"seg_%03d.ts",
		"index.m3u8",
	)

	assertHasArgSequence(t, args, "-vaapi_device", "/dev/dri/renderD128")
	assertHasArgSequence(t, args, "-vf", "scale=-2:720,format=nv12,hwupload")
	assertHasArgSequence(t, args, "-c:v", "h264_vaapi")
}

func TestSelectTranscodeQualitiesLandscape(t *testing.T) {
	qualities := selectTranscodeQualities(sourceVideoSize{width: 1920, height: 1080})
	want := []struct {
		name string
		res  string
	}{
		{"360p", "640x360"},
		{"480p", "854x480"},
		{"720p", "1280x720"},
		{"1080p", "1920x1080"},
	}
	assertQualities(t, qualities, want)
}

func TestSelectTranscodeQualitiesPortrait(t *testing.T) {
	qualities := selectTranscodeQualities(sourceVideoSize{width: 1080, height: 1920})
	want := []struct {
		name string
		res  string
	}{
		{"360p", "202x360"},
		{"480p", "270x480"},
		{"720p", "406x720"},
		{"1080p", "608x1080"},
	}
	assertQualities(t, qualities, want)
}

func TestSelectTranscodeQualitiesBelow360p(t *testing.T) {
	qualities := selectTranscodeQualities(sourceVideoSize{width: 426, height: 240})
	want := []struct {
		name string
		res  string
	}{
		{"240p", "426x240"},
	}
	assertQualities(t, qualities, want)
}

func TestSelectRequestedTranscodeQualities(t *testing.T) {
	qualities, err := selectRequestedTranscodeQualities(
		sourceVideoSize{width: 1920, height: 1080},
		[]string{"720p", "1080p"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name string
		res  string
	}{
		{"720p", "1280x720"},
		{"1080p", "1920x1080"},
	}
	assertQualities(t, qualities, want)
}

func TestSelectRequestedTranscodeQualitiesRejectsUnavailable(t *testing.T) {
	_, err := selectRequestedTranscodeQualities(
		sourceVideoSize{width: 1280, height: 720},
		[]string{"1080p"},
	)
	if err == nil {
		t.Fatal("expected unavailable quality error")
	}
}

func TestNormalizeTranscodeQualityNamesAllMeansAuto(t *testing.T) {
	qualities, err := normalizeTranscodeQualityNames([]string{"360p", "480p", "720p", "1080p"})
	if err != nil {
		t.Fatal(err)
	}
	if qualities != nil {
		t.Fatalf("qualities = %v, want nil auto mode", qualities)
	}
}

func TestNormalizeTranscodeQualityNamesRejectsUnknown(t *testing.T) {
	_, err := normalizeTranscodeQualityNames([]string{"4k"})
	if err == nil {
		t.Fatal("expected unknown quality error")
	}
}

func TestBuildMasterPlaylistMergesExistingQualities(t *testing.T) {
	existing := "#EXTM3U\n#EXT-X-VERSION:3\n\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720\n" +
		"720p/index.m3u8\n\n"

	got := buildMasterPlaylist(existing, []transcodeQuality{
		{name: "360p", bandwidth: "1000000", res: "640x360"},
	})

	assertContains(t, got, "360p/index.m3u8")
	assertContains(t, got, "720p/index.m3u8")
	if strings.Index(got, "360p/index.m3u8") > strings.Index(got, "720p/index.m3u8") {
		t.Fatalf("master playlist should be sorted by quality height:\n%s", got)
	}
}

func TestBuildMasterPlaylistOverridesExistingQuality(t *testing.T) {
	existing := "#EXTM3U\n#EXT-X-VERSION:3\n\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=1,RESOLUTION=1x1\n" +
		"720p/index.m3u8\n\n"

	got := buildMasterPlaylist(existing, []transcodeQuality{
		{name: "720p", bandwidth: "2800000", res: "1280x720"},
	})

	assertContains(t, got, "BANDWIDTH=2800000,RESOLUTION=1280x720")
	if strings.Contains(got, "BANDWIDTH=1,RESOLUTION=1x1") {
		t.Fatalf("master playlist kept stale quality metadata:\n%s", got)
	}
}

func TestParseMasterPlaylistEntriesHandlesSignedAbsoluteURIs(t *testing.T) {
	raw := "#EXTM3U\n#EXT-X-VERSION:3\n\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=1000000,RESOLUTION=640x360\n" +
		"https://example.test/api/hls/10/360p/index.m3u8?expires=1&sign=x\n\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720\n" +
		"720p\\index.m3u8\n\n"

	entries := parseMasterPlaylistEntries(raw)
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2: %#v", len(entries), entries)
	}
	if entries[0].name != "360p" || entries[1].name != "720p" {
		t.Fatalf("entries = %#v, want 360p and 720p", entries)
	}
}

func TestParseAvailableTranscodeQualityNames(t *testing.T) {
	got := parseAvailableTranscodeQualityNames("selected transcode qualities are not available for source; available: 360p, 480p, 720p")
	if strings.Join(got, ",") != "360p,480p,720p" {
		t.Fatalf("available qualities = %#v", got)
	}
}

func assertQualities(t *testing.T, got []transcodeQuality, want []struct {
	name string
	res  string
}) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("quality count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i, q := range got {
		if q.name != want[i].name || q.res != want[i].res {
			t.Fatalf("quality[%d] = %s %s, want %s %s", i, q.name, q.res, want[i].name, want[i].res)
		}
	}
}

func testEncoder(name string) transcodeEncoder {
	return transcodeEncoder{name: name}
}

func testQuality() transcodeQuality {
	return transcodeQuality{
		name:     "720p",
		height:   720,
		scale:    "-2:720",
		videoBit: "2500k",
		audioBit: "128k",
	}
}

func assertHasArgSequence(t *testing.T, args []string, want ...string) {
	t.Helper()
	for i := 0; i <= len(args)-len(want); i++ {
		found := true
		for j, item := range want {
			if args[i+j] != item {
				found = false
				break
			}
		}
		if found {
			return
		}
	}
	t.Fatalf("args missing sequence %v in %v", want, args)
}

func assertMissingArg(t *testing.T, args []string, want string) {
	t.Helper()
	for _, arg := range args {
		if arg == want {
			t.Fatalf("args should not include %q: %v", want, args)
		}
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in:\n%s", want, got)
	}
}
