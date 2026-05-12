package video

import (
	"flutter-admin-go/internal/config"
)

func redisAddr() string {
	return config.Load().Redis.Addr
}
