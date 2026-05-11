package video

import (
	"os"
	"strings"
)

func redisAddr() string {
	s := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if s == "" {
		return "localhost:6379"
	}
	return s
}
