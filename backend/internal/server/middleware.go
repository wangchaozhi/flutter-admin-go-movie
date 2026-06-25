package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"flutter-admin-go/internal/common"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// statusRecorder captures the response status and size while delegating to the
// underlying ResponseWriter. It forwards Flush so streaming responses (HLS
// playlists, ranged asset reads) are not buffered by the wrapper.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// withObservability assigns each request a request id (surfaced via the
// X-Request-ID response header and the request context), recovers from panics
// with a 500 instead of crashing the connection, and emits one structured
// access-log line per request. Health checks are logged at Debug so routine
// polling does not flood the default Info output.
func withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := newRequestID()
		w.Header().Set("X-Request-ID", reqID)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, reqID))

		defer func() {
			if rv := recover(); rv != nil {
				if !rec.wrote {
					common.WriteJSON(rec, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "internal server error"})
				}
				slog.Error("panic recovered",
					"request_id", reqID,
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rv,
				)
			}

			level := slog.LevelInfo
			switch {
			case rec.status >= 500:
				level = slog.LevelError
			case rec.status >= 400:
				level = slog.LevelWarn
			case r.URL.Path == "/api/health":
				level = slog.LevelDebug
			}
			slog.LogAttrs(r.Context(), level, "http request",
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.String("ip", common.ClientIP(r)),
				slog.Duration("duration", time.Since(start)),
			)
		}()

		next.ServeHTTP(rec, r)
	})
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req"
	}
	return hex.EncodeToString(b[:])
}

// RequestID returns the request id assigned by withObservability for the given
// context, or "" when called outside a traced request.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}
