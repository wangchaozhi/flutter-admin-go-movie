package video

import "testing"

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
