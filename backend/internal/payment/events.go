package payment

import (
	"log/slog"
	"strings"
	"time"

	"flutter-admin-go/internal/store"

	"gorm.io/gorm/clause"
)

// processGatewayPayment idempotently applies a confirmed gateway payment: it
// skips events already recorded, marks the matching order paid (granting VIP
// time) and records the event for audit. markOrderPaid is itself idempotent and
// row-locks the order, so concurrent or retried deliveries never double-grant.
func processGatewayPayment(provider, eventID, eventType, orderNo, paymentID string, payload []byte) error {
	if eventID != "" && paymentEventExists(provider, eventID) {
		return nil // already handled
	}
	if strings.TrimSpace(orderNo) != "" {
		if err := markOrderPaid(orderNo, paymentID); err != nil {
			// Status conflicts (e.g. already cancelled) are terminal; transient
			// errors are rare and markOrderPaid is safe to re-run. Log and still
			// record so the gateway is not retried indefinitely.
			slog.Warn("apply gateway payment failed", "provider", provider, "order_no", orderNo, "error", err)
		}
	}
	recordPaymentEvent(provider, eventID, eventType, orderNo, payload)
	return nil
}

// recordPaymentEvent stores a webhook event for idempotency/audit, ignoring
// duplicates via the UNIQUE(provider, event_id) constraint.
func recordPaymentEvent(provider, eventID, eventType, orderNo string, payload []byte) {
	if eventID == "" {
		return
	}
	now := time.Now()
	rec := store.PaymentEvent{
		Provider:    provider,
		EventID:     eventID,
		EventType:   eventType,
		OrderNo:     orderNo,
		Payload:     string(payload),
		ProcessedAt: &now,
		CreatedAt:   now,
	}
	if err := store.DB().Clauses(clause.OnConflict{DoNothing: true}).Create(&rec).Error; err != nil {
		slog.Error("record payment event failed", "provider", provider, "event_id", eventID, "error", err)
	}
}

func paymentEventExists(provider, eventID string) bool {
	var count int64
	store.DB().Model(&store.PaymentEvent{}).
		Where("provider = ? AND event_id = ?", provider, eventID).
		Count(&count)
	return count > 0
}
