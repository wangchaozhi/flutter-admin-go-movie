package video

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

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

type MediaTrackOption struct {
	ID             int64  `json:"id"`
	Type           string `json:"type"`
	Label          string `json:"label"`
	Language       string `json:"language,omitempty"`
	Title          string `json:"title,omitempty"`
	Codec          string `json:"codec,omitempty"`
	StreamPosition int    `json:"stream_position"`
	Default        bool   `json:"default"`
	Forced         bool   `json:"forced"`
	URL            string `json:"url"`
}

type mediaTrackSummary struct {
	Scanned       bool
	AudioCount    int
	SubtitleCount int
}

const hlsSignedURLTTLSeconds = 6 * 60 * 60
const hlsAudioGroupID = "audio"
const hlsSubtitleGroupID = "subs"

// vipPreviewSeconds is how long a non-VIP viewer may preview a VIP-only video
// before the app shows the paywall (mainstream "first few minutes" trial).
const vipPreviewSeconds = 5 * 60

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

	audioTracks, subtitleTracks := readyMasterMediaTracks(r.Context(), videoID)
	master := rewriteMasterPlaylist(videoID, raw, audioTracks, subtitleTracks)

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(master))
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

	indexRelPath := strings.Trim(strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/hls/%d/", videoID)), "/")
	if !validHLSRelativePath(indexRelPath) || !strings.HasSuffix(indexRelPath, "index.m3u8") {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid path"})
		return
	}

	if track, ok := subtitleTrackForPlaylist(r.Context(), videoID, indexRelPath); ok {
		playlist := renderSubtitleMediaPlaylist(videoID, track, videoDurationSeconds(r.Context(), videoID))
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(playlist))
		return
	}

	indexKey := fmt.Sprintf("hls/%d/%s", videoID, indexRelPath)
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
			tsRelPath := hlsSegmentRelativePath(indexRelPath, line)
			if tsRelPath == "" {
				buf.WriteString(line + "\n")
				continue
			}
			tsPath := fmt.Sprintf("/hls/%d/%s", videoID, tsRelPath)
			signed := SignPath(tsPath, hlsSignedURLTTLSeconds)
			buf.WriteString(videoBaseURL(r) + signed + "\n")
		} else {
			buf.WriteString(line + "\n")
		}
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func readyMasterMediaTracks(ctx context.Context, videoID int64) ([]store.VideoMediaTrack, []store.VideoMediaTrack) {
	var tracks []store.VideoMediaTrack
	err := store.DB().WithContext(ctx).
		Where("video_id = ? AND track_type IN ? AND status = ? AND object_key <> ?", videoID, []string{"audio", "subtitle"}, "ready", "").
		Order("track_type asc, stream_position asc").
		Find(&tracks).Error
	if err != nil {
		return nil, nil
	}
	audioTracks := make([]store.VideoMediaTrack, 0)
	subtitleTracks := make([]store.VideoMediaTrack, 0)
	for _, track := range tracks {
		switch track.TrackType {
		case "audio":
			audioTracks = append(audioTracks, track)
		case "subtitle":
			subtitleTracks = append(subtitleTracks, track)
		}
	}
	return audioTracks, subtitleTracks
}

func rewriteMasterPlaylist(videoID int64, raw string, audioTracks, subtitleTracks []store.VideoMediaTrack) string {
	audioRenditions := renderAudioRenditionTags(videoID, audioTracks)
	subtitleRenditions := renderSubtitleRenditionTags(videoID, subtitleTracks)
	hasAudioGroup := len(audioRenditions) > 0
	hasSubtitleGroup := len(subtitleRenditions) > 0

	var buf bytes.Buffer
	insertedRenditions := false
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if isGeneratedMasterMediaLine(trimmed) {
			continue
		}
		if !insertedRenditions && strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF") {
			writeMasterRenditions(&buf, audioRenditions, subtitleRenditions)
			insertedRenditions = true
		}
		if strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF") {
			line = addStreamInfMediaGroups(line, hasAudioGroup, hasSubtitleGroup)
		}
		if relPath := hlsPlaylistRelativePath(line); relPath != "" {
			subPath := fmt.Sprintf("/api/hls/%d/%s", videoID, relPath)
			buf.WriteString(SignPath(subPath, hlsSignedURLTTLSeconds) + "\n")
		} else {
			buf.WriteString(line + "\n")
		}
	}
	return buf.String()
}

func writeMasterRenditions(buf *bytes.Buffer, audioRenditions, subtitleRenditions []string) {
	for _, line := range audioRenditions {
		buf.WriteString(line + "\n")
	}
	for _, line := range subtitleRenditions {
		buf.WriteString(line + "\n")
	}
	if len(audioRenditions) > 0 || len(subtitleRenditions) > 0 {
		buf.WriteString("\n")
	}
}

func renderAudioRenditionTags(videoID int64, tracks []store.VideoMediaTrack) []string {
	if !hasAlternateAudioTracks(tracks) {
		return nil
	}

	defaultTrack := store.VideoMediaTrack{
		TrackType:      "audio",
		StreamPosition: 0,
		Title:          "Default",
	}
	for _, track := range tracks {
		if track.StreamPosition == 0 {
			defaultTrack = track
			break
		}
	}

	usedNames := map[string]int{}
	renditions := []string{renderMediaRenditionTag("AUDIO", hlsAudioGroupID, uniqueRenditionName(mediaTrackLabel(defaultTrack), usedNames), defaultTrack.Language, true, true, false, "")}
	alternateCount := 0
	for _, track := range tracks {
		if track.StreamPosition == 0 {
			continue
		}
		url := signedMediaTrackURL(videoID, track.ObjectKey)
		if url == "" {
			continue
		}
		renditions = append(renditions, renderMediaRenditionTag("AUDIO", hlsAudioGroupID, uniqueRenditionName(mediaTrackLabel(track), usedNames), track.Language, false, true, false, url))
		alternateCount++
	}
	if alternateCount == 0 {
		return nil
	}
	return renditions
}

func hasAlternateAudioTracks(tracks []store.VideoMediaTrack) bool {
	for _, track := range tracks {
		if track.StreamPosition > 0 {
			return true
		}
	}
	return false
}

func renderSubtitleRenditionTags(videoID int64, tracks []store.VideoMediaTrack) []string {
	if len(tracks) == 0 {
		return nil
	}
	usedNames := map[string]int{}
	renditions := make([]string, 0, len(tracks))
	for _, track := range tracks {
		url := subtitlePlaylistURL(videoID, track.ObjectKey)
		if url == "" {
			continue
		}
		renditions = append(renditions, renderMediaRenditionTag("SUBTITLES", hlsSubtitleGroupID, uniqueRenditionName(mediaTrackLabel(track), usedNames), track.Language, track.IsDefault, true, track.IsForced, url))
	}
	return renditions
}

func renderMediaRenditionTag(mediaType, groupID, name, language string, isDefault, autoselect, forced bool, uri string) string {
	attrs := []string{
		"TYPE=" + mediaType,
		"GROUP-ID=" + hlsQuoteAttribute(groupID),
		"NAME=" + hlsQuoteAttribute(name),
	}
	if strings.TrimSpace(language) != "" {
		attrs = append(attrs, "LANGUAGE="+hlsQuoteAttribute(language))
	}
	attrs = append(attrs,
		"DEFAULT="+hlsBool(isDefault),
		"AUTOSELECT="+hlsBool(autoselect),
	)
	if mediaType == "AUDIO" {
		attrs = append(attrs, "CHANNELS="+hlsQuoteAttribute("2"))
	}
	if mediaType == "SUBTITLES" {
		attrs = append(attrs, "FORCED="+hlsBool(forced))
	}
	if strings.TrimSpace(uri) != "" {
		attrs = append(attrs, "URI="+hlsQuoteAttribute(uri))
	}
	return "#EXT-X-MEDIA:" + strings.Join(attrs, ",")
}

func uniqueRenditionName(name string, used map[string]int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Track"
	}
	used[name]++
	if used[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s %d", name, used[name])
}

func hlsBool(value bool) string {
	if value {
		return "YES"
	}
	return "NO"
}

func hlsQuoteAttribute(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func isGeneratedMasterMediaLine(line string) bool {
	if !strings.HasPrefix(line, "#EXT-X-MEDIA:") {
		return false
	}
	groupID := strings.Trim(parseMasterAttribute(line, "GROUP-ID"), `"`)
	return groupID == hlsAudioGroupID || groupID == hlsSubtitleGroupID
}

func addStreamInfMediaGroups(line string, hasAudioGroup, hasSubtitleGroup bool) string {
	additions := make([]string, 0, 2)
	if hasAudioGroup && parseMasterAttribute(line, "AUDIO") == "" {
		additions = append(additions, "AUDIO="+hlsQuoteAttribute(hlsAudioGroupID))
	}
	if hasSubtitleGroup && parseMasterAttribute(line, "SUBTITLES") == "" {
		additions = append(additions, "SUBTITLES="+hlsQuoteAttribute(hlsSubtitleGroupID))
	}
	if len(additions) == 0 {
		return line
	}
	separator := ","
	if !strings.Contains(line, ":") {
		separator = ":"
	}
	return line + separator + strings.Join(additions, ",")
}

func subtitlePlaylistURL(videoID int64, objectKey string) string {
	relPath := subtitlePlaylistRelPath(videoID, objectKey)
	if relPath == "" {
		return ""
	}
	return SignPath(fmt.Sprintf("/api/hls/%d/%s", videoID, relPath), hlsSignedURLTTLSeconds)
}

func subtitlePlaylistRelPath(videoID int64, objectKey string) string {
	prefix := fmt.Sprintf("hls/%d/", videoID)
	if !strings.HasPrefix(objectKey, prefix) || !strings.HasSuffix(objectKey, ".vtt") {
		return ""
	}
	relPath := strings.TrimPrefix(objectKey, prefix)
	playlistRelPath := strings.TrimSuffix(relPath, ".vtt") + "/index.m3u8"
	if !validHLSRelativePath(playlistRelPath) {
		return ""
	}
	return playlistRelPath
}

func subtitleVTTObjectKeyForPlaylist(videoID int64, playlistRelPath string) string {
	if !strings.HasSuffix(playlistRelPath, "/index.m3u8") {
		return ""
	}
	vttRelPath := strings.TrimSuffix(playlistRelPath, "/index.m3u8") + ".vtt"
	if !validHLSRelativePath(vttRelPath) {
		return ""
	}
	return fmt.Sprintf("hls/%d/%s", videoID, vttRelPath)
}

func subtitleTrackForPlaylist(ctx context.Context, videoID int64, playlistRelPath string) (store.VideoMediaTrack, bool) {
	objectKey := subtitleVTTObjectKeyForPlaylist(videoID, playlistRelPath)
	if objectKey == "" {
		return store.VideoMediaTrack{}, false
	}
	var track store.VideoMediaTrack
	err := store.DB().WithContext(ctx).
		Where("video_id = ? AND track_type = ? AND status = ? AND object_key = ?", videoID, "subtitle", "ready", objectKey).
		First(&track).Error
	return track, err == nil
}

func videoDurationSeconds(ctx context.Context, videoID int64) int {
	var video store.Video
	if err := store.DB().WithContext(ctx).Select("duration").First(&video, videoID).Error; err != nil {
		return 0
	}
	return video.Duration
}

func renderSubtitleMediaPlaylist(videoID int64, track store.VideoMediaTrack, duration int) string {
	if duration <= 0 {
		duration = 1
	}
	vttURL := signedMediaTrackURL(videoID, track.ObjectKey)
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", duration))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", float64(duration)))
	b.WriteString(vttURL + "\n")
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// GET /api/hls/{videoId}/tracks/{...}.vtt?expires=xxx&sign=xxx
func HLSAssetHandler(w http.ResponseWriter, r *http.Request) {
	videoID, requestPath, ok := parseHLSRequest(r, w)
	if !ok {
		return
	}
	if !verifyHLSSign(r, requestPath, w) {
		return
	}

	relPath := strings.Trim(strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/hls/%d/", videoID)), "/")
	if !validHLSRelativePath(relPath) || !strings.HasSuffix(relPath, ".vtt") {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid path"})
		return
	}

	obj, err := store.ObjectClient().GetObject(r.Context(), store.VideoBucket(), fmt.Sprintf("hls/%d/%s", videoID, relPath), minio.GetObjectOptions{})
	if err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	defer obj.Close()

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, obj)
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

	// VIP-only videos are previewable: a non-VIP (or anonymous) viewer may watch
	// the first few minutes, after which the app shows a paywall. VIP members get
	// full access. vipLocked tells the app to enforce the preview window.
	vipLocked := false
	if v.IsVip && !v.IsFree {
		vipLocked = true
		if userID, ok := parseMobileAuth(r); ok {
			var user store.MobileUser
			if err := store.DB().First(&user, userID).Error; err == nil &&
				user.VIPUntil != nil && user.VIPUntil.After(time.Now()) {
				vipLocked = false
			}
		}
	}
	previewSeconds := 0
	if vipLocked {
		previewSeconds = vipPreviewSeconds
	}

	masterPath := fmt.Sprintf("/api/hls/%d/master.m3u8", id)
	signedPath := SignPath(masterPath, hlsSignedURLTTLSeconds)
	qualities := signedHLSQualities(r.Context(), id)
	audioTracks := signedMediaTrackOptions(r.Context(), id, "audio")
	subtitleTracks := signedMediaTrackOptions(r.Context(), id, "subtitle")
	trackSummary := loadMediaTrackSummary(r.Context(), v)

	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"video_id":                  id,
		"type":                      "hls",
		"url":                       signedPath,
		"qualities":                 qualities,
		"audio_tracks":              audioTracks,
		"subtitle_tracks":           subtitleTracks,
		"media_tracks_scanned":      trackSummary.Scanned,
		"audio_track_count":         trackSummary.AudioCount,
		"subtitle_track_count":      trackSummary.SubtitleCount,
		"has_multiple_audio_tracks": trackSummary.AudioCount > 1,
		"has_subtitle_tracks":       trackSummary.SubtitleCount > 0,
		"auto_label":                "自动",
		"vip_locked":                vipLocked,
		"preview_seconds":           previewSeconds,
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
		relPath := hlsPlaylistRelativePath(line)
		if relPath == "" {
			continue
		}
		name := qualityNameFromMasterURI(relPath)
		if name == "" {
			resolution = ""
			continue
		}
		subPath := fmt.Sprintf("/api/hls/%d/%s", videoID, relPath)
		qualities = append(qualities, HLSQualityOption{
			Name:       name,
			Label:      qualityLabel(name, resolution),
			Resolution: resolution,
			URL:        SignPath(subPath, hlsSignedURLTTLSeconds),
		})
		resolution = ""
	}
	return qualities
}

func signedMediaTrackOptions(ctx context.Context, videoID int64, trackType string) []MediaTrackOption {
	var tracks []store.VideoMediaTrack
	err := store.DB().
		Where("video_id = ? AND track_type = ? AND status = ? AND object_key <> ?", videoID, trackType, "ready", "").
		Order("stream_position asc").
		Find(&tracks).Error
	if err != nil || len(tracks) == 0 {
		return nil
	}
	options := make([]MediaTrackOption, 0, len(tracks))
	for _, track := range tracks {
		if trackType == "audio" && track.StreamPosition == 0 {
			continue
		}
		url := signedMediaTrackURL(videoID, track.ObjectKey)
		if url == "" {
			continue
		}
		options = append(options, MediaTrackOption{
			ID:             track.ID,
			Type:           track.TrackType,
			Label:          mediaTrackLabel(track),
			Language:       track.Language,
			Title:          track.Title,
			Codec:          track.CodecName,
			StreamPosition: track.StreamPosition,
			Default:        track.IsDefault,
			Forced:         track.IsForced,
			URL:            url,
		})
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func loadMediaTrackSummary(ctx context.Context, v store.Video) mediaTrackSummary {
	summary := mediaTrackSummary{
		Scanned:       v.MediaTracksScanned,
		AudioCount:    v.AudioTrackCount,
		SubtitleCount: v.SubtitleTrackCount,
	}
	srcKey := sourceKeyForVideo(v)

	var sourceRows int64
	sourceQuery := store.DB().WithContext(ctx).Model(&store.VideoMediaTrack{}).
		Where("video_id = ? AND track_type = ?", v.ID, "source")
	if strings.TrimSpace(srcKey) != "" {
		sourceQuery = sourceQuery.Where("source_key = ?", srcKey)
	}
	if err := sourceQuery.Count(&sourceRows).Error; err == nil && sourceRows > 0 {
		summary.Scanned = true
	}

	var audioRows int64
	audioQuery := store.DB().WithContext(ctx).Model(&store.VideoMediaTrack{}).
		Where("video_id = ? AND track_type = ?", v.ID, "audio")
	if strings.TrimSpace(srcKey) != "" {
		audioQuery = audioQuery.Where("source_key = ?", srcKey)
	}
	if err := audioQuery.Count(&audioRows).Error; err == nil && int(audioRows) > summary.AudioCount {
		summary.AudioCount = int(audioRows)
	}

	var subtitleRows int64
	subtitleQuery := store.DB().WithContext(ctx).Model(&store.VideoMediaTrack{}).
		Where("video_id = ? AND track_type = ? AND status <> ?", v.ID, "subtitle", "unsupported")
	if strings.TrimSpace(srcKey) != "" {
		subtitleQuery = subtitleQuery.Where("source_key = ?", srcKey)
	}
	if err := subtitleQuery.Count(&subtitleRows).Error; err == nil && int(subtitleRows) > summary.SubtitleCount {
		summary.SubtitleCount = int(subtitleRows)
	}

	if summary.AudioCount > 0 || summary.SubtitleCount > 0 {
		summary.Scanned = true
	}
	return summary
}

func signedMediaTrackURL(videoID int64, objectKey string) string {
	prefix := fmt.Sprintf("hls/%d/", videoID)
	if !strings.HasPrefix(objectKey, prefix) {
		return ""
	}
	relPath := strings.TrimPrefix(objectKey, prefix)
	if !validHLSRelativePath(relPath) {
		return ""
	}
	return SignPath(fmt.Sprintf("/api/hls/%d/%s", videoID, relPath), hlsSignedURLTTLSeconds)
}

func mediaTrackLabel(track store.VideoMediaTrack) string {
	if title := strings.TrimSpace(track.Title); title != "" {
		return title
	}
	if language := strings.TrimSpace(track.Language); language != "" {
		return language
	}
	prefix := "Track"
	if track.TrackType == "audio" {
		prefix = "Audio"
	} else if track.TrackType == "subtitle" {
		prefix = "Subtitle"
	}
	return fmt.Sprintf("%s %d", prefix, track.StreamPosition+1)
}

func hlsPlaylistRelativePath(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	uriPath := strings.Trim(masterURIPath(line), "/")
	if !strings.HasSuffix(uriPath, ".m3u8") || strings.Contains(uriPath, "://") || !validHLSRelativePath(uriPath) {
		return ""
	}
	return uriPath
}

func hlsSegmentRelativePath(indexRelPath, segmentLine string) string {
	raw := strings.TrimSpace(segmentLine)
	if raw == "" || path.IsAbs(raw) || strings.Contains(raw, "://") {
		return ""
	}
	segmentPath := strings.Trim(masterURIPath(raw), "/")
	if !validHLSRelativePath(segmentPath) {
		return ""
	}
	joined := path.Clean(path.Join(path.Dir(indexRelPath), segmentPath))
	if !validHLSRelativePath(joined) {
		return ""
	}
	return joined
}

func validHLSRelativePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
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
	CoverURL     string                 `json:"cover_url"`
	CategoryName string                 `json:"category_name"`
	AIMetadata   *store.VideoAIMetadata `json:"ai_metadata,omitempty"`
}

type AppWatchHistoryItem struct {
	AppVideoItem
	Position  int       `json:"position"`
	Progress  int       `json:"progress"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AppFavoriteItem struct {
	AppVideoItem
	FavoritedAt time.Time `json:"favorited_at"`
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
	vipOnly, _ := strconv.ParseBool(q.Get("is_vip"))
	keyword := strings.TrimSpace(q.Get("q"))

	db := store.DB().Where("status = ?", "ready")
	if categoryID > 0 {
		db = db.Where("category_id = ?", categoryID)
	}
	// VIP channel: only videos that carry the "VIP 专属" badge, i.e. vip and
	// not free (matches the app's video.isVip && !video.isFree definition).
	if vipOnly {
		db = db.Where("is_vip = ? AND is_free = ?", true, false)
	}
	// Keyword search over the title (case-insensitive). Powers the app search box.
	if keyword != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}

	var total int64
	db.Model(&store.Video{}).Count(&total)

	var videos []store.Video
	db.Order("id desc").Offset((page - 1) * perPage).Limit(perPage).Find(&videos)

	// Resolve category names for just the categories on this page, rather than
	// scanning the whole categories table on every request.
	catNames := map[int]string{}
	if len(videos) > 0 {
		catIDSet := map[int]struct{}{}
		for _, v := range videos {
			if v.CategoryID > 0 {
				catIDSet[v.CategoryID] = struct{}{}
			}
		}
		if len(catIDSet) > 0 {
			catIDs := make([]int, 0, len(catIDSet))
			for id := range catIDSet {
				catIDs = append(catIDs, id)
			}
			var cats []store.Category
			store.DB().Where("id IN ?", catIDs).Find(&cats)
			for _, c := range cats {
				catNames[c.ID] = c.Name
			}
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

// GET /api/mobile/watch-history?limit=20
func AppWatchHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	limit := mobileLimit(r, 20, 50)
	var records []store.VideoPlayRecord
	if err := store.DB().
		Where("user_id = ?", userID).
		Order("updated_at desc").
		Limit(limit).
		Find(&records).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	videoIDs := make([]int64, 0, len(records))
	for _, rec := range records {
		videoIDs = append(videoIDs, rec.VideoID)
	}
	videoItems := appVideoItemsByID(videoIDs)
	items := make([]AppWatchHistoryItem, 0, len(records))
	for _, rec := range records {
		video, ok := videoItems[rec.VideoID]
		if !ok {
			continue
		}
		progress := 0
		if rec.Duration > 0 {
			progress = rec.Position * 100 / rec.Duration
			if progress > 100 {
				progress = 100
			}
		}
		items = append(items, AppWatchHistoryItem{
			AppVideoItem: video,
			Position:     rec.Position,
			Progress:     progress,
			UpdatedAt:    rec.UpdatedAt,
		})
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: items})
}

// GET /api/mobile/favorites
func AppFavoritesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	limit := mobileLimit(r, 20, 50)
	var favorites []store.VideoFavorite
	if err := store.DB().
		Where("user_id = ?", userID).
		Order("id desc").
		Limit(limit).
		Find(&favorites).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	videoIDs := make([]int64, 0, len(favorites))
	for _, fav := range favorites {
		videoIDs = append(videoIDs, fav.VideoID)
	}
	videoItems := appVideoItemsByID(videoIDs)
	items := make([]AppFavoriteItem, 0, len(favorites))
	for _, fav := range favorites {
		video, ok := videoItems[fav.VideoID]
		if !ok {
			continue
		}
		items = append(items, AppFavoriteItem{
			AppVideoItem: video,
			FavoritedAt:  fav.CreatedAt,
		})
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: items})
}

// POST/DELETE /api/mobile/favorites/{video_id}
func AppFavoriteByVideoHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	videoID, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/mobile/favorites/"), 10, 64)
	if err != nil || videoID <= 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid video id"})
		return
	}
	switch r.Method {
	case http.MethodPost:
		fav := store.VideoFavorite{UserID: int64(userID), VideoID: videoID, CreatedAt: time.Now()}
		store.DB().Where("user_id = ? AND video_id = ?", userID, videoID).FirstOrCreate(&fav)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
	case http.MethodDelete:
		store.DB().Where("user_id = ? AND video_id = ?", userID, videoID).Delete(&store.VideoFavorite{})
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func AppMobileSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseMobileAuth(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		setting := loadMobileSetting(userID)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: setting})
	case http.MethodPut:
		var req struct {
			AutoPlay         bool   `json:"auto_play"`
			WifiOnly         bool   `json:"wifi_only"`
			PreferredQuality string `json:"preferred_quality"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}
		if req.PreferredQuality == "" {
			req.PreferredQuality = "auto"
		}
		setting := store.MobileUserSetting{
			UserID:     int64(userID),
			AutoPlay:   req.AutoPlay,
			WifiOnly:   req.WifiOnly,
			PreferredQ: req.PreferredQuality,
			UpdatedAt:  time.Now(),
		}
		if err := store.DB().Save(&setting).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: setting})
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func mobileLimit(r *http.Request, fallback, max int) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = fallback
	}
	if limit > max {
		limit = max
	}
	return limit
}

func appVideoItemsByID(videoIDs []int64) map[int64]AppVideoItem {
	if len(videoIDs) == 0 {
		return map[int64]AppVideoItem{}
	}
	var videos []store.Video
	store.DB().Where("id IN ?", videoIDs).Find(&videos)
	catNames := map[int]string{}
	if len(videos) > 0 {
		var cats []store.Category
		store.DB().Find(&cats)
		for _, c := range cats {
			catNames[c.ID] = c.Name
		}
	}
	items := map[int64]AppVideoItem{}
	for _, v := range videos {
		items[v.ID] = AppVideoItem{
			Video:        v,
			CoverURL:     coverURL(v),
			CategoryName: catNames[v.CategoryID],
		}
	}
	return items
}

func loadMobileSetting(userID int) store.MobileUserSetting {
	var setting store.MobileUserSetting
	if err := store.DB().First(&setting, "user_id = ?", userID).Error; err == nil {
		return setting
	}
	setting = store.MobileUserSetting{
		UserID:     int64(userID),
		AutoPlay:   true,
		WifiOnly:   false,
		PreferredQ: "auto",
		UpdatedAt:  time.Now(),
	}
	store.DB().Create(&setting)
	return setting
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
		AIMetadata:   loadVideoAIMetadata(v.ID),
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
		if req.Position < 0 {
			req.Position = 0
		}
		if req.Duration < 0 {
			req.Duration = 0
		}
		if req.Duration > 0 && req.Position > req.Duration {
			req.Position = req.Duration
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
