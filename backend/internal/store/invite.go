package store

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Invite-code consumption outcomes. The handler maps these to user-facing
// messages; the conditional UPDATE in ConsumeInviteCode is the source of truth
// so concurrent sign-ups can never exceed max_uses.
var (
	ErrInviteRequired  = errors.New("invite code required")
	ErrInviteInvalid   = errors.New("invite code invalid")
	ErrInviteDisabled  = errors.New("invite code disabled")
	ErrInviteExpired   = errors.New("invite code expired")
	ErrInviteExhausted = errors.New("invite code exhausted")
)

// ConsumeInviteCode validates and atomically consumes one use of an invite code.
// The leading SELECT only produces a precise error; the conditional UPDATE is
// what actually reserves the use, so two simultaneous registrations sharing the
// last slot can't both succeed. max_uses = 0 means unlimited.
func ConsumeInviteCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrInviteRequired
	}
	var ic InviteCode
	if err := db.Where("code = ?", code).First(&ic).Error; err != nil {
		return ErrInviteInvalid
	}
	now := time.Now()
	if ic.Status != "active" {
		return ErrInviteDisabled
	}
	if ic.ExpiresAt != nil && !ic.ExpiresAt.After(now) {
		return ErrInviteExpired
	}
	if ic.MaxUses > 0 && ic.UsedCount >= ic.MaxUses {
		return ErrInviteExhausted
	}
	res := db.Model(&InviteCode{}).
		Where("id = ? AND status = 'active' AND (max_uses = 0 OR used_count < max_uses) AND (expires_at IS NULL OR expires_at > ?)", ic.ID, now).
		Updates(map[string]any{
			"used_count": gorm.Expr("used_count + 1"),
			"updated_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInviteExhausted
	}
	return nil
}

// ListInviteCodes returns codes newest-first for the admin module.
func ListInviteCodes() ([]InviteCode, error) {
	var codes []InviteCode
	err := db.Order("created_at DESC, id DESC").Find(&codes).Error
	return codes, err
}

// CreateInviteCode persists a new code. Caller supplies a unique code string.
func CreateInviteCode(ic *InviteCode) error {
	now := time.Now()
	ic.CreatedAt = now
	ic.UpdatedAt = now
	if ic.Status == "" {
		ic.Status = "active"
	}
	return db.Create(ic).Error
}

// SetInviteCodeStatus flips a code between "active" and "disabled".
func SetInviteCodeStatus(id int, status string) error {
	return db.Model(&InviteCode{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

// InviteCodeExists reports whether a code string is already taken.
func InviteCodeExists(code string) (bool, error) {
	var count int64
	err := db.Model(&InviteCode{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}
