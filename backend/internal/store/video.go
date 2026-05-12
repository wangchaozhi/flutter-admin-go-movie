package store

import "time"

type Video struct {
	ID           int64     `gorm:"primaryKey;column:id"           json:"id"`
	Title        string    `gorm:"column:title"                   json:"title"`
	Description  string    `gorm:"column:description"             json:"description"`
	CategoryID   int       `gorm:"column:category_id"             json:"category_id"`
	CoverKey     string    `gorm:"column:cover_key"               json:"cover_key"`
	OriginalKey  string    `gorm:"column:original_key"            json:"original_key"`
	HLSMasterKey string    `gorm:"column:hls_master_key"          json:"hls_master_key"`
	Duration     int       `gorm:"column:duration"                json:"duration"`
	Size         int64     `gorm:"column:size"                    json:"size"`
	Status       string    `gorm:"column:status"                  json:"status"`
	IsVip        bool      `gorm:"column:is_vip"                  json:"is_vip"`
	IsFree       bool      `gorm:"column:is_free"                 json:"is_free"`
	CreatedAt    time.Time `gorm:"column:created_at"              json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"              json:"updated_at"`
}

func (Video) TableName() string { return "videos" }

type VideoTranscodeTask struct {
	ID           int64      `gorm:"primaryKey;column:id"    json:"id"`
	VideoID      int64      `gorm:"column:video_id"         json:"video_id"`
	Status       string     `gorm:"column:status"           json:"status"`
	ErrorMessage string     `gorm:"column:error_message"    json:"error_message"`
	StartedAt    *time.Time `gorm:"column:started_at"       json:"started_at"`
	FinishedAt   *time.Time `gorm:"column:finished_at"      json:"finished_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"       json:"created_at"`
}

func (VideoTranscodeTask) TableName() string { return "video_transcode_tasks" }

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

type MobileUserSetting struct {
	UserID     int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
	AutoPlay   bool      `gorm:"column:auto_play"          json:"auto_play"`
	WifiOnly   bool      `gorm:"column:wifi_only"          json:"wifi_only"`
	PreferredQ string    `gorm:"column:preferred_quality"  json:"preferred_quality"`
	UpdatedAt  time.Time `gorm:"column:updated_at"         json:"updated_at"`
}

func (MobileUserSetting) TableName() string { return "mobile_user_settings" }
