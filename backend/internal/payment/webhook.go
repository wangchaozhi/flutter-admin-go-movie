package payment

import (
	"context"
	"io"
	"net/http"
	"strings"

	"flutter-admin-go/internal/common"
)

// ackResponse is the raw acknowledgement written back to a payment gateway after
// handling its callback. Each gateway expects its own success body/format.
type ackResponse struct {
	status      int
	contentType string
	body        []byte
}

func ackJSON(status int, body string) ackResponse {
	return ackResponse{status: status, contentType: "application/json", body: []byte(body)}
}

func ackText(status int, body string) ackResponse {
	return ackResponse{status: status, contentType: "text/plain; charset=utf-8", body: []byte(body)}
}

// webhookFunc verifies and applies a gateway callback and returns the ack to
// send back. It owns the HTTP status: 2xx acknowledges, 4xx rejects a bad/
// unverifiable callback (no retry), 5xx asks the gateway to retry.
type webhookFunc func(ctx context.Context, cfg Config, r *http.Request, body []byte) ackResponse

// webhookRegistry maps a provider to its callback handler. Adding a gateway is a
// single entry here plus the handler in that provider's file — the dispatcher
// and routing need no change.
var webhookRegistry = map[string]webhookFunc{
	"stripe": stripeWebhook,
	"paypal": paypalWebhook,
	"wechat": wechatWebhook,
	"alipay": alipayWebhook,
}

// WebhookHandler dispatches POST /api/webhooks/{provider} to the registered
// handler for that gateway.
func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/webhooks/"), "/"))
	fn, ok := webhookRegistry[provider]
	if !ok {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "unknown webhook provider"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "read body failed"})
		return
	}
	ack := fn(r.Context(), LoadConfig(), r, body)
	w.Header().Set("Content-Type", ack.contentType)
	w.WriteHeader(ack.status)
	_, _ = w.Write(ack.body)
}
