package payment

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"flutter-admin-go/internal/store"
)

type CheckoutSession struct {
	ProviderOrderID string
	CheckoutURL     string
}

// RefundResult carries the provider-side refund identifier so the order can be
// reconciled against the gateway later.
type RefundResult struct {
	RefundID string
}

type Provider interface {
	Name() string
	CreateCheckout(ctx context.Context, order store.Order, product store.Product) (CheckoutSession, error)
	// Refund settles a full refund for an already-paid order with the gateway.
	Refund(ctx context.Context, order store.Order) (RefundResult, error)
}

func providerFor(name string, cfg Config) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mock":
		if !cfg.MockEnabled {
			return nil, fmt.Errorf("mock payments are disabled")
		}
		return mockProvider{cfg: cfg}, nil
	case "stripe":
		return stripeProvider{cfg: cfg}, nil
	case "paypal":
		return paypalProvider{cfg: cfg}, nil
	case "wechat":
		return wechatProvider{cfg: cfg}, nil
	case "alipay":
		return alipayProvider{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}

type mockProvider struct {
	cfg Config
}

func (p mockProvider) Name() string { return "mock" }

func (p mockProvider) CreateCheckout(_ context.Context, order store.Order, _ store.Product) (CheckoutSession, error) {
	return CheckoutSession{
		ProviderOrderID: "mock_" + order.OrderNo,
		CheckoutURL:     strings.TrimRight(p.cfg.PublicBaseURL, "/") + "/api/orders/" + url.PathEscape(order.OrderNo) + "/mock-complete",
	}, nil
}

func (p mockProvider) Refund(_ context.Context, order store.Order) (RefundResult, error) {
	return RefundResult{RefundID: "mock_refund_" + order.OrderNo}, nil
}

// stripeProvider implements the Stripe Checkout + refund flow. Its methods live
// in stripe.go.
type stripeProvider struct {
	cfg Config
}

func (p stripeProvider) Name() string { return "stripe" }

// paypalProvider implements the PayPal Orders v2 checkout + capture + refund
// flow. Its methods live in paypal.go.
type paypalProvider struct {
	cfg Config
}

func (p paypalProvider) Name() string { return "paypal" }

// wechatProvider implements WeChat Pay v3 Native (QR) checkout + refund. Its
// methods live in wechat.go.
type wechatProvider struct {
	cfg Config
}

func (p wechatProvider) Name() string { return "wechat" }

// alipayProvider implements Alipay precreate (QR) checkout + refund. Its methods
// live in alipay.go.
type alipayProvider struct {
	cfg Config
}

func (p alipayProvider) Name() string { return "alipay" }
