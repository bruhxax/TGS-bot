package remnawave

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHonorsRuntimeSwitch(t *testing.T) {
	disabled := New(map[string]any{"remnawave": map[string]any{"enabled": false}}, "https://panel.example", "token")
	if disabled.Configured() {
		t.Fatal("disabled Remnawave integration used environment fallback")
	}
	enabled := New(map[string]any{"remnawave": map[string]any{"enabled": true}}, "https://panel.example", "token")
	if !enabled.Configured() {
		t.Fatal("enabled Remnawave integration did not use environment fallback")
	}
}

func TestUpdateUserPreservesSquadsWhenOmitted(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"uuid": "user-1"}})
	}))
	defer server.Close()
	client := &Client{BaseURL: server.URL, Token: "token", HTTP: server.Client()}
	traffic := int64(10)
	if _, err := client.UpdateUser(context.Background(), map[string]any{"uuid": "user-1"}, Entitlement{TrafficBytes: &traffic}); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["activeInternalSquads"]; ok {
		t.Fatal("internal squads were sent even though no squad change was requested")
	}
	if _, ok := body["externalSquadUuid"]; ok {
		t.Fatal("external squad was sent even though no squad change was requested")
	}
}
