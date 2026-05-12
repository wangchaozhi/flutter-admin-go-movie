package main

import (
	"log"
	"net/http"

	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/server"
	"flutter-admin-go/internal/store"
)

func main() {
	cfg := config.Load()
	if err := store.Init(cfg); err != nil {
		log.Fatal(err)
	}

	handler := server.NewRouter()
	log.Printf("server started, env=%s addr=%s", cfg.Env, cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		log.Fatal(err)
	}
}
