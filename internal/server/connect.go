package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	qrcode "github.com/skip2/go-qrcode"
)

const connectHandoffLifetime = 15 * time.Minute

type connectHandoff struct {
	Scheme          string `json:"scheme"`
	SubscriptionURL string `json:"subscription_url"`
	ClientName      string `json:"client_name"`
	ExpiresAt       int64  `json:"expires_at"`
}

func (s *Server) createConnectHandoff(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scheme     string `json:"scheme"`
		ClientName string `json:"client_name"`
	}
	if !decode(w, r, &body) {
		return
	}
	scheme, err := safeClientScheme(body.Scheme)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clientName := strings.TrimSpace(body.ClientName)
	if clientName == "" || len([]rune(clientName)) > 48 {
		writeError(w, http.StatusBadRequest, "Некорректное название приложения")
		return
	}
	u := current(r)
	if u.SubscriptionURL == "" {
		writeError(w, http.StatusBadRequest, "Ссылка подписки ещё не создана")
		return
	}
	if err := safeSubscriptionURL(u.SubscriptionURL); err != nil {
		writeError(w, http.StatusBadRequest, "Ссылка подписки недоступна")
		return
	}
	payload := connectHandoff{
		Scheme:          scheme,
		SubscriptionURL: u.SubscriptionURL,
		ClientName:      clientName,
		ExpiresAt:       time.Now().Add(connectHandoffLifetime).Unix(),
	}
	token, err := sealConnectHandoff(s.Config.AppSecret, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось подготовить переход")
		return
	}
	handoffPageURL := strings.TrimRight(s.Config.PublicBaseURL, "/") + "/connect/" + token
	// The QR is consumed by VPN clients, so it must contain the subscription
	// itself. The browser handoff URL is intentionally used only by the button.
	png, err := qrcode.Encode(u.SubscriptionURL, qrcode.Medium, 512)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать QR-код")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":          handoffPageURL + "/launch",
		"fallback_url": handoffPageURL,
		"qr_data_url":  "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		"expires_at":   payload.ExpiresAt,
	})
}

func (s *Server) connectHandoffLaunch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Referrer-Policy", "no-referrer")
	payload, err := openConnectHandoff(s.Config.AppSecret, r.PathValue("token"), time.Now())
	if err != nil {
		connectPageError(w, http.StatusGone, "Ссылка устарела", "Вернитесь в Mini App и нажмите «Добавить подписку» ещё раз.")
		return
	}
	w.Header().Set("Location", payload.Scheme+payload.SubscriptionURL)
	w.WriteHeader(http.StatusFound)
}

func (s *Server) connectHandoffPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Referrer-Policy", "no-referrer")
	payload, err := openConnectHandoff(s.Config.AppSecret, r.PathValue("token"), time.Now())
	if err != nil {
		connectPageError(w, http.StatusGone, "Ссылка устарела", "Вернитесь в Mini App и нажмите «Добавить подписку» ещё раз.")
		return
	}
	target := payload.Scheme + payload.SubscriptionURL
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = connectPageTemplate.Execute(w, map[string]any{
		"Client": payload.ClientName,
		"Target": template.URL(target), // target is restricted to a validated custom scheme and HTTPS subscription URL.
	})
}

func connectPageError(w http.ResponseWriter, status int, title, text string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = connectErrorTemplate.Execute(w, map[string]string{"Title": title, "Text": text})
}

func safeClientScheme(raw string) (string, error) {
	if raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("Некорректная ссылка приложения")
	}
	if raw == "" || len(raw) > 256 || !strings.Contains(raw, "://") {
		return "", fmt.Errorf("Некорректная ссылка приложения")
	}
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("\"'<>`\\", r) {
			return "", fmt.Errorf("Некорректная ссылка приложения")
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return "", fmt.Errorf("Некорректная ссылка приложения")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "javascript", "data", "file", "blob":
		return "", fmt.Errorf("Разрешена только ссылка приложения")
	}
	return raw, nil
}

func safeSubscriptionURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("invalid subscription URL")
	}
	return nil
}

func sealConnectHandoff(secret string, payload connectHandoff) (string, error) {
	gcm, err := connectCipher(secret)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, raw, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func openConnectHandoff(secret, token string, now time.Time) (connectHandoff, error) {
	var payload connectHandoff
	if token == "" || len(token) > 4096 {
		return payload, fmt.Errorf("invalid token")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return payload, err
	}
	gcm, err := connectCipher(secret)
	if err != nil {
		return payload, err
	}
	if len(sealed) < gcm.NonceSize() {
		return payload, fmt.Errorf("invalid token")
	}
	raw, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, err
	}
	if payload.ExpiresAt <= now.Unix() || safeSubscriptionURL(payload.SubscriptionURL) != nil {
		return connectHandoff{}, fmt.Errorf("expired token")
	}
	if _, err := safeClientScheme(payload.Scheme); err != nil {
		return connectHandoff{}, err
	}
	return payload, nil
}

func connectCipher(secret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

var connectPageTemplate = template.Must(template.New("connect").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#111315"><title>Открыть {{.Client}}</title>
<style>:root{color-scheme:dark}*{box-sizing:border-box}body{margin:0;min-height:100dvh;display:grid;place-items:center;padding:28px 22px;background:#111315;color:#fff;font:15px/1.5 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{width:min(100%,340px);display:flex;flex-direction:column;align-items:center;text-align:center}.mark{width:48px;height:48px;margin:0 0 18px;color:#2aabee}.mark svg{width:100%;height:100%;display:block}h1{margin:0;font-size:23px;line-height:1.15;letter-spacing:-.35px}p{margin:8px 0 26px;color:#8f969e}.button{min-height:48px;width:100%;display:flex;align-items:center;justify-content:center;padding:0 18px;border:1px solid #299fd8;border-radius:11px;background:#2aabee;color:#07131a;font-weight:760;text-decoration:none}.button:focus-visible{outline:3px solid #ffffff;outline-offset:3px}.hint{margin:12px 0 0;font-size:12px;color:#697078}@media(prefers-reduced-motion:reduce){*{scroll-behavior:auto!important}}</style></head>
<body><main><div class="mark" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none"><path d="M9.6 14.4l4.8-4.8M7.25 16.75l-1.1 1.1a3.32 3.32 0 0 1-4.7-4.7l3.1-3.1a3.32 3.32 0 0 1 4.7 0M16.75 7.25l1.1-1.1a3.32 3.32 0 0 1 4.7 4.7l-3.1 3.1a3.32 3.32 0 0 1-4.7 0" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg></div><h1>TGS VPN</h1><p>Открываем {{.Client}}</p><a class="button" href="{{.Target}}">Открыть {{.Client}}</a><p class="hint">Если окно не появилось, нажмите кнопку</p></main></body></html>`))

var connectErrorTemplate = template.Must(template.New("connect-error").Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>:root{color-scheme:dark}*{box-sizing:border-box}body{margin:0;min-height:100dvh;display:grid;place-items:center;padding:24px;background:#111315;color:#fff;font:15px/1.5 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{width:min(100%,400px)}h1{margin:0 0 10px;font-size:26px}p{margin:0;color:#9da3aa}</style></head><body><main><h1>{{.Title}}</h1><p>{{.Text}}</p></main></body></html>`))
