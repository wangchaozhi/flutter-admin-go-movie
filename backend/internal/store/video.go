package store

import "time"

type Video struct {
	ID                 int64       `gorm:"primaryKey;column:id"           json:"id"`
	Title              string      `gorm:"column:title"                   json:"title"`
	Description        string      `gorm:"column:description"             json:"description"`
	CategoryID         int         `gorm:"column:category_id"             json:"category_id"`
	Actors             StringArray `gorm:"column:actors;type:jsonb"       json:"actors"`
	Directors          StringArray `gorm:"column:directors;type:jsonb"    json:"directors"`
	Genres             StringArray `gorm:"column:genres;type:jsonb"       json:"genres"`
	Region             string      `gorm:"column:region"                  json:"region"`
	ReleaseYear        int         `gorm:"column:release_year"            json:"release_year"`
	Language           string      `gorm:"column:language"                json:"language"`
	CoverKey           string      `gorm:"column:cover_key"               json:"cover_key"`
	OriginalKey        string      `gorm:"column:original_key"            json:"original_key"`
	HLSMasterKey       string      `gorm:"column:hls_master_key"          json:"hls_master_key"`
	Duration           int         `gorm:"column:duration"                json:"duration"`
	Size               int64       `gorm:"column:size"                    json:"size"`
	SourceWidth        int         `gorm:"column:source_width"            json:"source_width"`
	SourceHeight       int         `gorm:"column:source_height"           json:"source_height"`
	AudioTrackCount    int         `gorm:"column:audio_track_count"      json:"audio_track_count"`
	SubtitleTrackCount int         `gorm:"column:subtitle_track_count"   json:"subtitle_track_count"`
	MediaTracksScanned bool        `gorm:"column:media_tracks_scanned"   json:"media_tracks_scanned"`
	Status             string      `gorm:"column:status"                  json:"status"`
	IsVip              bool        `gorm:"column:is_vip"                  json:"is_vip"`
	IsFree             bool        `gorm:"column:is_free"                 json:"is_free"`
	CreatedAt          time.Time   `gorm:"column:created_at"              json:"created_at"`
	UpdatedAt          time.Time   `gorm:"column:updated_at"              json:"updated_at"`

	TranscodedQualities         []string `gorm:"-" json:"transcoded_qualities,omitempty"`
	AvailableTranscodeQualities []string `gorm:"-" json:"available_transcode_qualities,omitempty"`
	// Transcoding reports whether the video currently has an active transcode
	// task. It lets the admin show progress even when the video stays "ready"
	// during a merge re-transcode (the video is not taken offline for those).
	Transcoding bool `gorm:"-" json:"transcoding,omitempty"`
}

func (Video) TableName() string { return "videos" }

type VideoTranscodeTask struct {
	ID             int64      `gorm:"primaryKey;column:id"    json:"id"`
	VideoID        int64      `gorm:"column:video_id"         json:"video_id"`
	BatchID        int64      `gorm:"column:batch_id"         json:"batch_id"`
	Quality        string     `gorm:"column:quality"          json:"quality"`
	PreviousStatus string     `gorm:"column:previous_status"  json:"previous_status"`
	Status         string     `gorm:"column:status"           json:"status"`
	StatusMessage  string     `gorm:"column:status_message"   json:"status_message"`
	Progress       int        `gorm:"column:progress"          json:"progress"`
	Attempt        int        `gorm:"column:attempt"           json:"attempt"`
	ErrorMessage   string     `gorm:"column:error_message"    json:"error_message"`
	StartedAt      *time.Time `gorm:"column:started_at"       json:"started_at"`
	FinishedAt     *time.Time `gorm:"column:finished_at"      json:"finished_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"       json:"created_at"`
}

func (VideoTranscodeTask) TableName() string { return "video_transcode_tasks" }

// VideoExtractTrackTask is one background audio/subtitle extraction run for a
// video source. Unlike a transcode task it has no per-quality concept; it
// records how many audio/subtitle tracks were detected and how many were
// successfully extracted versus failed.
type VideoExtractTrackTask struct {
	ID            int64      `gorm:"primaryKey;column:id"   json:"id"`
	VideoID       int64      `gorm:"column:video_id"        json:"video_id"`
	SourceKey     string     `gorm:"column:source_key"      json:"source_key"`
	Status        string     `gorm:"column:status"          json:"status"`
	StatusMessage string     `gorm:"column:status_message"  json:"status_message"`
	AudioCount    int        `gorm:"column:audio_count"     json:"audio_count"`
	SubtitleCount int        `gorm:"column:subtitle_count"  json:"subtitle_count"`
	ReadyCount    int        `gorm:"column:ready_count"     json:"ready_count"`
	FailedCount   int        `gorm:"column:failed_count"    json:"failed_count"`
	ErrorMessage  string     `gorm:"column:error_message"   json:"error_message"`
	StartedAt     *time.Time `gorm:"column:started_at"      json:"started_at"`
	FinishedAt    *time.Time `gorm:"column:finished_at"     json:"finished_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"      json:"created_at"`
}

func (VideoExtractTrackTask) TableName() string { return "video_extract_track_tasks" }

type VideoMediaTrack struct {
	ID             int64     `gorm:"primaryKey;column:id"       json:"id"`
	VideoID        int64     `gorm:"column:video_id"            json:"video_id"`
	SourceKey      string    `gorm:"column:source_key"          json:"source_key"`
	SourceETag     string    `gorm:"column:source_etag"         json:"source_etag"`
	SourceSize     int64     `gorm:"column:source_size"         json:"source_size"`
	TrackType      string    `gorm:"column:track_type"          json:"track_type"`
	StreamIndex    int       `gorm:"column:stream_index"        json:"stream_index"`
	StreamPosition int       `gorm:"column:stream_position"     json:"stream_position"`
	CodecName      string    `gorm:"column:codec_name"          json:"codec_name"`
	Language       string    `gorm:"column:language"            json:"language"`
	Title          string    `gorm:"column:title"               json:"title"`
	IsDefault      bool      `gorm:"column:is_default"          json:"is_default"`
	IsForced       bool      `gorm:"column:is_forced"           json:"is_forced"`
	ObjectKey      string    `gorm:"column:object_key"          json:"object_key"`
	Status         string    `gorm:"column:status"              json:"status"`
	ErrorMessage   string    `gorm:"column:error_message"       json:"error_message"`
	CreatedAt      time.Time `gorm:"column:created_at"          json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"          json:"updated_at"`
}

func (VideoMediaTrack) TableName() string { return "video_media_tracks" }

type VideoPlayRecord struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id"`
	VideoID   int64     `gorm:"column:video_id"`
	Position  int       `gorm:"column:position"`
	Duration  int       `gorm:"column:duration"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (VideoPlayRecord) TableName() string { return "video_play_records" }

type VideoFavorite struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID    int64     `gorm:"column:user_id"        json:"user_id"`
	VideoID   int64     `gorm:"column:video_id"       json:"video_id"`
	CreatedAt time.Time `gorm:"column:created_at"     json:"created_at"`
}

func (VideoFavorite) TableName() string { return "video_favorites" }

type VideoAIMetadata struct {
	VideoID      int64       `gorm:"primaryKey;column:video_id"       json:"video_id"`
	Provider     string      `gorm:"column:provider"                  json:"provider"`
	Model        string      `gorm:"column:model"                     json:"model"`
	Status       string      `gorm:"column:status"                    json:"status"`
	Synopsis     string      `gorm:"column:synopsis"                  json:"synopsis"`
	Highlights   StringArray `gorm:"column:highlights;type:jsonb"     json:"highlights"`
	Tags         StringArray `gorm:"column:tags;type:jsonb"           json:"tags"`
	ErrorMessage string      `gorm:"column:error_message"             json:"error_message,omitempty"`
	GeneratedAt  *time.Time  `gorm:"column:generated_at"              json:"generated_at,omitempty"`
	CreatedAt    time.Time   `gorm:"column:created_at"                json:"created_at"`
	UpdatedAt    time.Time   `gorm:"column:updated_at"                json:"updated_at"`
}

func (VideoAIMetadata) TableName() string { return "video_ai_metadata" }

type MobileUserSetting struct {
	UserID     int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
	AutoPlay   bool      `gorm:"column:auto_play"          json:"auto_play"`
	WifiOnly   bool      `gorm:"column:wifi_only"          json:"wifi_only"`
	PreferredQ string    `gorm:"column:preferred_quality"  json:"preferred_quality"`
	UpdatedAt  time.Time `gorm:"column:updated_at"         json:"updated_at"`
}

func (MobileUserSetting) TableName() string { return "mobile_user_settings" }
