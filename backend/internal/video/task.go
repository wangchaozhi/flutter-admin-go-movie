package video

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const TypeTranscode = "video:transcode"
const TypeExtractTracks = "video:extract_tracks"

const transcodeTaskTimeout = 6 * time.Hour
const transcodeTaskMaxRetry = 2

const extractTracksTaskTimeout = 2 * time.Hour
const extractTracksTaskMaxRetry = 1

type TranscodePayload struct {
	VideoID        int64    `json:"video_id"`
	TaskID         int64    `json:"task_id"`
	BatchID        int64    `json:"batch_id,omitempty"`
	Quality        string   `json:"quality,omitempty"`
	Qualities      []string `json:"qualities,omitempty"`
	MergeExisting  bool     `json:"merge_existing,omitempty"`
	PreviousStatus string   `json:"previous_status,omitempty"`
}

func NewTranscodeTask(videoID, taskID, batchID int64, quality string, mergeExisting bool, previousStatus string) (*asynq.Task, error) {
	payload, err := json.Marshal(TranscodePayload{
		VideoID:        videoID,
		TaskID:         taskID,
		BatchID:        batchID,
		Quality:        quality,
		MergeExisting:  mergeExisting,
		PreviousStatus: previousStatus,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeTranscode, payload), nil
}

func ParseTranscodePayload(t *asynq.Task) (*TranscodePayload, error) {
	var p TranscodePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func AsynqClient() *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr()})
}

func EnqueueTranscode(ctx context.Context, videoID, taskID, batchID int64, quality string, mergeExisting bool, previousStatus string) error {
	task, err := NewTranscodeTask(videoID, taskID, batchID, quality, mergeExisting, previousStatus)
	if err != nil {
		return err
	}
	client := AsynqClient()
	defer client.Close()
	_, err = client.EnqueueContext(ctx, task, asynq.Timeout(transcodeTaskTimeout), asynq.MaxRetry(transcodeTaskMaxRetry))
	return err
}

type ExtractTracksPayload struct {
	VideoID   int64  `json:"video_id"`
	SourceKey string `json:"source_key"`
}

func NewExtractTracksTask(videoID int64, sourceKey string) (*asynq.Task, error) {
	payload, err := json.Marshal(ExtractTracksPayload{
		VideoID:   videoID,
		SourceKey: sourceKey,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExtractTracks, payload), nil
}

func ParseExtractTracksPayload(t *asynq.Task) (*ExtractTracksPayload, error) {
	var p ExtractTracksPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// EnqueueExtractTracks schedules background extraction of the audio/subtitle
// tracks for a freshly uploaded source. The video is expected to already be in
// the "extracting" status when this is called.
func EnqueueExtractTracks(ctx context.Context, videoID int64, sourceKey string) error {
	task, err := NewExtractTracksTask(videoID, sourceKey)
	if err != nil {
		return err
	}
	client := AsynqClient()
	defer client.Close()
	_, err = client.EnqueueContext(ctx, task, asynq.Timeout(extractTracksTaskTimeout), asynq.MaxRetry(extractTracksTaskMaxRetry))
	return err
}
