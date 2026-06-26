package store

import (
	"log/slog"
	"time"
)

// AuditLog records one admin mutation (POST/PUT/DELETE under /api/admin) for
// after-the-fact review of who changed what.
type AuditLog struct {
	ID        int64     `gorm:"primaryKey;column:id"   json:"id"`
	RequestID string    `gorm:"column:request_id"      json:"request_id"`
	Username  string    `gorm:"column:username"        json:"username"`
	Method    string    `gorm:"column:method"          json:"method"`
	Path      string    `gorm:"column:path"            json:"path"`
	Status    int       `gorm:"column:status"          json:"status"`
	IP        string    `gorm:"column:ip"              json:"ip"`
	CreatedAt time.Time `gorm:"column:created_at"      json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// WriteAuditLog persists an audit entry. It is best-effort: a failure is logged
// but never propagated, so auditing can never break the request it describes.
// Intended to be called from a goroutine off the request path.
func WriteAuditLog(entry AuditLog) {
	if db == nil {
		return
	}
	if err := db.Create(&entry).Error; err != nil {
		slog.Error("write audit log failed", "error", err, "path", entry.Path)
	}
}
