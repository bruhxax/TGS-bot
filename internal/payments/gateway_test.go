package payments

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestParseLavaWebhook(t *testing.T) {
	raw := []byte(`{"status":"success","amount":"199.00","order_id":"42","invoice_id":"lava-payment"}`)
	cfg := map[string]any{"enabled": true, "shop_id": "shop", "secret_key": "secret", "additional_key": "webhook-secret"}
	headers := http.Header{"Signature": {hmacHex(sha256.New, []byte("webhook-secret"), raw)}}

	result, err := parseLavaWebhook(cfg, headers, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Paid || result.MerchantOrderID != 42 || result.ExternalID != "lava-payment" || result.Amount != 199 {
		t.Fatalf("unexpected webhook result: %+v", result)
	}

	headers.Set("Signature", "invalid")
	if _, err = parseLavaWebhook(cfg, headers, raw); err == nil {
		t.Fatal("invalid LAVA signature was accepted")
	}
}

func TestParseFreeKassaWebhook(t *testing.T) {
	cfg := map[string]any{"enabled": true, "shop_id": "shop", "secret_word": "pay", "secret_word2": "notify"}
	form := url.Values{"MERCHANT_ID": {"shop"}, "AMOUNT": {"1490.00"}, "MERCHANT_ORDER_ID": {"77"}, "intid": {"fk-payment"}}
	signature := fmt.Sprintf("%x", md5.Sum([]byte("shop:1490.00:notify:77")))
	form.Set("SIGN", signature)

	result, err := parseFreeKassaWebhook(cfg, form)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Paid || result.MerchantOrderID != 77 || result.ExternalID != "fk-payment" || result.Currency != "RUB" {
		t.Fatalf("unexpected webhook result: %+v", result)
	}

	form.Set("AMOUNT", "1.00")
	if _, err = parseFreeKassaWebhook(cfg, form); err == nil {
		t.Fatal("tampered FreeKassa amount was accepted")
	}
}

func TestParsePallyWebhook(t *testing.T) {
	cfg := map[string]any{"enabled": true, "shop_id": "shop", "api_token": "token", "api_url": "https://example.test"}
	form := url.Values{"InvId": {"91"}, "OutSum": {"499.00"}, "Status": {"SUCCESS"}, "CurrencyIn": {"RUB"}}
	form.Set("SignatureValue", strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte("499.00:91:token")))))

	result, err := parsePallyWebhook(cfg, form)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Paid || result.MerchantOrderID != 91 || result.Amount != 499 || result.Currency != "RUB" {
		t.Fatalf("unexpected webhook result: %+v", result)
	}
}

func TestAlternativeProviderRequiresCompleteConfiguration(t *testing.T) {
	for _, provider := range AlternativeProviders {
		t.Run(provider, func(t *testing.T) {
			if err := validateAlternativeConfig(provider, map[string]any{"enabled": true}); err == nil {
				t.Fatal("incomplete configuration was accepted")
			}
		})
	}
}
