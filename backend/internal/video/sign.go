package video

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"flutter-admin-go/internal/config"
)

func hlsSecret() string {
	return config.Load().Video.HLSSecret
}

func configuredVideoBaseURL() string {
	return config.Load().Video.VideoBaseURL
}

func videoBaseURL(r *http.Request) string {
	if s := configuredVideoBaseURL(); s != "" {
		return s
	}

	scheme := "http"
	if forwardedProto := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	} else if r.TLS != nil {
		scheme = "https"
	}

	host := firstForwardedValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	if strings.Contains(hostname, ":") && !strings.HasPrefix(hostname, "[") {
		hostname = "[" + hostname + "]"
	}
	return fmt.Sprintf("%s://%s:8081", scheme, hostname)
}

func firstForwardedValue(value string) string {
	if value == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(value, ",")[0])
}

func apiBaseURL() string {
	return config.Load().Video.APIBaseURL
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
