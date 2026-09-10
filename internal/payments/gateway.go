package payments

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var AlternativeProviders = []string{"lava", "wata", "platega", "freekassa", "heleket", "pally"}

type CreateRequest struct {
	MerchantOrderID int64
	AmountKopecks   int64
	Description     string
	ReturnURL       string
	WebhookBaseURL  string
	TelegramID      int64
	Username        string
}

type WebhookResult struct {
	MerchantOrderID int64
	ExternalID      string
	Amount          float64
	Currency        string
	Paid            bool
	Cancelled       bool
}

func providerEnabled(cfg map[string]any) error {
	if !boolValue(cfg, "enabled") {
		return errors.New("платёжная интеграция выключена")
	}
	return nil
}

func requireFields(cfg map[string]any, names ...string) error {
	for _, name := range names {
		if strings.TrimSpace(stringValue(cfg, name)) == "" || stringValue(cfg, name) == "••••••••" {
			return fmt.Errorf("не заполнено поле %s", name)
		}
	}
	return nil
}

func validateAlternativeConfig(provider string, cfg map[string]any) error {
	if err := providerEnabled(cfg); err != nil {
		return err
	}
	switch provider {
	case "lava":
		return requireFields(cfg, "shop_id", "secret_key", "additional_key")
	case "wata":
		return requireFields(cfg, "access_token", "api_url")
	case "platega":
		return requireFields(cfg, "merchant_id", "secret_key", "api_url")
	case "freekassa":
		return requireFields(cfg, "shop_id", "secret_word", "secret_word2")
	case "heleket":
		return requireFields(cfg, "merchant_id", "api_key", "api_url")
	case "pally":
		return requireFields(cfg, "shop_id", "api_token", "api_url")
	default:
		return fmt.Errorf("неизвестная платёжная интеграция %q", provider)
	}
}

func (p *Provider) CreateAlternative(ctx context.Context, provider string, cfg map[string]any, input CreateRequest) (string, string, error) {
	if err := validateAlternativeConfig(provider, cfg); err != nil {
		return "", "", err
	}
	switch provider {
	case "lava":
		return p.createLava(ctx, cfg, input)
	case "wata":
		return p.createWata(ctx, cfg, input)
	case "platega":
		return p.createPlatega(ctx, cfg, input)
	case "freekassa":
		return p.createFreeKassa(cfg, input)
	case "heleket":
		return p.createHeleket(ctx, cfg, input)
	case "pally":
		return p.createPally(ctx, cfg, input)
	default:
		return "", "", fmt.Errorf("неизвестный способ оплаты")
	}
}

func webhookURL(base, provider string) string {
	return strings.TrimRight(base, "/") + "/api/webhooks/payments/" + provider
}

func amountFloat(kopecks int64) float64 { return float64(kopecks) / 100 }
func formatAmount(value float64) string { return strconv.FormatFloat(value, 'f', 2, 64) }

func (p *Provider) createLava(ctx context.Context, cfg map[string]any, input CreateRequest) (string, string, error) {
	payload := map[string]any{
		"sum": amountFloat(input.AmountKopecks), "orderId": strconv.FormatInt(input.MerchantOrderID, 10), "shopId": stringValue(cfg, "shop_id"),
		"hookUrl": webhookURL(input.WebhookBaseURL, "lava"), "successUrl": input.ReturnURL, "failUrl": input.ReturnURL,
		"expire": 60, "comment": input.Description,
	}
	raw, _ := json.Marshal(payload)
	var response struct {
		Data struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := p.doJSON(ctx, http.MethodPost, "https://api.lava.ru/business/invoice/create", raw, map[string]string{"Signature": hmacHex(sha256.New, []byte(stringValue(cfg, "secret_key")), raw)}, &response); err != nil {
		return "", "", err
	}
	if response.Data.ID == "" || response.Data.URL == "" {
		return "", "", fmt.Errorf("LAVA не вернула ссылку на оплату: %s", response.Error)
	}
	return response.Data.ID, response.Data.URL, nil
}

func (p *Provider) createWata(ctx context.Context, cfg map[string]any, input CreateRequest) (string, string, error) {
	payload := map[string]any{"amount": amountFloat(input.AmountKopecks), "currency": "RUB", "description": input.Description, "orderId": strconv.FormatInt(input.MerchantOrderID, 10), "successRedirectUrl": input.ReturnURL, "failRedirectUrl": input.ReturnURL}
	raw, _ := json.Marshal(payload)
	var response struct{ ID, URL string }
	endpoint := strings.TrimRight(stringValue(cfg, "api_url"), "/") + "/links"
	if err := p.doJSON(ctx, http.MethodPost, endpoint, raw, map[string]string{"Authorization": "Bearer " + stringValue(cfg, "access_token")}, &response); err != nil {
		return "", "", err
	}
	if response.ID == "" || response.URL == "" {
		return "", "", errors.New("WATA не вернула ссылку на оплату")
	}
	return response.ID, response.URL, nil
}

func (p *Provider) createPlatega(ctx context.Context, cfg map[string]any, input CreateRequest) (string, string, error) {
	payload := map[string]any{
		"paymentDetails": map[string]any{"amount": amountFloat(input.AmountKopecks), "currency": "RUB"},
		"description":    input.Description, "return": input.ReturnURL, "failedUrl": input.ReturnURL,
		"payload":  strconv.FormatInt(input.MerchantOrderID, 10),
		"metadata": map[string]string{"userId": strconv.FormatInt(input.TelegramID, 10), "userName": input.Username},
	}
	raw, _ := json.Marshal(payload)
	var response struct {
		TransactionID string `json:"transactionId"`
		URL           string `json:"url"`
	}
	endpoint := strings.TrimRight(stringValue(cfg, "api_url"), "/") + "/v2/transaction/process"
	if err := p.doJSON(ctx, http.MethodPost, endpoint, raw, map[string]string{"X-MerchantId": stringValue(cfg, "merchant_id"), "X-Secret": stringValue(cfg, "secret_key")}, &response); err != nil {
		return "", "", err
	}
	if response.TransactionID == "" || response.URL == "" {
		return "", "", errors.New("Platega не вернула ссылку на оплату")
	}
	return response.TransactionID, response.URL, nil
}

func (p *Provider) createFreeKassa(cfg map[string]any, input CreateRequest) (string, string, error) {
	amount := formatAmount(amountFloat(input.AmountKopecks))
	orderID := strconv.FormatInt(input.MerchantOrderID, 10)
	signRaw := strings.Join([]string{stringValue(cfg, "shop_id"), amount, stringValue(cfg, "secret_word"), "RUB", orderID}, ":")
	sign := fmt.Sprintf("%x", md5.Sum([]byte(signRaw)))
	query := url.Values{"m": {stringValue(cfg, "shop_id")}, "oa": {amount}, "currency": {"RUB"}, "o": {orderID}, "s": {sign}, "lang": {"ru"}}
	return orderID, "https://pay.fk.money/?" + query.Encode(), nil
}

func (p *Provider) createHeleket(ctx context.Context, cfg map[string]any, input CreateRequest) (string, string, error) {
	orderID := strconv.FormatInt(input.MerchantOrderID, 10)
	payload := map[string]any{"amount": formatAmount(amountFloat(input.AmountKopecks)), "currency": "RUB", "order_id": orderID, "url_return": input.ReturnURL, "url_success": input.ReturnURL, "url_callback": webhookURL(input.WebhookBaseURL, "heleket"), "lifetime": 3600, "theme": "dark", "additional_data": strconv.FormatInt(input.TelegramID, 10)}
	raw, _ := json.Marshal(payload)
	sign := fmt.Sprintf("%x", md5.Sum([]byte(base64.StdEncoding.EncodeToString(raw)+stringValue(cfg, "api_key"))))
	var response struct {
		State   int    `json:"state"`
		Message string `json:"message"`
		Result  struct {
			UUID string `json:"uuid"`
			URL  string `json:"url"`
		} `json:"result"`
	}
	endpoint := strings.TrimRight(stringValue(cfg, "api_url"), "/") + "/v1/payment"
	if err := p.doJSON(ctx, http.MethodPost, endpoint, raw, map[string]string{"merchant": stringValue(cfg, "merchant_id"), "sign": sign}, &response); err != nil {
		return "", "", err
	}
	if response.State != 0 || response.Result.UUID == "" || response.Result.URL == "" {
		return "", "", fmt.Errorf("Heleket: %s", response.Message)
	}
	return response.Result.UUID, response.Result.URL, nil
}

func (p *Provider) createPally(ctx context.Context, cfg map[string]any, input CreateRequest) (string, string, error) {
	orderID := strconv.FormatInt(input.MerchantOrderID, 10)
	form := url.Values{"amount": {formatAmount(amountFloat(input.AmountKopecks))}, "order_id": {orderID}, "description": {input.Description}, "type": {"normal"}, "shop_id": {stringValue(cfg, "shop_id")}, "currency_in": {"RUB"}, "custom": {orderID}, "name": {input.Description}}
	endpoint := strings.TrimRight(stringValue(cfg, "api_url"), "/") + "/api/v1/bill/create"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+stringValue(cfg, "api_token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("Pally HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var response struct {
		Success     json.RawMessage `json:"success"`
		LinkPageURL string          `json:"link_page_url"`
		BillID      json.RawMessage `json:"bill_id"`
		Message     string          `json:"message"`
		Error       string          `json:"error"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return "", "", err
	}
	billID, _ := jsonScalarString(response.BillID)
	if !jsonTruthy(response.Success) || strings.TrimSpace(response.LinkPageURL) == "" || strings.TrimSpace(billID) == "" {
		message := response.Message
		if message == "" {
			message = response.Error
		}
		if message == "" {
			message = "Pally не вернула ссылку на оплату"
		}
		return "", "", errors.New(message)
	}
	return billID, response.LinkPageURL, nil
}

func (p *Provider) doJSON(ctx context.Context, method, endpoint string, body []byte, headers map[string]string, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("платёжный API вернул HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if target != nil && len(raw) > 0 {
		if err = json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("некорректный ответ платёжного API: %w", err)
		}
	}
	return nil
}

func (p *Provider) ParseAlternativeWebhook(ctx context.Context, provider string, cfg map[string]any, headers http.Header, raw []byte, form url.Values) (WebhookResult, error) {
	if err := validateAlternativeConfig(provider, cfg); err != nil {
		return WebhookResult{}, err
	}
	switch provider {
	case "lava":
		return parseLavaWebhook(cfg, headers, raw)
	case "wata":
		return p.parseWataWebhook(ctx, cfg, headers, raw)
	case "platega":
		return parsePlategaWebhook(cfg, headers, raw)
	case "freekassa":
		return parseFreeKassaWebhook(cfg, form)
	case "heleket":
		return parseHeleketWebhook(cfg, raw)
	case "pally":
		return parsePallyWebhook(cfg, form)
	default:
		return WebhookResult{}, errors.New("неизвестный webhook")
	}
}

func parseLavaWebhook(cfg map[string]any, headers http.Header, raw []byte) (WebhookResult, error) {
	var payload struct {
		InvoiceID string          `json:"invoice_id"`
		OrderID   json.RawMessage `json:"order_id"`
		Status    string          `json:"status"`
		Amount    json.RawMessage `json:"amount"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return WebhookResult{}, err
	}
	signature := strings.TrimSpace(headers.Get("Signature"))
	expected := hmacHex(sha256.New, []byte(stringValue(cfg, "additional_key")), raw)
	if signature == "" || !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(signature))) {
		return WebhookResult{}, errors.New("неверная подпись LAVA")
	}
	orderID, err := jsonScalarString(payload.OrderID)
	if err != nil {
		return WebhookResult{}, err
	}
	merchantID, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return WebhookResult{}, err
	}
	amountText, err := jsonScalarString(payload.Amount)
	if err != nil {
		return WebhookResult{}, err
	}
	amount, err := strconv.ParseFloat(amountText, 64)
	if err != nil {
		return WebhookResult{}, err
	}
	status := strings.ToLower(payload.Status)
	return WebhookResult{MerchantOrderID: merchantID, ExternalID: payload.InvoiceID, Amount: amount, Currency: "RUB", Paid: status == "success" || status == "paid", Cancelled: status == "cancel" || status == "failed" || status == "expired"}, nil
}

func jsonScalarString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", errors.New("пустое JSON-значение")
	}
	var value string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return value, nil
	}
	return strings.TrimSpace(string(raw)), nil
}

func (p *Provider) parseWataWebhook(ctx context.Context, cfg map[string]any, headers http.Header, raw []byte) (WebhookResult, error) {
	signature := strings.TrimSpace(headers.Get("X-Signature"))
	if signature == "" {
		return WebhookResult{}, errors.New("нет подписи WATA")
	}
	publicKey, err := p.fetchWataPublicKey(ctx, cfg)
	if err != nil {
		return WebhookResult{}, err
	}
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return WebhookResult{}, err
	}
	digest := sha512.Sum512(raw)
	if err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA512, digest[:], sigBytes); err != nil {
		return WebhookResult{}, errors.New("неверная подпись WATA")
	}
	var payload struct {
		ID                string  `json:"id"`
		TransactionStatus string  `json:"transactionStatus"`
		Currency          string  `json:"currency"`
		OrderID           string  `json:"orderId"`
		Amount            float64 `json:"amount"`
	}
	if err = json.Unmarshal(raw, &payload); err != nil {
		return WebhookResult{}, err
	}
	merchantID, err := strconv.ParseInt(payload.OrderID, 10, 64)
	if err != nil {
		return WebhookResult{}, err
	}
	status := strings.ToLower(payload.TransactionStatus)
	return WebhookResult{MerchantOrderID: merchantID, ExternalID: payload.ID, Amount: payload.Amount, Currency: payload.Currency, Paid: status == "paid", Cancelled: status == "declined"}, nil
}

func (p *Provider) fetchWataPublicKey(ctx context.Context, cfg map[string]any) (*rsa.PublicKey, error) {
	endpoint := strings.TrimRight(stringValue(cfg, "api_url"), "/") + "/public-key"
	var response struct {
		Value string `json:"value"`
	}
	if err := p.doJSON(ctx, http.MethodGet, endpoint, nil, map[string]string{"Authorization": "Bearer " + stringValue(cfg, "access_token")}, &response); err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(response.Value))
	if block == nil {
		return nil, errors.New("некорректный публичный ключ WATA")
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("публичный ключ WATA не является RSA")
}

func parsePlategaWebhook(cfg map[string]any, headers http.Header, raw []byte) (WebhookResult, error) {
	if !hmac.Equal([]byte(headers.Get("X-MerchantId")), []byte(stringValue(cfg, "merchant_id"))) || !hmac.Equal([]byte(headers.Get("X-Secret")), []byte(stringValue(cfg, "secret_key"))) {
		return WebhookResult{}, errors.New("неверные данные webhook Platega")
	}
	var payload struct {
		ID       string  `json:"id"`
		Currency string  `json:"currency"`
		Status   string  `json:"status"`
		Amount   float64 `json:"amount"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return WebhookResult{}, err
	}
	status := strings.ToLower(payload.Status)
	return WebhookResult{ExternalID: payload.ID, Amount: payload.Amount, Currency: payload.Currency, Paid: status == "confirmed", Cancelled: status == "canceled" || status == "chargebacked"}, nil
}

func parseFreeKassaWebhook(cfg map[string]any, form url.Values) (WebhookResult, error) {
	merchantID, amount, orderID, signature := form.Get("MERCHANT_ID"), form.Get("AMOUNT"), form.Get("MERCHANT_ORDER_ID"), form.Get("SIGN")
	expected := fmt.Sprintf("%x", md5.Sum([]byte(strings.Join([]string{merchantID, amount, stringValue(cfg, "secret_word2"), orderID}, ":"))))
	if merchantID != stringValue(cfg, "shop_id") || !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(signature))) {
		return WebhookResult{}, errors.New("неверная подпись FreeKassa")
	}
	merchantOrderID, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return WebhookResult{}, err
	}
	value, _ := strconv.ParseFloat(strings.ReplaceAll(amount, ",", "."), 64)
	return WebhookResult{MerchantOrderID: merchantOrderID, ExternalID: form.Get("intid"), Amount: value, Currency: "RUB", Paid: true}, nil
}

func parseHeleketWebhook(cfg map[string]any, raw []byte) (WebhookResult, error) {
	var payload struct {
		UUID          string `json:"uuid"`
		OrderID       string `json:"order_id"`
		Amount        string `json:"amount"`
		Currency      string `json:"currency"`
		PaymentStatus string `json:"status"`
		Sign          string `json:"sign"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return WebhookResult{}, err
	}
	unsigned := regexp.MustCompile(`,\s*"sign"\s*:\s*"[^"]*"\s*}`).ReplaceAll(raw, []byte("}"))
	if bytes.Equal(unsigned, raw) {
		return WebhookResult{}, errors.New("некорректный webhook Heleket")
	}
	expected := fmt.Sprintf("%x", md5.Sum([]byte(base64.StdEncoding.EncodeToString(unsigned)+stringValue(cfg, "api_key"))))
	if !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(payload.Sign))) {
		return WebhookResult{}, errors.New("неверная подпись Heleket")
	}
	merchantOrderID, err := strconv.ParseInt(payload.OrderID, 10, 64)
	if err != nil {
		return WebhookResult{}, err
	}
	value, _ := strconv.ParseFloat(payload.Amount, 64)
	status := strings.ToLower(payload.PaymentStatus)
	return WebhookResult{MerchantOrderID: merchantOrderID, ExternalID: payload.UUID, Amount: value, Currency: payload.Currency, Paid: status == "paid" || status == "paid_over", Cancelled: status == "cancel" || status == "fail" || status == "wrong_amount" || status == "system_fail" || strings.HasPrefix(status, "refund")}, nil
}

func parsePallyWebhook(cfg map[string]any, form url.Values) (WebhookResult, error) {
	invoiceID, amountText, signature := strings.TrimSpace(form.Get("InvId")), strings.TrimSpace(form.Get("OutSum")), strings.TrimSpace(form.Get("SignatureValue"))
	if invoiceID == "" || amountText == "" || signature == "" {
		return WebhookResult{}, errors.New("некорректный webhook Pally")
	}
	expected := strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(amountText+":"+invoiceID+":"+stringValue(cfg, "api_token")))))
	if !hmac.Equal([]byte(expected), []byte(strings.ToUpper(signature))) {
		return WebhookResult{}, errors.New("неверная подпись Pally")
	}
	merchantOrderID, err := strconv.ParseInt(invoiceID, 10, 64)
	if err != nil {
		return WebhookResult{}, err
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(amountText, ",", "."), 64)
	if err != nil {
		return WebhookResult{}, err
	}
	status := strings.ToUpper(strings.TrimSpace(form.Get("Status")))
	return WebhookResult{MerchantOrderID: merchantOrderID, Amount: value, Currency: strings.ToUpper(strings.TrimSpace(form.Get("CurrencyIn"))), Paid: status == "SUCCESS" || status == "OVERPAID", Cancelled: status == "FAIL" || status == "CANCELED" || status == "CANCELLED"}, nil
}

func jsonTruthy(raw json.RawMessage) bool {
	value, err := jsonScalarString(raw)
	if err != nil {
		return false
	}
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "success"
}

func hmacHex(newHash func() hash.Hash, key, raw []byte) string {
	mac := hmac.New(newHash, key)
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}
