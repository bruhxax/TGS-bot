package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Client struct {
	Token string
	HTTP  *http.Client
}

func New(token string) *Client {
	return &Client{Token: token, HTTP: &http.Client{Timeout: 65 * time.Second}}
}

type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}
type Chat struct {
	ID int64 `json:"id"`
}
type Message struct {
	MessageID int    `json:"message_id"`
	From      User   `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	Caption   string `json:"caption"`
}
type CallbackQuery struct {
	ID      string  `json:"id"`
	From    User    `json:"from"`
	Message Message `json:"message"`
	Data    string  `json:"data"`
}
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

func (c *Client) Call(ctx context.Context, method string, payload any, result any) error {
	raw, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+c.Token+"/"+method, bytes.NewReader(raw))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if e = json.Unmarshal(data, &envelope); e != nil {
		return e
	}
	if !envelope.OK {
		return fmt.Errorf("Telegram %s: %s", method, envelope.Description)
	}
	if result != nil && len(envelope.Result) > 0 {
		return json.Unmarshal(envelope.Result, result)
	}
	return nil
}
func (c *Client) Send(ctx context.Context, chatID int64, text string, markup any) error {
	body := map[string]any{"chat_id": chatID, "text": text}
	if markup != nil {
		body["reply_markup"] = markup
	}
	return c.Call(ctx, "sendMessage", body, nil)
}
func (c *Client) SendHTML(ctx context.Context, chatID int64, text string, markup any) error {
	body := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML"}
	if markup != nil {
		body["reply_markup"] = markup
	}
	return c.Call(ctx, "sendMessage", body, nil)
}
func (c *Client) CopyMessage(ctx context.Context, chatID, fromChatID int64, messageID int, markup any) (int, error) {
	body := map[string]any{"chat_id": chatID, "from_chat_id": fromChatID, "message_id": messageID}
	if markup != nil {
		body["reply_markup"] = markup
	}
	var result struct {
		MessageID int `json:"message_id"`
	}
	err := c.Call(ctx, "copyMessage", body, &result)
	return result.MessageID, err
}
func (c *Client) AnswerCallback(ctx context.Context, id, text string) error {
	return c.Call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}
func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	var out []Update
	e := c.Call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 55, "allowed_updates": []string{"message", "callback_query"}}, &out)
	return out, e
}
func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.Call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}
func (c *Client) SetMenuButton(ctx context.Context, webURL string) error {
	return c.Call(ctx, "setChatMenuButton", map[string]any{"menu_button": map[string]any{"type": "web_app", "text": "TGS-bot", "web_app": map[string]string{"url": webURL}}}, nil)
}
func (c *Client) SetCommands(ctx context.Context) error {
	return c.Call(ctx, "setMyCommands", map[string]any{"commands": []map[string]string{{"command": "start", "description": "Открыть TGS-bot"}, {"command": "myid", "description": "Показать Telegram ID"}, {"command": "broadcast", "description": "Создать рассылку (админ)"}}}, nil)
}
func (c *Client) SetName(ctx context.Context, name string) error {
	return c.Call(ctx, "setMyName", map[string]any{"name": name}, nil)
}
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	e := c.Call(ctx, "getMe", map[string]any{}, &u)
	return u, e
}
func WebAppButton(text, link string) map[string]any {
	return map[string]any{"text": text, "web_app": map[string]string{"url": link}}
}
func StyledWebAppButton(text, link, style, customEmojiID string) map[string]any {
	button := WebAppButton(text, link)
	if style == "primary" || style == "success" || style == "danger" {
		button["style"] = style
	}
	if customEmojiID != "" {
		button["icon_custom_emoji_id"] = customEmojiID
	}
	return button
}
func URLButton(text, link string) map[string]any { return map[string]any{"text": text, "url": link} }
func StyledURLButton(text, link, style, customEmojiID string) map[string]any {
	button := map[string]any{"text": text, "url": link}
	if style == "primary" || style == "success" || style == "danger" {
		button["style"] = style
	}
	if customEmojiID != "" {
		button["icon_custom_emoji_id"] = customEmojiID
	}
	return button
}
func CallbackButton(text, data string) map[string]any {
	return map[string]any{"text": text, "callback_data": data}
}
func QueryEscape(v string) string { return url.QueryEscape(v) }

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func RenderHTML(template string, values map[string]string) string {
	if len(values) == 0 {
		return template
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	replacements := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		replacements = append(replacements, "{"+key+"}", html.EscapeString(values[key]))
	}
	return strings.NewReplacer(replacements...).Replace(template)
}

func PlainText(value string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTagPattern.ReplaceAllString(value, "")))
}
