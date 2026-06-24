package video

import "testing"

func TestMediaStreamCounts(t *testing.T) {
	audioCount, subtitleCount := mediaStreamCounts([]ffprobeStreamInfo{
		{CodecType: "video", CodecName: "h264"},
		{CodecType: "audio", CodecName: "aac"},
		{CodecType: "audio", CodecName: "aac"},
		{CodecType: "subtitle", CodecName: "subrip"},
		{CodecType: "subtitle", CodecName: "hdmv_pgs_subtitle"},
	})

	if audioCount != 2 {
		t.Fatalf("audio count = %d, want 2", audioCount)
	}
	if subtitleCount != 1 {
		t.Fatalf("subtitle count = %d, want 1", subtitleCount)
	}
}
