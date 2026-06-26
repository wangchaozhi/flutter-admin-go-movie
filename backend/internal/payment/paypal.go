package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"flutter-admin-go/internal/store"
)

// zeroDecimalCurrencies are billed without minor units; PayPal expects their
// amount "value" as a whole number to match how price_cents is stored for them.
var zeroDecimalCurrencies = map[string]bool{"JPY": true, "TWD": true, "HUF": true}

// CreateCheckout opens a PayPal Orders v2 order (intent CAPTURE) and returns the
// approval URL the buyer is redirected to. The order number rides along as
// custom_id/reference_id so the webhook can map the capture back to our order.
func (p paypalProvider) CreateCheckout(ctx context.Context, order store.Order, product store.Product) (CheckoutSession, error) {
	if p.cfg.PayPalClientID == "" || p.cfg.PayPalSecret == "" {
		return CheckoutSession{}, fmt.Errorf("PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are not configured")
	}
	token, err := p.token(ctx)
	if err != nil {
		return CheckoutSession{}, err
	}

	reqBody := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{{
			"reference_id": order.OrderNo,
			"custom_id":    order.OrderNo,
			"amount": map[string]any{
				"currency_code": strings.ToUpper(order.Currency),
				"value":         paypalAmountValue(order.Currency, order.AmountCents),
			},
		}},
		"application_context": map[string]any{
			"return_url":  p.returnURL(),
			"cancel_url":  p.cancelURL(),
			"user_action": "PAY_NOW",
		},
	}

	raw, status, err := p.doJSON(ctx, http.MethodPost, "/v2/checkout/orders", token, reqBody)
	if err != nil {
		return CheckoutSession{}, err
	}
	if status >= 300 {
		return CheckoutSession{}, fmt.Errorf("paypal create order failed: %s", paypalError(raw, status))
	}
	var parsed paypalOrderResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return CheckoutSession{}, fmt.Errorf("paypal create order: invalid response")
	}
	approve := ""
	for _, link := range parsed.Links {
		if strings.EqualFold(link.Rel, "approve") || strings.EqualFold(link.Rel, "payer-action") {
			approve = link.Href
			break
		}
	}
	if parsed.ID == "" || approve == "" {
		return CheckoutSession{}, fmt.Errorf("paypal returned no approval link")
	}
	return CheckoutSession{ProviderOrderID: parsed.ID, CheckoutURL: approve}, nil
}

// captureOrder captures an approved PayPal order and returns the capture id used
// later for refunds. Safe to treat an already-captured order as success.
func (p paypalProvider) captureOrder(ctx context.Context, paypalOrderID string) (string, error) {
	token, err := p.token(ctx)
	if err != nil {
		return "", err
	}
	raw, status, err := p.doJSON(ctx, http.MethodPost, "/v2/checkout/orders/"+url.PathEscape(paypalOrderID)+"/capture", token, nil)
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("paypal capture failed: %s", paypalError(raw, status))
	}
	var parsed paypalCaptureResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("paypal capture: invalid response")
	}
	for _, unit := range parsed.PurchaseUnits {
		for _, cap := range unit.Payments.Captures {
			if cap.ID != "" {
				return cap.ID, nil
			}
		}
	}
	return "", nil
}

// Refund issues a full refund against the capture recorded for the order.
func (p paypalProvider) Refund(ctx context.Context, order store.Order) (RefundResult, error) {
	if p.cfg.PayPalClientID == "" || p.cfg.PayPalSecret == "" {
		return RefundResult{}, fmt.Errorf("PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are not configured")
	}
	captureID := strings.TrimSpace(order.ProviderPaymentID)
	if captureID == "" {
		return RefundResult{}, fmt.Errorf("order has no paypal capture to refund")
	}
	token, err := p.token(ctx)
	if err != nil {
		return RefundResult{}, err
	}
	raw, status, err := p.doJSON(ctx, http.MethodPost, "/v2/payments/captures/"+url.PathEscape(captureID)+"/refund", token, map[string]any{})
	if err != nil {
		return RefundResult{}, err
	}
	if status >= 300 {
		return RefundResult{}, fmt.Errorf("paypal refund failed: %s", paypalError(raw, status))
	}
	var parsed paypalRefundResponse
	_ = json.Unmarshal(raw, &parsed)
	return RefundResult{RefundID: parsed.ID}, nil
}

// verifyWebhook asks PayPal to verify the transmission signature of an incoming
// webhook against the configured webhook id, the standard server-side check.
func (p paypalProvider) verifyWebhook(ctx context.Context, headers http.Header, rawEvent []byte) (bool, error) {
	if p.cfg.PayPalWebhookID == "" {
		return false, fmt.Errorf("PAYPAL_WEBHOOK_ID is not configured")
	}
	token, err := p.token(ctx)
	if err != nil {
		return false, err
	}
	reqBody := map[string]any{
		"auth_algo":         headers.Get("Paypal-Auth-Algo"),
		"cert_url":          headers.Get("Paypal-Cert-Url"),
		"transmission_id":   headers.Get("Paypal-Transmission-Id"),
		"transmission_sig":  headers.Get("Paypal-Transmission-Sig"),
		"transmission_time": headers.Get("Paypal-Transmission-Time"),
		"webhook_id":        p.cfg.PayPalWebhookID,
		"webhook_event":     json.RawMessage(rawEvent),
	}
	raw, status, err := p.doJSON(ctx, http.MethodPost, "/v1/notifications/verify-webhook-signature", token, reqBody)
	if err != nil {
		return false, err
	}
	if status >= 300 {
		return false, fmt.Errorf("paypal verify failed: %s", paypalError(raw, status))
	}
	var parsed paypalVerifyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, fmt.Errorf("paypal verify: invalid response")
	}
	return strings.EqualFold(parsed.VerificationStatus, "SUCCESS"), nil
}

// token fetches an OAuth2 access token via client-credentials.
func (p paypalProvider) token(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base()+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(p.cfg.PayPalClientID, p.cfg.PayPalSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := paymentHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("paypal auth failed: %s", paypalError(raw, resp.StatusCode))
	}
	var parsed paypalTokenResponse
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.AccessToken == "" {
		return "", fmt.Errorf("paypal auth: invalid token response")
	}
	return parsed.AccessToken, nil
}

func (p paypalProvider) doJSON(ctx context.Context, method, path, token string, body any) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base()+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := paymentHTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func (p paypalProvider) base() string {
	if p.cfg.PayPalBaseURL != "" {
		return strings.TrimRight(p.cfg.PayPalBaseURL, "/")
	}
	return "https://api-m.sandbox.paypal.com"
}

func (p paypalProvider) returnURL() string {
	return strings.TrimRight(p.cfg.PublicBaseURL, "/") + "/payment/success"
}

func (p paypalProvider) cancelURL() string {
	return strings.TrimRight(p.cfg.PublicBaseURL, "/") + "/payment/cancel"
}

func paypalAmountValue(currency string, cents int) string {
	if zeroDecimalCurrencies[strings.ToUpper(currency)] {
		return strconv.Itoa(cents)
	}
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

func paypalError(raw []byte, status int) string {
	var parsed struct {
		Message string `json:"message"`
		Name    string `json:"name"`
	}
	if json.Unmarshal(raw, &parsed) == nil && parsed.Message != "" {
		return parsed.Message
	}
	return "HTTP " + strconv.Itoa(status)
}

type paypalOrderResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Links  []struct {
		Href string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"links"`
}

type paypalCaptureResponse struct {
	Status        string `json:"status"`
	PurchaseUnits []struct {
		Payments struct {
			Captures []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"captures"`
		} `json:"payments"`
	} `json:"purchase_units"`
}

type paypalRefundResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type paypalTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type paypalVerifyResponse struct {
	VerificationStatus string `json:"verification_status"`
}

// paypalEvent is the subset of a webhook event we consume.
type paypalEvent struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Resource  struct {
		ID            string `json:"id"`
		CustomID      string `json:"custom_id"`
		PurchaseUnits []struct {
			CustomID    string `json:"custom_id"`
			ReferenceID string `json:"reference_id"`
		} `json:"purchase_units"`
	} `json:"resource"`
}

// handlePayPalEvent verifies-then-applies an incoming PayPal webhook. On order
// approval it captures the funds; on capture completion it marks the order paid.
func handlePayPalEvent(ctx context.Context, provider paypalProvider, payload []byte) error {
	var event paypalEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("invalid event payload")
	}
	if event.ID != "" && paymentEventExists("paypal", event.ID) {
		return nil // already handled (also guards against re-capturing)
	}

	switch event.EventType {
	case "PAYMENT.CAPTURE.COMPLETED":
		orderNo := event.Resource.CustomID
		return processGatewayPayment("paypal", event.ID, event.EventType, orderNo, event.Resource.ID, payload)
	case "CHECKOUT.ORDER.APPROVED":
		orderNo := event.Resource.CustomID
		if orderNo == "" && len(event.Resource.PurchaseUnits) > 0 {
			orderNo = event.Resource.PurchaseUnits[0].CustomID
		}
		captureID, err := provider.captureOrder(ctx, event.Resource.ID)
		if err != nil {
			return err // let PayPal retry on transient capture failures
		}
		return processGatewayPayment("paypal", event.ID, event.EventType, orderNo, captureID, payload)
	default:
		recordPaymentEvent("paypal", event.ID, event.EventType, "", payload)
		return nil
	}
}
