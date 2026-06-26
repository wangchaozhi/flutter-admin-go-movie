package payment

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"regexp"
	"strings"
)

// Shared crypto helpers for the Chinese payment gateways: WeChat Pay v3 (RSA
// request signing + AES-256-GCM callback decryption) and Alipay (RSA2 request
// signing + callback signature verification). Keeping them here keeps each
// provider file focused on its API surface.

var pemBodyCleaner = regexp.MustCompile(`\s+`)

// parseRSAPrivateKey accepts a PEM block (PKCS#1 or PKCS#8) or a bare base64
// key body and returns the RSA private key.
func parseRSAPrivateKey(input string) (*rsa.PrivateKey, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty private key")
	}
	der, err := decodeKeyDER(input, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

// parseRSAPublicKey accepts a PEM block (PKIX or PKCS#1) or a bare base64 key
// body (as Alipay commonly distributes its public key) and returns the key.
func parseRSAPublicKey(input string) (*rsa.PublicKey, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty public key")
	}
	der, err := decodeKeyDER(input, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	if parsed, err := x509.ParsePKIXPublicKey(der); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
		return nil, fmt.Errorf("public key is not RSA")
	}
	if key, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("parse public key failed")
}

// decodeKeyDER returns the DER bytes for a key given either a PEM block or a
// bare base64 body, wrapping bare bodies in the given PEM type as needed.
func decodeKeyDER(input, pemType string) ([]byte, error) {
	if strings.Contains(input, "-----BEGIN") {
		block, _ := pem.Decode([]byte(input))
		if block == nil {
			return nil, fmt.Errorf("invalid PEM block")
		}
		return block.Bytes, nil
	}
	body := pemBodyCleaner.ReplaceAllString(input, "")
	der, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 key body: %w", err)
	}
	_ = pemType
	return der, nil
}

// rsaSignSHA256 signs msg with RSA-SHA256 (PKCS#1 v1.5) and returns base64.
func rsaSignSHA256(key *rsa.PrivateKey, msg []byte) (string, error) {
	digest := sha256.Sum256(msg)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// rsaVerifySHA256 verifies a base64 RSA-SHA256 signature over msg.
func rsaVerifySHA256(key *rsa.PublicKey, msg []byte, signatureB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	digest := sha256.Sum256(msg)
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig)
}

// aesGCMDecrypt decrypts a WeChat Pay v3 callback resource. ciphertext is the
// base64-decoded value whose trailing 16 bytes are the GCM auth tag; successful
// decryption with the shared APIv3 key authenticates the payload.
func aesGCMDecrypt(apiV3Key, nonce, associatedData, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(apiV3Key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size")
	}
	return gcm.Open(nil, nonce, ciphertext, associatedData)
}
