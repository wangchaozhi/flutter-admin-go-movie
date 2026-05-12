package payment

import (
	"flutter-admin-go/internal/config"
)

type Config struct {
	MockEnabled      bool
	PublicBaseURL    string
	DefaultCurrency  string
	StripeSecretKey  string
	StripeWebhookKey string
	StripeSuccessURL string
	StripeCancelURL  string
	PayPalClientID   string
	PayPalSecret     string
	PayPalWebhookID  string
	PayPalBaseURL    string
}

func LoadConfig() Config {
	cfg := config.Load().Payment
	return Config{
		MockEnabled:      cfg.MockEnabled,
		PublicBaseURL:    cfg.PublicBaseURL,
		DefaultCurrency:  cfg.DefaultCurrency,
		StripeSecretKey:  cfg.StripeSecretKey,
		StripeWebhookKey: cfg.StripeWebhookKey,
		StripeSuccessURL: cfg.StripeSuccessURL,
		StripeCancelURL:  cfg.StripeCancelURL,
		PayPalClientID:   cfg.PayPalClientID,
		PayPalSecret:     cfg.PayPalSecret,
		PayPalWebhookID:  cfg.PayPalWebhookID,
		PayPalBaseURL:    cfg.PayPalBaseURL,
	}
}
