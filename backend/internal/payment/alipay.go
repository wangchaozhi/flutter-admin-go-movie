package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/store"
)

// CreateCheckout opens an Alipay precreate (QR) order and returns the qr_code
// the app renders. out_trade_no carries our order number for the async notify.
func (p alipayProvider) CreateCheckout(ctx context.Context, order store.Order, product store.Product) (CheckoutSession, error) {
	if p.cfg.AlipayAppID == "" || p.cfg.AlipayPrivateKey == "" {
		return CheckoutSession{}, fmt.Errorf("Alipay (ALIPAY_APP_ID/PRIVATE_KEY) is not configured")
	}
	subject := product.Name
	if subject == "" {
		subject = product.Code
	}
	node, err := p.call(ctx, "alipay.trade.precreate", map[string]any{
		"out_trade_no": order.OrderNo,
		"total_amount": alipayAmount(order.AmountCents),
		"subject":      subject,
	}, map[string]string{"notify_url": p.notifyURL()})
	if err != nil {
		return CheckoutSession{}, err
	}
	var parsed struct {
		Code   string `json:"code"`
		SubMsg string `json:"sub_msg"`
		QRCode string `json:"qr_code"`
	}
	_ = json.Unmarshal(node, &parsed)
	if parsed.Code != "10000" || parsed.QRCode == "" {
		return CheckoutSession{}, fmt.Errorf("alipay create order failed: %s", firstNonEmpty(parsed.SubMsg, parsed.Code))
	}
	return CheckoutSession{ProviderOrderID: order.OrderNo, CheckoutURL: parsed.QRCode}, nil
}

// Refund issues a full refund for the order's Alipay payment.
func (p alipayProvider) Refund(ctx context.Context, order store.Order) (RefundResult, error) {
	if p.cfg.AlipayAppID == "" || p.cfg.AlipayPrivateKey == "" {
		return RefundResult{}, fmt.Errorf("Alipay (ALIPAY_APP_ID/PRIVATE_KEY) is not configured")
	}
	node, err := p.call(ctx, "alipay.trade.refund", map[string]any{
		"out_trade_no":  order.OrderNo,
		"refund_amount": alipayAmount(order.AmountCents),
	}, nil)
	if err != nil {
		return RefundResult{}, err
	}
	var parsed struct {
		Code    string `json:"code"`
		SubMsg  string `json:"sub_msg"`
		TradeNo string `json:"trade_no"`
	}
	_ = json.Unmarshal(node, &parsed)
	if parsed.Code != "10000" {
		return RefundResult{}, fmt.Errorf("alipay refund failed: %s", firstNonEmpty(parsed.SubMsg, parsed.Code))
	}
	return RefundResult{RefundID: parsed.TradeNo}, nil
}

func (p alipayProvider) gateway() string {
	if p.cfg.AlipayGateway != "" {
		return p.cfg.AlipayGateway
	}
	return "https://openapi.alipay.com/gateway.do"
}

func (p alipayProvider) notifyURL() string {
	if p.cfg.AlipayNotifyURL != "" {
		return p.cfg.AlipayNotifyURL
	}
	return strings.TrimRight(p.cfg.PublicBaseURL, "/") + "/api/webhooks/alipay"
}

// call signs and sends an OpenAPI request (RSA2), returning the inner response
// node (e.g. alipay_trade_precreate_response) as raw JSON.
func (p alipayProvider) call(ctx context.Context, method string, biz map[string]any, extra map[string]string) (json.RawMessage, error) {
	key, err := parseRSAPrivateKey(p.cfg.AlipayPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("alipay private key: %w", err)
	}
	bizJSON, err := json.Marshal(biz)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"app_id":      p.cfg.AlipayAppID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(bizJSON),
	}
	for k, v := range extra {
		if v != "" {
			params[k] = v
		}
	}
	signature, err := rsaSignSHA256(key, []byte(buildAlipaySignContent(params, false)))
	if err != nil {
		return nil, err
	}
	params["sign"] = signature

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.gateway(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := paymentHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("alipay: invalid response")
	}
	nodeKey := strings.ReplaceAll(method, ".", "_") + "_response"
	node, ok := top[nodeKey]
	if !ok {
		return nil, fmt.Errorf("alipay: missing %s", nodeKey)
	}
	return node, nil
}

// alipayWebhook verifies the async notification signature with the Alipay public
// key and, on a successful trade, marks the order paid.
func alipayWebhook(_ context.Context, cfg Config, _ *http.Request, body []byte) ackResponse {
	if cfg.AlipayPublicKey == "" {
		return ackText(http.StatusBadRequest, "failure")
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return ackText(http.StatusBadRequest, "failure")
	}
	params := make(map[string]string, len(values))
	for k := range values {
		params[k] = values.Get(k)
	}

	pub, err := parseRSAPublicKey(cfg.AlipayPublicKey)
	if err != nil {
		return ackText(http.StatusBadRequest, "failure")
	}
	if err := rsaVerifySHA256(pub, []byte(buildAlipaySignContent(params, true)), params["sign"]); err != nil {
		return ackText(http.StatusBadRequest, "failure")
	}

	status := params["trade_status"]
	if status != "TRADE_SUCCESS" && status != "TRADE_FINISHED" {
		return ackText(http.StatusOK, "success") // acknowledge non-paid states
	}
	if err := processGatewayPayment("alipay", params["notify_id"], status, params["out_trade_no"], params["trade_no"], body); err != nil {
		return ackText(http.StatusInternalServerError, "failure")
	}
	return ackText(http.StatusOK, "success")
}

// buildAlipaySignContent builds the RSA2 sign string: parameters sorted by key,
// skipping empties and the sign field (and sign_type when verifying a notify),
// joined as k=v&.... Values are used as-is (already URL-decoded on notify).
func buildAlipaySignContent(params map[string]string, excludeSignType bool) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" || k == "sign" {
			continue
		}
		if excludeSignType && k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func alipayAmount(cents int) string {
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
