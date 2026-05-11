package video

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"

	"github.com/minio/minio-go/v7"
)

type HLSQualityOption struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Resolution string `json:"resolution,omitempty"`
	URL        string `json:"url"`
}

// GET /api/hls/{videoId}/master.m3u8?expires=xxx&sign=xxx
func HLSMasterHandler(w http.ResponseWriter, r *http.Request) {
	videoID, path, ok := parseHLSRequest(r, w)
	if !ok {
		return
	}
	if !verifyHLSSign(r, path, w) {
		return
	}

	masterKey := fmt.Sprintf("hls/%d/master.m3u8", videoID)
	raw, err := readMinioText(r.Context(), masterKey)
	if err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, ".m3u8") && !strings.HasPrefix(line, "#") {
			quality := strings.TrimSuffix(line, "/index.m3u8")
			quality = strings.TrimSuffix(quality, "index.m3u8")
			quality = strings.Trim(quality, "/")
			subPath := fmt.Sprintf("/api/hls/%d/%s/index.m3u8", videoID, quality)
			signed := SignPath(subPath, 1800)
			buf.WriteString(signed + "\n")
		} else {
			buf.WriteString(line + "\n")
		}
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// GET /api/hls/{videoId}/{quality}/index.m3u8?expires=xxx&sign=xxx
func HLSIndexHandler(w http.ResponseWriter, r *http.Request) {
	videoID, path, ok := parseHLSRequest(r, w)
	if !ok {
		return
	}
	if !verifyHLSSign(r, path, w) {
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/hls/%d/", videoID)), "/")
	if len(parts) < 2 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid path"})
		return
	}
	quality := parts[0]

	indexKey := fmt.Sprintf("hls/%d/%s/index.m3u8", videoID, quality)
	raw, err := readMinioText(r.Context(), indexKey)
	if err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, ".ts") && !strings.HasPrefix(line, "#") {
			tsPath := fmt.Sprintf("/hls/%d/%s/%s", videoID, quality, line)
			signed := SignPath(tsPath, 1800)
			buf.WriteString(videoBaseURL(r) + signed + "\n")
		} else {
			buf.WriteString(line + "\n")
		}
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// GET /api/videos/{id}/play  (VIP videos require mobile JWT)
func AppPlayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/videos/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}

	var v store.Video
	if err := store.DB().First(&v, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	if v.Status != "ready" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "video not ready"})
		return
	}

	// VIP-only videos require a valid mobile token
	if v.IsVip && !v.IsFree {
		if _, ok := parseMobileAuth(r); !ok {
			common.WriteJSON(w, http.StatusForbidden, common.APIResponse{Code: 403, Msg: "vip required"})
			return
		}
	}

	masterPath := fmt.Sprintf("/api/hls/%d/master.m3u8", id)
	signedPath := SignPath(masterPath, 1800)
	qualities := signedHLSQualities(r.Context(), id)

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"video_id":   id,
		"type":       "hls",
		"url":        signedPath,
		"qualities":  qualities,
		"auto_label": "自动",
	}})
}

func signedHLSQualities(ctx context.Context, videoID int64) []HLSQualityOption {
	raw, err := readMinioText(ctx, fmt.Sprintf("hls/%d/master.m3u8", videoID))
	if err != nil {
		return nil
	}

	var qualities []HLSQualityOption
	var resolution string
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			resolution = parseStreamResolution(line)
			continue
		}
		if strings.HasPrefix(line, "#") || !strings.HasSuffix(line, ".m3u8") {
			continue
		}
		name := strings.TrimSuffix(line, "/index.m3u8")
		name = strings.TrimSuffix(name, "index.m3u8")
		name = strings.Trim(name, "/")
		if name == "" {
			resolution = ""
			continue
		}
		subPath := fmt.Sprintf("/api/hls/%d/%s/index.m3u8", videoID, name)
		qualities = append(qualities, HLSQualityOption{
			Name:       name,
			Label:      qualityLabel(name, resolution),
			Resolution: resolution,
			URL:        SignPath(subPath, 1800),
		})
		resolution = ""
	}
	return qualities
}

func parseStreamResolution(line string) string {
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "RESOLUTION=") {
			return strings.TrimPrefix(part, "RESOLUTION=")
		}
	}
	return ""
}

func qualityLabel(name, resolution string) string {
	if name != "" {
		return name
	}
	if resolution == "" {
		return "清晰度"
	}
	parts := strings.Split(resolution, "x")
	if len(parts) == 2 && parts[1] != "" {
		return parts[1] + "p"
	}
	return resolution
}

// GET /api/videos/{id}/cover  — proxy cover image from MinIO (no auth needed)
func AppCoverHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/videos/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var v store.Video
	if store.DB().First(&v, id).Error != nil || v.CoverKey == "" {
		http.NotFound(w, r)
		return
	}
	obj, err := store.ObjectClient().GetObject(r.Context(), store.VideoBucket(), v.CoverKey, minio.GetObjectOptions{})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer obj.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, obj)
}

// AppVideoItem is the app-facing video response, extending Video with cover_url and category_name.
type AppVideoItem struct {
	store.Video
	CoverURL     string `json:"cover_url"`
	CategoryName string `json:"category_name"`
}

func coverURL(v store.Video) string {
	if v.CoverKey == "" {
		return ""
	}
	return "/api/videos/" + strconv.FormatInt(v.ID, 10) + "/cover"
}

// GET /api/videos?category_id=1&page=1&per_page=20
func AppListVideosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	categoryID, _ := strconv.Atoi(q.Get("category_id"))

	db := store.DB().Where("status = ?", "ready")
	if categoryID > 0 {
		db = db.Where("category_id = ?", categoryID)
	}

	var total int64
	db.Model(&store.Video{}).Count(&total)

	var videos []store.Video
	db.Order("id desc").Offset((page - 1) * perPage).Limit(perPage).Find(&videos)

	// load category names in one query
	catNames := map[int]string{}
	if len(videos) > 0 {
		var cats []store.Category
		store.DB().Find(&cats)
		for _, c := range cats {
			catNames[c.ID] = c.Name
		}
	}

	items := make([]AppVideoItem, len(videos))
	for i, v := range videos {
		items[i] = AppVideoItem{
			Video:        v,
			CoverURL:     coverURL(v),
			CategoryName: catNames[v.CategoryID],
		}
	}

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"total":    total,
		"page":     page,
		"per_page": perPage,
		"items":    items,
	}})
}

// GET /api/videos/{id}
func AppGetVideoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/videos/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	var v store.Video
	if err := store.DB().First(&v, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	var catName string
	if v.CategoryID > 0 {
		var cat store.Category
		if store.DB().First(&cat, v.CategoryID).Error == nil {
			catName = cat.Name
		}
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: AppVideoItem{
		Video:        v,
		CoverURL:     coverURL(v),
		CategoryName: catName,
	}})
}

func parseHLSRequest(r *http.Request, w http.ResponseWriter) (int64, string, bool) {
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/hls/")
	parts := strings.SplitN(trimmed, "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return 0, "", false
	}
	return id, path, true
}

func verifyHLSSign(r *http.Request, path string, w http.ResponseWriter) bool {
	expiresStr := r.URL.Query().Get("expires")
	sign := r.URL.Query().Get("sign")
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil || !VerifySign(path, expires, sign) {
		common.WriteJSON(w, http.StatusForbidden, common.APIResponse{Code: 403, Msg: "invalid or expired signature"})
		return false
	}
	return true
}

func readMinioText(ctx context.Context, key string) (string, error) {
	obj, err := store.ObjectClient().GetObject(ctx, store.VideoBucket(), key, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(obj)
	return buf.String(), err
}

// parseMobileAuth extracts and validates the Bearer JWT from the request,
// returning the user ID. Returns 0 and false on failure.
func parseMobileAuth(r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	raw = strings.TrimPrefix(raw, "Bearer ")
	if raw == "" {
		return 0, false
	}
	claims, err := admin.ParseMobileToken(raw)
	if err != nil {
		return 0, false
	}
	return claims.UserID, true
}

// POST /api/videos/{id}/progress  — upsert play position (mobile auth required)
// GET  /api/videos/{id}/progress  — read last position (mobile auth required)
func AppProgressHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/videos/"), "/")
	videoID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		var rec store.VideoPlayRecord
		err := store.DB().Where("user_id = ? AND video_id = ?", userID, videoID).First(&rec).Error
		if err != nil {
			common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]int{"position": 0}})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]int{"position": rec.Position}})

	case http.MethodPost:
		var req struct {
			Position int `json:"position"`
			Duration int `json:"duration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}
		var rec store.VideoPlayRecord
		store.DB().Where("user_id = ? AND video_id = ?", userID, videoID).First(&rec)
		rec.UserID = int64(userID)
		rec.VideoID = videoID
		rec.Position = req.Position
		if req.Duration > 0 {
			rec.Duration = req.Duration
		}
		if rec.ID == 0 {
			store.DB().Create(&rec)
		} else {
			store.DB().Save(&rec)
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})

	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}
