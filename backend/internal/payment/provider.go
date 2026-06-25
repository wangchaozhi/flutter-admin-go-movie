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

type stripeProvider struct {
	cfg Config
}

func (p stripeProvider) Name() string { return "stripe" }

func (p stripeProvider) CreateCheckout(_ context.Context, _ store.Order, _ store.Product) (CheckoutSession, error) {
	if p.cfg.StripeSecretKey == "" {
		return CheckoutSession{}, fmt.Errorf("STRIPE_SECRET_KEY is not configured")
	}
	return CheckoutSession{}, fmt.Errorf("stripe checkout is not implemented yet")
}

func (p stripeProvider) Refund(_ context.Context, _ store.Order) (RefundResult, error) {
	if p.cfg.StripeSecretKey == "" {
		return RefundResult{}, fmt.Errorf("STRIPE_SECRET_KEY is not configured")
	}
	return RefundResult{}, fmt.Errorf("stripe refund is not implemented yet")
}

type paypalProvider struct {
	cfg Config
}

func (p paypalProvider) Name() string { return "paypal" }

func (p paypalProvider) CreateCheckout(_ context.Context, _ store.Order, _ store.Product) (CheckoutSession, error) {
	if p.cfg.PayPalClientID == "" || p.cfg.PayPalSecret == "" {
		return CheckoutSession{}, fmt.Errorf("PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are not configured")
	}
	return CheckoutSession{}, fmt.Errorf("paypal orders v2 checkout is not implemented yet")
}

func (p paypalProvider) Refund(_ context.Context, _ store.Order) (RefundResult, error) {
	if p.cfg.PayPalClientID == "" || p.cfg.PayPalSecret == "" {
		return RefundResult{}, fmt.Errorf("PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are not configured")
	}
	return RefundResult{}, fmt.Errorf("paypal refund is not implemented yet")
}
