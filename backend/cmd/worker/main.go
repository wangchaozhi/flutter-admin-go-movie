package main

import (
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

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr},
		asynq.Config{Concurrency: cfg.Worker.TranscodeConcurrency},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(video.TypeTranscode, video.HandleTranscodeTask)

	log.Printf("worker started, env=%s redis=%s concurrency=%d encoder=%s temp_dir=%s", cfg.Env, cfg.Redis.Addr, cfg.Worker.TranscodeConcurrency, cfg.Worker.TranscodeVideoEncoder, cfg.Worker.TranscodeTempDir)
	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}
