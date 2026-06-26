package payment

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

func testKeyPEMs(t *testing.T) (privPEM, pubPEM string, key *rsa.PrivateKey) {
	t.Helper()
	var err error
	key, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal priv: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return privPEM, pubPEM, key
}

func TestRSASignVerifyRoundTrip(t *testing.T) {
	privPEM, pubPEM, _ := testKeyPEMs(t)
	priv, err := parseRSAPrivateKey(privPEM)
	if err != nil {
		t.Fatalf("parse priv PEM: %v", err)
	}
	pub, err := parseRSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("parse pub PEM: %v", err)
	}

	msg := []byte("app_id=2021&method=alipay.trade.precreate")
	sig, err := rsaSignSHA256(priv, msg)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := rsaVerifySHA256(pub, msg, sig); err != nil {
		t.Fatalf("verify valid signature failed: %v", err)
	}
	if err := rsaVerifySHA256(pub, []byte("tampered"), sig); err == nil {
		t.Fatal("expected verification of tampered message to fail")
	}
}

func TestParseKeysAcceptBareBase64(t *testing.T) {
	privPEM, pubPEM, _ := testKeyPEMs(t)
	// Strip PEM armor to the bare base64 body, as Alipay distributes its keys.
	privBlock, _ := pem.Decode([]byte(privPEM))
	pubBlock, _ := pem.Decode([]byte(pubPEM))
	bareProv := base64.StdEncoding.EncodeToString(privBlock.Bytes)
	barePub := base64.StdEncoding.EncodeToString(pubBlock.Bytes)

	if _, err := parseRSAPrivateKey(bareProv); err != nil {
		t.Fatalf("parse bare private key: %v", err)
	}
	if _, err := parseRSAPublicKey(barePub); err != nil {
		t.Fatalf("parse bare public key: %v", err)
	}
}

func TestAESGCMDecrypt(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	nonce := []byte("123456789012")                   // 12 bytes
	aad := []byte("transaction")
	plaintext := []byte(`{"out_trade_no":"ORD1","trade_state":"SUCCESS"}`)

	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	got, err := aesGCMDecrypt(key, nonce, aad, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("decrypt mismatch: %s", got)
	}
	// Wrong AAD must fail authentication.
	if _, err := aesGCMDecrypt(key, nonce, []byte("wrong"), ciphertext); err == nil {
		t.Fatal("expected GCM auth failure with wrong AAD")
	}
}

func TestBuildAlipaySignContent(t *testing.T) {
	params := map[string]string{
		"app_id":    "2021",
		"method":    "alipay.trade.precreate",
		"sign":      "SIG",
		"sign_type": "RSA2",
		"charset":   "utf-8",
		"empty":     "",
	}
	// Verifying a notify excludes sign + sign_type + empties, sorted by key.
	got := buildAlipaySignContent(params, true)
	want := "app_id=2021&charset=utf-8&method=alipay.trade.precreate"
	if got != want {
		t.Fatalf("notify sign content = %q, want %q", got, want)
	}
	// Signing a request keeps sign_type but still drops sign + empties.
	gotReq := buildAlipaySignContent(params, false)
	wantReq := "app_id=2021&charset=utf-8&method=alipay.trade.precreate&sign_type=RSA2"
	if gotReq != wantReq {
		t.Fatalf("request sign content = %q, want %q", gotReq, wantReq)
	}
}
