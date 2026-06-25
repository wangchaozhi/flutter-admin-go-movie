package main

import (
	"log/slog"
	"net/http"
	"os"

	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/server"
	"flutter-admin-go/internal/store"
)

func main() {
	cfg := config.Load()
	server.SetupLogging(cfg.Env)
	if err := store.Init(cfg); err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}

	handler := server.NewRouter()
	slog.Info("server started", "env", cfg.Env, "addr", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
