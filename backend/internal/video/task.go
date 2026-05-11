package video

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
)

const TypeTranscode = "video:transcode"

type TranscodePayload struct {
	VideoID int64 `json:"video_id"`
	TaskID  int64 `json:"task_id"`
}

func NewTranscodeTask(videoID, taskID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(TranscodePayload{VideoID: videoID, TaskID: taskID})
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

func EnqueueTranscode(ctx context.Context, videoID, taskID int64) error {
	task, err := NewTranscodeTask(videoID, taskID)
	if err != nil {
		return err
	}
	client := AsynqClient()
	defer client.Close()
	_, err = client.EnqueueContext(ctx, task)
	return err
}
