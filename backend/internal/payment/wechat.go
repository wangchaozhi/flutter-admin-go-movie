package payment

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/store"
)

const wechatAPIBase = "https://api.mch.weixin.qq.com"

// CreateCheckout opens a WeChat Pay v3 Native (QR) transaction and returns the
// code_url the app renders as a QR code. The order number rides as out_trade_no
// so the callback maps the payment back to our order.
func (p wechatProvider) CreateCheckout(ctx context.Context, order store.Order, product store.Product) (CheckoutSession, error) {
	if err := p.requireConfig(); err != nil {
		return CheckoutSession{}, err
	}
	desc := product.Name
	if desc == "" {
		desc = product.Code
	}
	reqBody, _ := json.Marshal(map[string]any{
		"appid":        p.cfg.WeChatAppID,
		"mchid":        p.cfg.WeChatMchID,
		"description":  desc,
		"out_trade_no": order.OrderNo,
		"notify_url":   p.notifyURL(),
		"amount": map[string]any{
			"total":    order.AmountCents,
			"currency": defaultCNY(order.Currency),
		},
	})
	raw, status, err := p.signedRequest(ctx, http.MethodPost, "/v3/pay/transactions/native", reqBody)
	if err != nil {
		return CheckoutSession{}, err
	}
	if status >= 300 {
		return CheckoutSession{}, fmt.Errorf("wechat create order failed: %s", wechatError(raw, status))
	}
	var parsed struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.CodeURL == "" {
		return CheckoutSession{}, fmt.Errorf("wechat returned no code_url")
	}
	// WeChat assigns transaction_id only after payment; key the order by our
	// out_trade_no until the callback fills in the real transaction id.
	return CheckoutSession{ProviderOrderID: order.OrderNo, CheckoutURL: parsed.CodeURL}, nil
}

// Refund issues a full refund for the order's WeChat payment.
func (p wechatProvider) Refund(ctx context.Context, order store.Order) (RefundResult, error) {
	if err := p.requireConfig(); err != nil {
		return RefundResult{}, err
	}
	reqBody, _ := json.Marshal(map[string]any{
		"out_trade_no":  order.OrderNo,
		"out_refund_no": "RF" + order.OrderNo,
		"amount": map[string]any{
			"refund":   order.AmountCents,
			"total":    order.AmountCents,
			"currency": defaultCNY(order.Currency),
		},
	})
	raw, status, err := p.signedRequest(ctx, http.MethodPost, "/v3/refund/domestic/refunds", reqBody)
	if err != nil {
		return RefundResult{}, err
	}
	if status >= 300 {
		return RefundResult{}, fmt.Errorf("wechat refund failed: %s", wechatError(raw, status))
	}
	var parsed struct {
		RefundID string `json:"refund_id"`
	}
	_ = json.Unmarshal(raw, &parsed)
	return RefundResult{RefundID: parsed.RefundID}, nil
}

func (p wechatProvider) requireConfig() error {
	if p.cfg.WeChatMchID == "" || p.cfg.WeChatAppID == "" || p.cfg.WeChatPrivateKey == "" || p.cfg.WeChatSerialNo == "" {
		return fmt.Errorf("WeChat Pay (WECHAT_MCH_ID/APP_ID/SERIAL_NO/PRIVATE_KEY) is not configured")
	}
	return nil
}

func (p wechatProvider) notifyURL() string {
	if p.cfg.WeChatNotifyURL != "" {
		return p.cfg.WeChatNotifyURL
	}
	return strings.TrimRight(p.cfg.PublicBaseURL, "/") + "/api/webhooks/wechat"
}

// signedRequest performs a WeChat Pay v3 request with the merchant RSA signature
// in the Authorization header, per the v3 signing scheme.
func (p wechatProvider) signedRequest(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	key, err := parseRSAPrivateKey(p.cfg.WeChatPrivateKey)
	if err != nil {
		return nil, 0, fmt.Errorf("wechat private key: %w", err)
	}
	nonce, err := randomHex(16)
	if err != nil {
		return nil, 0, err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	message := method + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + string(body) + "\n"
	signature, err := rsaSignSHA256(key, []byte(message))
	if err != nil {
		return nil, 0, err
	}
	auth := fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		p.cfg.WeChatMchID, nonce, ts, p.cfg.WeChatSerialNo, signature,
	)

	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, wechatAPIBase+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := paymentHTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return raw, resp.StatusCode, err
}

// wechatWebhook decrypts and applies a WeChat Pay v3 payment notification. The
// AES-256-GCM decryption with the shared APIv3 key authenticates the payload.
func wechatWebhook(_ context.Context, cfg Config, _ *http.Request, body []byte) ackResponse {
	if len(cfg.WeChatAPIv3Key) != 32 {
		return ackJSON(http.StatusBadRequest, `{"code":"FAIL","message":"WECHAT_API_V3_KEY must be 32 bytes"}`)
	}
	var notify struct {
		ID        string `json:"id"`
		EventType string `json:"event_type"`
		Resource  struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &notify); err != nil {
		return ackJSON(http.StatusBadRequest, `{"code":"FAIL","message":"invalid body"}`)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(notify.Resource.Ciphertext)
	if err != nil {
		return ackJSON(http.StatusBadRequest, `{"code":"FAIL","message":"invalid ciphertext"}`)
	}
	plain, err := aesGCMDecrypt(
		[]byte(cfg.WeChatAPIv3Key),
		[]byte(notify.Resource.Nonce),
		[]byte(notify.Resource.AssociatedData),
		ciphertext,
	)
	if err != nil {
		// Failed authenticated-decryption means the payload is not from WeChat.
		return ackJSON(http.StatusBadRequest, `{"code":"FAIL","message":"decrypt failed"}`)
	}

	var resource struct {
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
	}
	if err := json.Unmarshal(plain, &resource); err != nil {
		return ackJSON(http.StatusBadRequest, `{"code":"FAIL","message":"invalid resource"}`)
	}
	if resource.TradeState != "SUCCESS" {
		return ackJSON(http.StatusOK, `{"code":"SUCCESS"}`) // acknowledge non-paid states
	}
	if err := processGatewayPayment("wechat", notify.ID, notify.EventType, resource.OutTradeNo, resource.TransactionID, body); err != nil {
		return ackJSON(http.StatusInternalServerError, `{"code":"FAIL","message":"processing failed"}`)
	}
	return ackJSON(http.StatusOK, `{"code":"SUCCESS"}`)
}

func wechatError(raw []byte, status int) string {
	var parsed struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &parsed) == nil && parsed.Message != "" {
		return parsed.Message
	}
	return "HTTP " + strconv.Itoa(status)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func defaultCNY(currency string) string {
	if strings.TrimSpace(currency) == "" {
		return "CNY"
	}
	return strings.ToUpper(currency)
}
