package store

import "time"

type Category struct {
	ID        int       `gorm:"primaryKey;column:id"    json:"id"`
	Name      string    `gorm:"column:name"             json:"name"`
	SortOrder int       `gorm:"column:sort_order"       json:"sort_order"`
	CreatedAt time.Time `gorm:"column:created_at"       json:"created_at"`
}

func (Category) TableName() string { return "categories" }
