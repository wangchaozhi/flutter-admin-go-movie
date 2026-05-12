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

type Provider interface {
	Name() string
	CreateCheckout(ctx context.Context, order store.Order, product store.Product) (CheckoutSession, error)
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
