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

	WeChatAppID      string
	WeChatMchID      string
	WeChatAPIv3Key   string
	WeChatSerialNo   string
	WeChatPrivateKey string
	WeChatNotifyURL  string

	AlipayAppID      string
	AlipayPrivateKey string
	AlipayPublicKey  string
	AlipayGateway    string
	AlipayNotifyURL  string
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

		WeChatAppID:      cfg.WeChatAppID,
		WeChatMchID:      cfg.WeChatMchID,
		WeChatAPIv3Key:   cfg.WeChatAPIv3Key,
		WeChatSerialNo:   cfg.WeChatSerialNo,
		WeChatPrivateKey: cfg.WeChatPrivateKey,
		WeChatNotifyURL:  cfg.WeChatNotifyURL,

		AlipayAppID:      cfg.AlipayAppID,
		AlipayPrivateKey: cfg.AlipayPrivateKey,
		AlipayPublicKey:  cfg.AlipayPublicKey,
		AlipayGateway:    cfg.AlipayGateway,
		AlipayNotifyURL:  cfg.AlipayNotifyURL,
	}
}
