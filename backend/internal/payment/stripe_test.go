package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func signStripe(payload []byte, secret string, at time.Time) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d", at.Unix())))
	mac.Write([]byte("."))
	mac.Write(payload)
	return fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

func TestVerifyStripeSignature(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_1","type":"checkout.session.completed"}`)
	header := signStripe(payload, secret, time.Now())

	if err := verifyStripeSignature(payload, header, secret); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := verifyStripeSignature(payload, header, "whsec_other"); err == nil {
		t.Fatal("expected mismatch with wrong secret")
	}
	if err := verifyStripeSignature([]byte(`{"id":"evt_2"}`), header, secret); err == nil {
		t.Fatal("expected mismatch with tampered payload")
	}
	stale := signStripe(payload, secret, time.Now().Add(-10*time.Minute))
	if err := verifyStripeSignature(payload, stale, secret); err == nil {
		t.Fatal("expected stale timestamp to be rejected")
	}
	if err := verifyStripeSignature(payload, "garbage", secret); err == nil {
		t.Fatal("expected malformed header to be rejected")
	}
}

func TestPaypalAmountValue(t *testing.T) {
	cases := []struct {
		currency string
		cents    int
		want     string
	}{
		{"USD", 999, "9.99"},
		{"EUR", 100, "1.00"},
		{"JPY", 500, "500"}, // zero-decimal
		{"TWD", 300, "300"}, // zero-decimal
	}
	for _, c := range cases {
		if got := paypalAmountValue(c.currency, c.cents); got != c.want {
			t.Errorf("paypalAmountValue(%s,%d) = %s, want %s", c.currency, c.cents, got, c.want)
		}
	}
}
