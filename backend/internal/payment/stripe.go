package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/store"
)

const stripeAPIBase = "https://api.stripe.com"

// stripeSignatureTolerance bounds how old a webhook timestamp may be, to limit
// replay of captured signatures.
const stripeSignatureTolerance = 5 * time.Minute

// CreateCheckout creates a Stripe Checkout Session for the order and returns its
// hosted payment URL. The order number is carried as client_reference_id so the
// webhook can tie the completed session back to our order.
func (p stripeProvider) CreateCheckout(ctx context.Context, order store.Order, product store.Product) (CheckoutSession, error) {
	if p.cfg.StripeSecretKey == "" {
		return CheckoutSession{}, fmt.Errorf("STRIPE_SECRET_KEY is not configured")
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", p.successURL())
	form.Set("cancel_url", p.cancelURL())
	form.Set("client_reference_id", order.OrderNo)
	form.Set("metadata[order_no]", order.OrderNo)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", strings.ToLower(order.Currency))
	form.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(order.AmountCents))
	name := product.Name
	if name == "" {
		name = product.Code
	}
	form.Set("line_items[0][price_data][product_data][name]", name)

	body, err := p.postForm(ctx, "/v1/checkout/sessions", form)
	if err != nil {
		return CheckoutSession{}, err
	}
	id, _ := body["id"].(string)
	checkoutURL, _ := body["url"].(string)
	if id == "" || checkoutURL == "" {
		return CheckoutSession{}, fmt.Errorf("stripe returned an incomplete session")
	}
	return CheckoutSession{ProviderOrderID: id, CheckoutURL: checkoutURL}, nil
}

// Refund issues a full refund against the payment intent captured for the order.
func (p stripeProvider) Refund(ctx context.Context, order store.Order) (RefundResult, error) {
	if p.cfg.StripeSecretKey == "" {
		return RefundResult{}, fmt.Errorf("STRIPE_SECRET_KEY is not configured")
	}
	if strings.TrimSpace(order.ProviderPaymentID) == "" {
		return RefundResult{}, fmt.Errorf("order has no stripe payment intent to refund")
	}
	form := url.Values{}
	form.Set("payment_intent", order.ProviderPaymentID)
	body, err := p.postForm(ctx, "/v1/refunds", form)
	if err != nil {
		return RefundResult{}, err
	}
	id, _ := body["id"].(string)
	return RefundResult{RefundID: id}, nil
}

func (p stripeProvider) successURL() string {
	if p.cfg.StripeSuccessURL != "" {
		return p.cfg.StripeSuccessURL
	}
	return strings.TrimRight(p.cfg.PublicBaseURL, "/") + "/payment/success"
}

func (p stripeProvider) cancelURL() string {
	if p.cfg.StripeCancelURL != "" {
		return p.cfg.StripeCancelURL
	}
	return strings.TrimRight(p.cfg.PublicBaseURL, "/") + "/payment/cancel"
}

// postForm performs a form-encoded Stripe API call and returns the decoded JSON
// object, surfacing the gateway's error message on non-2xx responses.
func (p stripeProvider) postForm(ctx context.Context, path string, form url.Values) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stripeAPIBase+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.StripeSecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := paymentHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("stripe error: %s", stripeErrorMessage(parsed, resp.StatusCode))
	}
	return parsed, nil
}

func stripeErrorMessage(body map[string]any, status int) string {
	if errObj, ok := body["error"].(map[string]any); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return "HTTP " + strconv.Itoa(status)
}

// verifyStripeSignature validates the Stripe-Signature header against the raw
// payload using the webhook signing secret, following Stripe's scheme:
// signed_payload = "{t}.{payload}", expected = hex(HMAC-SHA256(secret, signed_payload)).
func verifyStripeSignature(payload []byte, sigHeader, secret string) error {
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return fmt.Errorf("missing timestamp or signature")
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	if d := time.Since(time.Unix(ts, 0)); d > stripeSignatureTolerance || d < -stripeSignatureTolerance {
		return fmt.Errorf("timestamp outside tolerance")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range signatures {
		if hmac.Equal([]byte(expected), []byte(sig)) {
			return nil
		}
	}
	return fmt.Errorf("signature mismatch")
}

// stripeEvent is the subset of a Stripe webhook event we consume.
type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID                string `json:"id"`
			ClientReferenceID string `json:"client_reference_id"`
			PaymentIntent     string `json:"payment_intent"`
			PaymentStatus     string `json:"payment_status"`
		} `json:"object"`
	} `json:"data"`
}

// handleStripeEvent records the event for idempotency and, for a completed paid
// checkout, marks the matching order paid (granting VIP time). Unknown event
// types are accepted and ignored so Stripe does not retry them.
func handleStripeEvent(payload []byte) error {
	var event stripeEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("invalid event payload")
	}
	if event.ID == "" {
		return fmt.Errorf("event missing id")
	}

	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		obj := event.Data.Object
		if obj.PaymentStatus != "" && obj.PaymentStatus != "paid" {
			return nil // not actually paid (e.g. unpaid/no_payment_required handled elsewhere)
		}
		return processGatewayPayment("stripe", event.ID, event.Type, obj.ClientReferenceID, obj.PaymentIntent, payload)
	default:
		// Record-and-ignore other events for idempotency/audit.
		recordPaymentEvent("stripe", event.ID, event.Type, "", payload)
		return nil
	}
}
