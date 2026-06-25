package server

import (
	"log/slog"
	"os"
	"strings"

	"flutter-admin-go/internal/config"
)

// SetupLogging installs a process-wide slog default tuned to the environment:
// human-friendly text on stderr for local/dev, structured JSON on stdout for
// prod (so log shippers can parse it). LOG_LEVEL (debug|info|warn|error)
// overrides the default level when set.
func SetupLogging(env config.Env) {
	level := slog.LevelInfo
	if env != config.EnvProd {
		level = slog.LevelDebug
	}
	if v := strings.TrimSpace(os.Getenv("LOG_LEVEL")); v != "" {
		switch strings.ToLower(v) {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if env == config.EnvProd {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}
