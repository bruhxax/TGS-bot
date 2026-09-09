package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(settings map[string]any, fallbackURL, fallbackToken string) *Client {
	item, _ := settings["remnawave"].(map[string]any)
	enabled, _ := item["enabled"].(bool)
	base, _ := item["url"].(string)
	token, _ := item["token"].(string)
	if base == "" {
		base = fallbackURL
	}
	if token == "" {
		token = fallbackToken
	}
	if !enabled {
		base = ""
		token = ""
	}
	return &Client{BaseURL: strings.TrimRight(base, "/"), Token: token, HTTP: &http.Client{Timeout: 25 * time.Second}}
}

func (c *Client) Configured() bool { return c.BaseURL != "" && c.Token != "" }

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body any) (any, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("интеграция Remnawave не настроена")
	}
	var reader io.Reader
	if body != nil {
		raw, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		reader = bytes.NewReader(raw)
	}
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, e := http.NewRequestWithContext(ctx, method, u, reader)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Remnawave HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var envelope map[string]any
	if e = json.Unmarshal(raw, &envelope); e != nil {
		return nil, e
	}
	if v, ok := envelope["response"]; ok {
		return v, nil
	}
	return envelope, nil
}

func (c *Client) UserByTelegram(ctx context.Context, id int64) (map[string]any, error) {
	q := url.Values{"telegramId": {strconv.FormatInt(id, 10)}, "size": {"10"}}
	data, e := c.request(ctx, http.MethodGet, "/api/users/stream", q, nil)
	if e == nil {
		if obj, ok := data.(map[string]any); ok {
			if users, ok := obj["users"].([]any); ok && len(users) > 0 {
				if u, ok := users[0].(map[string]any); ok {
					return u, nil
				}
			}
			return nil, nil
		}
	}
	data, e2 := c.request(ctx, http.MethodGet, "/api/users/by-telegram-id/"+strconv.FormatInt(id, 10), nil, nil)
	if e2 != nil {
		return nil, e
	}
	if list, ok := data.([]any); ok {
		if len(list) == 0 {
			return nil, nil
		}
		if u, ok := list[0].(map[string]any); ok {
			return u, nil
		}
	}
	u, _ := data.(map[string]any)
	return u, nil
}

type Entitlement struct {
	TelegramID        int64
	Username          string
	Status            *string
	ExpireAt          *time.Time
	TrafficBytes      *int64
	DeviceLimit       *int
	InternalSquads    *[]string
	ExternalSquadUUID *string
}

func (c *Client) CreateUser(ctx context.Context, e Entitlement) (map[string]any, error) {
	body := map[string]any{"username": e.Username, "status": "ACTIVE", "trafficLimitStrategy": "NO_RESET", "telegramId": e.TelegramID, "description": "Managed by TGS-bot", "tag": "TGS_BOT"}
	applyEntitlementBody(body, e)
	if _, ok := body["expireAt"]; !ok {
		body["expireAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if _, ok := body["trafficLimitBytes"]; !ok {
		body["trafficLimitBytes"] = int64(0)
	}
	if _, ok := body["hwidDeviceLimit"]; !ok {
		body["hwidDeviceLimit"] = 1
	}
	data, err := c.request(ctx, http.MethodPost, "/api/users", nil, body)
	if err != nil {
		return nil, err
	}
	out, _ := data.(map[string]any)
	return out, nil
}

func (c *Client) UpdateUser(ctx context.Context, user map[string]any, e Entitlement) (map[string]any, error) {
	body := map[string]any{}
	applyEntitlementBody(body, e)
	if id, ok := numberInt64(user["id"]); ok {
		body["id"] = id
	} else {
		body["uuid"] = user["uuid"]
	}
	data, err := c.request(ctx, http.MethodPatch, "/api/users", nil, body)
	if err != nil {
		return nil, err
	}
	out, _ := data.(map[string]any)
	return out, nil
}

func applyEntitlementBody(body map[string]any, e Entitlement) {
	if e.Status != nil {
		body["status"] = *e.Status
	}
	if e.ExpireAt != nil {
		body["expireAt"] = e.ExpireAt.Format(time.RFC3339Nano)
	}
	if e.TrafficBytes != nil {
		body["trafficLimitBytes"] = *e.TrafficBytes
	}
	if e.DeviceLimit != nil {
		body["hwidDeviceLimit"] = *e.DeviceLimit
	}
	if e.InternalSquads != nil {
		body["activeInternalSquads"] = *e.InternalSquads
	}
	if e.ExternalSquadUUID != nil {
		if *e.ExternalSquadUUID == "" {
			body["externalSquadUuid"] = nil
		} else {
			body["externalSquadUuid"] = *e.ExternalSquadUUID
		}
	}
}

func (c *Client) UpdateStatus(ctx context.Context, user map[string]any, status string) (map[string]any, error) {
	body := map[string]any{"status": status}
	if id, ok := numberInt64(user["id"]); ok {
		body["id"] = id
	} else {
		body["uuid"] = user["uuid"]
	}
	data, err := c.request(ctx, http.MethodPatch, "/api/users", nil, body)
	if err != nil {
		return nil, err
	}
	out, _ := data.(map[string]any)
	return out, nil
}

func (c *Client) Nodes(ctx context.Context) ([]map[string]any, error) {
	return c.list(ctx, "/api/nodes", "nodes")
}
func (c *Client) InternalSquads(ctx context.Context) ([]map[string]any, error) {
	items, e := c.list(ctx, "/api/internal-squads", "internalSquads")
	if e == nil && len(items) == 0 {
		return c.list(ctx, "/api/internal-squads", "squads")
	}
	return items, e
}
func (c *Client) ExternalSquads(ctx context.Context) ([]map[string]any, error) {
	items, e := c.list(ctx, "/api/external-squads", "externalSquads")
	if e == nil && len(items) == 0 {
		return c.list(ctx, "/api/external-squads", "squads")
	}
	return items, e
}
func (c *Client) list(ctx context.Context, path, key string) ([]map[string]any, error) {
	data, e := c.request(ctx, http.MethodGet, path, nil, nil)
	if e != nil {
		return nil, e
	}
	if obj, ok := data.(map[string]any); ok {
		data = obj[key]
	}
	raw, ok := data.([]any)
	if !ok {
		return []map[string]any{}, nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		if item, ok := v.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out, nil
}

func (c *Client) Devices(ctx context.Context, user map[string]any) ([]map[string]any, error) {
	identity := ""
	if id, ok := numberInt64(user["id"]); ok {
		identity = strconv.FormatInt(id, 10)
	} else {
		identity = fmt.Sprint(user["uuid"])
	}
	data, e := c.request(ctx, http.MethodGet, "/api/hwid/devices/"+url.PathEscape(identity), nil, nil)
	if e != nil {
		return nil, e
	}
	if obj, ok := data.(map[string]any); ok {
		data = obj["devices"]
	}
	raw, _ := data.([]any)
	out := []map[string]any{}
	for _, v := range raw {
		if item, ok := v.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out, nil
}
func (c *Client) DeleteDevice(ctx context.Context, user map[string]any, hwid string) ([]map[string]any, error) {
	body := map[string]any{"hwid": hwid}
	if id, ok := numberInt64(user["id"]); ok {
		body["userId"] = id
	} else {
		body["userUuid"] = user["uuid"]
	}
	data, e := c.request(ctx, http.MethodPost, "/api/hwid/devices/delete", nil, body)
	if e != nil {
		return nil, e
	}
	if obj, ok := data.(map[string]any); ok {
		data = obj["devices"]
	}
	raw, _ := data.([]any)
	out := []map[string]any{}
	for _, v := range raw {
		if item, ok := v.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out, nil
}

func numberInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, e := n.Int64()
		return i, e == nil
	}
	return 0, false
}
func truncate(v string, n int) string {
	if len(v) <= n {
		return v
	}
	return v[:n]
}
