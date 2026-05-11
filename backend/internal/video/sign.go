package video

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

func hlsSecret() string {
	s := strings.TrimSpace(os.Getenv("HLS_SECRET"))
	if s == "" {
		return "dev_secret"
	}
	return s
}

func videoBaseURL() string {
	s := strings.TrimSpace(os.Getenv("VIDEO_BASE_URL"))
	if s == "" {
		return "http://localhost:8081"
	}
	return s
}

func apiBaseURL() string {
	s := strings.TrimSpace(os.Getenv("API_BASE_URL"))
	if s == "" {
		return "http://localhost:8080"
	}
	return s
}

// SignPath returns a path with expires and sign query params.
// Format matches Nginx secure_link_md5: base64url(md5("{expires}{path} {secret}"))
func SignPath(path string, ttlSeconds int64) string {
	expires := time.Now().Unix() + ttlSeconds
	sign := computeSign(path, expires)
	return fmt.Sprintf("%s?expires=%d&sign=%s", path, expires, sign)
}

// VerifySign validates the signature against Nginx secure_link format.
func VerifySign(path string, expires int64, sign string) bool {
	if time.Now().Unix() > expires {
		return false
	}
	return computeSign(path, expires) == sign
}

// computeSign: base64url(md5("{expires}{path} {secret}"))
// Space between path and secret matches Nginx secure_link_md5 directive.
func computeSign(path string, expires int64) string {
	raw := fmt.Sprintf("%d%s %s", expires, path, hlsSecret())
	sum := md5.Sum([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
