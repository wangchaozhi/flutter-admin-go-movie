package payment

import (
	"os"
	"strings"
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
	return Config{
		MockEnabled:      strings.EqualFold(os.Getenv("PAYMENT_MOCK"), "true"),
		PublicBaseURL:    envOr("APP_PUBLIC_BASE_URL", "http://localhost:8080"),
		DefaultCurrency:  envOr("PAYMENT_DEFAULT_CURRENCY", "USD"),
		StripeSecretKey:  strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")),
		StripeWebhookKey: strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		StripeSuccessURL: strings.TrimSpace(os.Getenv("STRIPE_SUCCESS_URL")),
		StripeCancelURL:  strings.TrimSpace(os.Getenv("STRIPE_CANCEL_URL")),
		PayPalClientID:   strings.TrimSpace(os.Getenv("PAYPAL_CLIENT_ID")),
		PayPalSecret:     strings.TrimSpace(os.Getenv("PAYPAL_CLIENT_SECRET")),
		PayPalWebhookID:  strings.TrimSpace(os.Getenv("PAYPAL_WEBHOOK_ID")),
		PayPalBaseURL:    envOr("PAYPAL_BASE_URL", "https://api-m.sandbox.paypal.com"),
	}
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
