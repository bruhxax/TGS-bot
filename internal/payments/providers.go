package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Provider struct{ HTTP *http.Client }

func New() *Provider { return &Provider{HTTP: &http.Client{Timeout: 25 * time.Second}} }

func stringValue(m map[string]any, key string) string { v, _ := m[key].(string); return v }
func boolValue(m map[string]any, key string) bool     { v, _ := m[key].(bool); return v }

func (p *Provider) YooKassaCreate(ctx context.Context, cfg map[string]any, id string, kopecks int64, description, returnURL string) (string, string, error) {
	shop, secret := stringValue(cfg, "shop_id"), stringValue(cfg, "secret_key")
	if !boolValue(cfg, "enabled") || shop == "" || secret == "" {
		return "", "", fmt.Errorf("ЮKassa не настроена")
	}
	amount := fmt.Sprintf("%d.%02d", kopecks/100, kopecks%100)
	body := map[string]any{"amount": map[string]string{"value": amount, "currency": "RUB"}, "capture": true, "confirmation": map[string]string{"type": "redirect", "return_url": returnURL}, "description": description, "metadata": map[string]string{"tgs_payment_id": id}}
	if email := stringValue(cfg, "email"); email != "" {
		body["receipt"] = map[string]any{"customer": map[string]string{"email": email}, "items": []any{map[string]any{"description": description, "quantity": "1.00", "amount": map[string]string{"value": amount, "currency": "RUB"}, "vat_code": 1}}}
	}
	raw, _ := json.Marshal(body)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.yookassa.ru/v3/payments", bytes.NewReader(raw))
	if e != nil {
		return "", "", e
	}
	req.SetBasicAuth(shop, secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", id)
	resp, e := p.HTTP.Do(req)
	if e != nil {
		return "", "", e
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("ЮKassa HTTP %d", resp.StatusCode)
	}
	var out struct {
		ID           string `json:"id"`
		Confirmation struct {
			URL string `json:"confirmation_url"`
		} `json:"confirmation"`
	}
	if e = json.Unmarshal(data, &out); e != nil {
		return "", "", e
	}
	return out.ID, out.Confirmation.URL, nil
}

func (p *Provider) YooKassaGet(ctx context.Context, cfg map[string]any, id string) (map[string]any, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.yookassa.ru/v3/payments/"+id, nil)
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth(stringValue(cfg, "shop_id"), stringValue(cfg, "secret_key"))
	resp, e := p.HTTP.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ЮKassa verification HTTP %d", resp.StatusCode)
	}
	var out map[string]any
	e = json.NewDecoder(resp.Body).Decode(&out)
	return out, e
}

func (p *Provider) CryptoBotCreate(ctx context.Context, cfg map[string]any, id string, kopecks int64, description string) (string, string, error) {
	token := stringValue(cfg, "token")
	if !boolValue(cfg, "enabled") || token == "" {
		return "", "", fmt.Errorf("CryptoBot не настроен")
	}
	base := "https://pay.crypt.bot"
	if boolValue(cfg, "testnet") {
		base = "https://testnet-pay.crypt.bot"
	}
	body := map[string]any{"currency_type": "fiat", "fiat": "RUB", "accepted_assets": "USDT,TON,BTC,ETH,LTC,BNB,TRX,USDC", "amount": fmt.Sprintf("%d.%02d", kopecks/100, kopecks%100), "description": description, "payload": id, "expires_in": 3600}
	raw, _ := json.Marshal(body)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/createInvoice", bytes.NewReader(raw))
	if e != nil {
		return "", "", e
	}
	req.Header.Set("Crypto-Pay-API-Token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, e := p.HTTP.Do(req)
	if e != nil {
		return "", "", e
	}
	defer resp.Body.Close()
	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			InvoiceID any    `json:"invoice_id"`
			MiniURL   string `json:"mini_app_invoice_url"`
			BotURL    string `json:"bot_invoice_url"`
		} `json:"result"`
	}
	if e = json.NewDecoder(resp.Body).Decode(&out); e != nil {
		return "", "", e
	}
	if !out.OK {
		return "", "", fmt.Errorf("CryptoBot отклонил запрос")
	}
	u := out.Result.MiniURL
	if u == "" {
		u = out.Result.BotURL
	}
	return fmt.Sprint(out.Result.InvoiceID), u, nil
}

func VerifyCryptoBot(token string, body []byte, signature string) bool {
	sum := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, sum[:])
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (p *Provider) Test(ctx context.Context, kind string, cfg map[string]any) error {
	switch kind {
	case "yookassa":
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.yookassa.ru/v3/me", nil)
		req.SetBasicAuth(stringValue(cfg, "shop_id"), stringValue(cfg, "secret_key"))
		resp, e := p.HTTP.Do(req)
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return nil
	case "cryptobot":
		base := "https://pay.crypt.bot"
		if boolValue(cfg, "testnet") {
			base = "https://testnet-pay.crypt.bot"
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/getMe", nil)
		req.Header.Set("Crypto-Pay-API-Token", stringValue(cfg, "token"))
		resp, e := p.HTTP.Do(req)
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		var out struct {
			OK bool `json:"ok"`
		}
		e = json.NewDecoder(resp.Body).Decode(&out)
		if e != nil || !out.OK {
			return fmt.Errorf("CryptoBot rejected token")
		}
		return nil
	}
	for _, provider := range AlternativeProviders {
		if provider == kind {
			return validateAlternativeConfig(kind, cfg)
		}
	}
	return fmt.Errorf("unknown provider %s", strconv.Quote(kind))
}
