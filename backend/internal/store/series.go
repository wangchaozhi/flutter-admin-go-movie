package store

import "time"

// Series groups multiple episodes (each a row in videos tagged with series_id).
type Series struct {
	ID          int64       `gorm:"primaryKey;column:id"        json:"id"`
	Title       string      `gorm:"column:title"                json:"title"`
	Description string      `gorm:"column:description"          json:"description"`
	CoverKey    string      `gorm:"column:cover_key"            json:"cover_key"`
	CategoryID  int         `gorm:"column:category_id"          json:"category_id"`
	Region      string      `gorm:"column:region"               json:"region"`
	ReleaseYear int         `gorm:"column:release_year"         json:"release_year"`
	Genres      StringArray `gorm:"column:genres;type:jsonb"    json:"genres"`
	IsVip       bool        `gorm:"column:is_vip"               json:"is_vip"`
	Status      string      `gorm:"column:status"               json:"status"`
	CreatedAt   time.Time   `gorm:"column:created_at"           json:"created_at"`
	UpdatedAt   time.Time   `gorm:"column:updated_at"           json:"updated_at"`

	// Derived fields, populated by handlers (not stored).
	EpisodeCount int    `gorm:"-" json:"episode_count,omitempty"`
	CoverURL     string `gorm:"-" json:"cover_url,omitempty"`
	CategoryName string `gorm:"-" json:"category_name,omitempty"`
}

func (Series) TableName() string { return "series" }
