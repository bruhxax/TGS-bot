package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyCryptoBot(t *testing.T) {
	token := "crypto-pay-token"
	body := []byte(`{"update_type":"invoice_paid","payload":{"payload":"payment-id"}}`)
	key := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, key[:])
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	if !VerifyCryptoBot(token, body, signature) {
		t.Fatal("valid CryptoBot signature was rejected")
	}
	if VerifyCryptoBot(token, append(body, ' '), signature) {
		t.Fatal("tampered CryptoBot payload was accepted")
	}
}
