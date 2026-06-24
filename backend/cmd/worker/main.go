package main

import (
	"context"
	"log"

	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/store"
	"flutter-admin-go/internal/video"

	"github.com/hibiken/asynq"
)

func main() {
	cfg := config.Load()
	if err := store.Init(cfg); err != nil {
		log.Fatal(err)
	}

	janitorCtx, stopJanitor := context.WithCancel(context.Background())
	defer stopJanitor()
	video.StartJanitor(janitorCtx)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr},
		asynq.Config{Concurrency: cfg.Worker.TranscodeConcurrency},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(video.TypeTranscode, video.HandleTranscodeTask)
	mux.HandleFunc(video.TypeExtractTracks, video.HandleExtractTracksTask)

	log.Printf("worker started, env=%s redis=%s concurrency=%d encoder=%s temp_dir=%s", cfg.Env, cfg.Redis.Addr, cfg.Worker.TranscodeConcurrency, cfg.Worker.TranscodeVideoEncoder, cfg.Worker.TranscodeTempDir)
	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}
