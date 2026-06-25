package payment

import (
	"context"
	"log/slog"
	"time"

	"flutter-admin-go/internal/store"
)

const (
	orderExpiryInterval     = 5 * time.Minute
	orderExpiryStartupDelay = 30 * time.Second
)

// StartOrderExpiryJanitor periodically cancels checkout orders that were never
// completed before their expires_at deadline, so stale "pending"/"paying" rows
// don't linger forever. It runs until ctx is cancelled.
func StartOrderExpiryJanitor(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(orderExpiryStartupDelay):
		}
		ExpireStaleOrders()

		ticker := time.NewTicker(orderExpiryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ExpireStaleOrders()
			}
		}
	}()
}

// ExpireStaleOrders marks unpaid orders whose expiry has passed as "cancelled".
// Paid/refunded orders are never touched. Returns the number of rows updated.
func ExpireStaleOrders() int64 {
	now := time.Now()
	result := store.DB().Model(&store.Order{}).
		Where("status IN ? AND expires_at IS NOT NULL AND expires_at < ?", []string{"pending", "paying"}, now).
		Updates(map[string]interface{}{"status": "cancelled", "updated_at": now})
	if result.Error != nil {
		slog.Error("order expiry sweep failed", "error", result.Error)
		return 0
	}
	if result.RowsAffected > 0 {
		slog.Info("expired stale orders", "count", result.RowsAffected)
	}
	return result.RowsAffected
}
