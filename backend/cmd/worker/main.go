package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"flutter-admin-go/internal/store"
	"flutter-admin-go/internal/video"

	"github.com/hibiken/asynq"
)

func main() {
	if err := store.Init(""); err != nil {
		log.Fatal(err)
	}

	redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	concurrency := 2
	if s := strings.TrimSpace(os.Getenv("TRANSCODE_CONCURRENCY")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			concurrency = n
		}
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{Concurrency: concurrency},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(video.TypeTranscode, video.HandleTranscodeTask)

	log.Printf("worker started, concurrency=%d", concurrency)
	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}
