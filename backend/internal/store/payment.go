package store

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
	ID           int            `gorm:"primaryKey;column:id"       json:"id"`
	Code         string         `gorm:"column:code"                json:"code"`
	Name         string         `gorm:"column:name"                json:"name"`
	Description  string         `gorm:"column:description"         json:"description"`
	Kind         string         `gorm:"column:kind"                json:"kind"`
	PriceCents   int            `gorm:"column:price_cents"         json:"price_cents"`
	Currency     string         `gorm:"column:currency"            json:"currency"`
	DurationDays int            `gorm:"column:duration_days"       json:"duration_days"`
	VideoID      *int64         `gorm:"column:video_id"            json:"video_id,omitempty"`
	Status       string         `gorm:"column:status"              json:"status"`
	CreatedAt    time.Time      `gorm:"column:created_at"          json:"created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at"          json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;index"    json:"-"`
}

func (Product) TableName() string { return "products" }

type Order struct {
	ID                int            `gorm:"primaryKey;column:id"          json:"id"`
	OrderNo           string         `gorm:"column:order_no"               json:"order_no"`
	UserID            int            `gorm:"column:user_id"                json:"user_id"`
	ProductID         int            `gorm:"column:product_id"             json:"product_id"`
	Provider          string         `gorm:"column:provider"              json:"provider"`
	Status            string         `gorm:"column:status"                json:"status"`
	AmountCents       int            `gorm:"column:amount_cents"          json:"amount_cents"`
	Currency          string         `gorm:"column:currency"              json:"currency"`
	ProviderOrderID   string         `gorm:"column:provider_order_id"     json:"provider_order_id"`
	ProviderPaymentID string         `gorm:"column:provider_payment_id"   json:"provider_payment_id"`
	CheckoutURL       string         `gorm:"column:checkout_url"          json:"checkout_url"`
	PaidAt            *time.Time     `gorm:"column:paid_at"               json:"paid_at"`
	RefundedAt        *time.Time     `gorm:"column:refunded_at"           json:"refunded_at"`
	RefundID          string         `gorm:"column:refund_id"             json:"refund_id"`
	ExpiresAt         *time.Time     `gorm:"column:expires_at"            json:"expires_at"`
	CreatedAt         time.Time      `gorm:"column:created_at"            json:"created_at"`
	UpdatedAt         time.Time      `gorm:"column:updated_at"            json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index"       json:"-"`
	Product           Product        `gorm:"foreignKey:ProductID"         json:"product,omitempty"`
	User              MobileUser     `gorm:"foreignKey:UserID"            json:"user,omitempty"`
}

func (Order) TableName() string { return "orders" }

type PaymentEvent struct {
	ID          int        `gorm:"primaryKey;column:id"        json:"id"`
	Provider    string     `gorm:"column:provider"             json:"provider"`
	EventID     string     `gorm:"column:event_id"             json:"event_id"`
	EventType   string     `gorm:"column:event_type"           json:"event_type"`
	OrderNo     string     `gorm:"column:order_no"             json:"order_no"`
	Payload     string     `gorm:"column:payload;type:jsonb"   json:"payload"`
	ProcessedAt *time.Time `gorm:"column:processed_at"         json:"processed_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"           json:"created_at"`
}

func (PaymentEvent) TableName() string { return "payment_events" }
