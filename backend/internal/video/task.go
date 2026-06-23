package video

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const TypeTranscode = "video:transcode"

const transcodeTaskTimeout = 6 * time.Hour

type TranscodePayload struct {
	VideoID        int64    `json:"video_id"`
	TaskID         int64    `json:"task_id"`
	Quality        string   `json:"quality,omitempty"`
	Qualities      []string `json:"qualities,omitempty"`
	MergeExisting  bool     `json:"merge_existing,omitempty"`
	PreviousStatus string   `json:"previous_status,omitempty"`
}

func NewTranscodeTask(videoID, taskID int64, quality string, mergeExisting bool, previousStatus string) (*asynq.Task, error) {
	payload, err := json.Marshal(TranscodePayload{
		VideoID:        videoID,
		TaskID:         taskID,
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

func EnqueueTranscode(ctx context.Context, videoID, taskID int64, quality string, mergeExisting bool, previousStatus string) error {
	task, err := NewTranscodeTask(videoID, taskID, quality, mergeExisting, previousStatus)
	if err != nil {
		return err
	}
	client := AsynqClient()
	defer client.Close()
	_, err = client.EnqueueContext(ctx, task, asynq.Timeout(transcodeTaskTimeout), asynq.MaxRetry(0))
	return err
}
