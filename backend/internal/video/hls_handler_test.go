package video

import (
	"strings"
	"testing"

	"flutter-admin-go/internal/store"
)

func TestRewriteMasterPlaylistAddsStandardRenditionGroups(t *testing.T) {
	raw := "#EXTM3U\n#EXT-X-VERSION:3\n\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720\n" +
		"720p/index.m3u8\n"

	got := rewriteMasterPlaylist(10, raw, []store.VideoMediaTrack{
		{
			TrackType:      "audio",
			StreamPosition: 0,
			Language:       "eng",
			Title:          "English",
			ObjectKey:      "hls/10/tracks/source/audio/a0/index.m3u8",
		},
		{
			TrackType:      "audio",
			StreamPosition: 1,
			Language:       "jpn",
			Title:          "Japanese",
			ObjectKey:      "hls/10/tracks/source/audio/a1/index.m3u8",
		},
	}, []store.VideoMediaTrack{
		{
			TrackType:      "subtitle",
			StreamPosition: 0,
			Language:       "zho",
			Title:          "Chinese",
			IsDefault:      true,
			ObjectKey:      "hls/10/tracks/source/subtitles/s0.vtt",
		},
	})

	defaultAudioLine := playlistLineContaining(got, `NAME="English"`)
	assertContains(t, defaultAudioLine, `#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="eng",DEFAULT=YES,AUTOSELECT=YES,CHANNELS="2"`)
	if strings.Contains(defaultAudioLine, "URI=") {
		t.Fatalf("in-stream default audio rendition should not include URI:\n%s", defaultAudioLine)
	}
	assertContains(t, got, `#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Japanese",LANGUAGE="jpn",DEFAULT=NO,AUTOSELECT=YES,CHANNELS="2",URI="/api/hls/10/tracks/source/audio/a1/index.m3u8?expires=`)
	assertContains(t, got, `#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="Chinese",LANGUAGE="zho",DEFAULT=YES,AUTOSELECT=YES,FORCED=NO,URI="/api/hls/10/tracks/source/subtitles/s0/index.m3u8?expires=`)
	assertContains(t, got, `#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720,AUDIO="audio",SUBTITLES="subs"`)
	assertContains(t, got, `/api/hls/10/720p/index.m3u8?expires=`)
}

func TestRewriteMasterPlaylistSkipsSingleAudioGroup(t *testing.T) {
	raw := "#EXTM3U\n#EXT-X-VERSION:3\n\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720\n" +
		"720p/index.m3u8\n"

	got := rewriteMasterPlaylist(10, raw, []store.VideoMediaTrack{
		{
			TrackType:      "audio",
			StreamPosition: 0,
			Language:       "eng",
			Title:          "English",
			ObjectKey:      "hls/10/tracks/source/audio/a0/index.m3u8",
		},
	}, nil)

	if strings.Contains(got, "TYPE=AUDIO") || strings.Contains(got, `AUDIO="audio"`) {
		t.Fatalf("single source audio should not create an HLS audio group:\n%s", got)
	}
	assertContains(t, got, `/api/hls/10/720p/index.m3u8?expires=`)
}

func TestSubtitlePlaylistPathRoundTrip(t *testing.T) {
	objectKey := "hls/10/tracks/source/subtitles/s0.vtt"
	playlistRelPath := subtitlePlaylistRelPath(10, objectKey)
	if playlistRelPath != "tracks/source/subtitles/s0/index.m3u8" {
		t.Fatalf("subtitle playlist path = %q", playlistRelPath)
	}
	if got := subtitleVTTObjectKeyForPlaylist(10, playlistRelPath); got != objectKey {
		t.Fatalf("subtitle object key = %q, want %q", got, objectKey)
	}
}

func TestRenderSubtitleMediaPlaylist(t *testing.T) {
	got := renderSubtitleMediaPlaylist(10, store.VideoMediaTrack{
		ObjectKey: "hls/10/tracks/source/subtitles/s0.vtt",
	}, 42)

	assertContains(t, got, "#EXTM3U")
	assertContains(t, got, "#EXT-X-TARGETDURATION:42")
	assertContains(t, got, "#EXTINF:42.000,")
	assertContains(t, got, "/api/hls/10/tracks/source/subtitles/s0.vtt?expires=")
	assertContains(t, got, "#EXT-X-ENDLIST")
}

func playlistLineContaining(playlist, value string) string {
	for _, line := range strings.Split(playlist, "\n") {
		if strings.Contains(line, value) {
			return line
		}
	}
	return ""
}
