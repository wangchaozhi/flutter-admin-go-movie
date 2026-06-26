package payment

import (
	"net/http"
	"time"
)

// paymentHTTPClient is the shared client for outbound calls to payment gateways.
// A bounded timeout keeps a slow gateway from tying up a request goroutine.
var paymentHTTPClient = &http.Client{Timeout: 20 * time.Second}
