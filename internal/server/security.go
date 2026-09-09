package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"tgs-bot/internal/store"
)

type sessionPayload struct {
	TelegramID int64 `json:"telegram_id"`
	ExpiresAt  int64 `json:"expires_at"`
}

func verifyInitData(raw, botToken string, maxAge time.Duration) (store.TelegramUser, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return store.TelegramUser{}, err
	}
	received := values.Get("hash")
	if received == "" {
		return store.TelegramUser{}, fmt.Errorf("Telegram initData hash is missing")
	}
	values.Del("hash")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	check := strings.Join(parts, "\n")
	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	secretMac.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secretMac.Sum(nil))
	mac.Write([]byte(check))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(received)) {
		return store.TelegramUser{}, fmt.Errorf("Telegram initData signature is invalid")
	}
	authDate, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil || authDate <= 0 || time.Since(time.Unix(authDate, 0)) > maxAge {
		return store.TelegramUser{}, fmt.Errorf("Telegram initData has expired")
	}
	var user store.TelegramUser
	if err = json.Unmarshal([]byte(values.Get("user")), &user); err != nil || user.ID <= 0 {
		return store.TelegramUser{}, fmt.Errorf("Telegram user data is invalid")
	}
	return user, nil
}

func issueSession(secret string, telegramID int64) string {
	raw, _ := json.Marshal(sessionPayload{TelegramID: telegramID, ExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix()})
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func parseSession(secret, token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, fmt.Errorf("bad token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return 0, fmt.Errorf("bad signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, err
	}
	var p sessionPayload
	if err = json.Unmarshal(raw, &p); err != nil {
		return 0, err
	}
	if p.TelegramID <= 0 || p.ExpiresAt < time.Now().Unix() {
		return 0, fmt.Errorf("expired")
	}
	return p.TelegramID, nil
}
