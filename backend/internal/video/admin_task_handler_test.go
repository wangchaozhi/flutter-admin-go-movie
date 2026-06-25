package video

import (
	"testing"

	"flutter-admin-go/internal/store"
)

func TestDisplayTranscodeTasksByQualityUsesPreviousSuccessForPlayableCanceledTask(t *testing.T) {
	tasks := []store.VideoTranscodeTask{
		{ID: 4, Quality: "1080p", Status: "processing", StatusMessage: "转码 1080p"},
		{ID: 3, Quality: "720p", Status: "canceled", StatusMessage: "已取消", Progress: 100},
		{ID: 2, Quality: "720p", Status: "success", StatusMessage: "完成", Progress: 100},
	}

	got := displayTranscodeTasksByQuality(tasks, map[string]bool{"720p": true})

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Quality != "720p" || got[0].ID != 2 || got[0].Status != "success" {
		t.Fatalf("720p task = %#v, want previous success task", got[0])
	}
	if got[1].Quality != "1080p" || got[1].ID != 4 || got[1].Status != "processing" {
		t.Fatalf("1080p task = %#v, want latest processing task", got[1])
	}
}

func TestDisplayTranscodeTasksByQualityKeepsCanceledWhenNotPlayable(t *testing.T) {
	tasks := []store.VideoTranscodeTask{
		{ID: 2, Quality: "720p", Status: "canceled", StatusMessage: "已取消", Progress: 100},
		{ID: 1, Quality: "720p", Status: "success", StatusMessage: "完成", Progress: 100},
	}

	got := displayTranscodeTasksByQuality(tasks, map[string]bool{})

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != 2 || got[0].Status != "canceled" {
		t.Fatalf("task = %#v, want latest canceled task", got[0])
	}
}
