package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tgs-bot/internal/config"
	"tgs-bot/internal/store"
)

func TestCreateConnectHandoffUsesImmediateLaunch(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/connect/handoff", strings.NewReader(`{"scheme":"happ://add/","client_name":"Happ"}`))
	request = request.WithContext(context.WithValue(request.Context(), userKey, &store.User{SubscriptionURL: "https://example.com/sub"}))
	response := httptest.NewRecorder()
	server := &Server{Config: config.Config{AppSecret: "a-test-secret-that-is-long-enough", PublicBaseURL: "https://vpn.example"}}
	server.createConnectHandoff(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	pageURL, _ := payload["url"].(string)
	if !strings.HasPrefix(pageURL, "https://vpn.example/connect/") || !strings.HasSuffix(pageURL, "/launch") {
		t.Fatalf("url = %q, want immediate launch endpoint", pageURL)
	}
	if fallback, _ := payload["fallback_url"].(string); fallback != strings.TrimSuffix(pageURL, "/launch") {
		t.Fatalf("fallback_url = %q, want visible page for %q", fallback, pageURL)
	}
}

func TestConnectHandoffRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	want := connectHandoff{Scheme: "happ://add/", SubscriptionURL: "https://example.com/sub?id=7", ClientName: "Happ", ExpiresAt: now.Add(time.Minute).Unix()}
	token, err := sealConnectHandoff("a-test-secret-that-is-long-enough", want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := openConnectHandoff("a-test-secret-that-is-long-enough", token, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("handoff mismatch: got %#v, want %#v", got, want)
	}
	if strings.Contains(token, want.SubscriptionURL) {
		t.Fatal("subscription URL must not be visible in the handoff token")
	}
}

func TestConnectHandoffRejectsTamperAndExpiry(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	payload := connectHandoff{Scheme: "incy://import/", SubscriptionURL: "https://example.com/sub", ClientName: "INCY", ExpiresAt: now.Add(time.Second).Unix()}
	token, err := sealConnectHandoff("a-test-secret-that-is-long-enough", payload)
	if err != nil {
		t.Fatal(err)
	}
	last := token[len(token)-1]
	replacement := byte('A')
	if last == replacement {
		replacement = 'B'
	}
	if _, err := openConnectHandoff("a-test-secret-that-is-long-enough", token[:len(token)-1]+string(replacement), now); err == nil {
		t.Fatal("tampered token must be rejected")
	}
	if _, err := openConnectHandoff("a-test-secret-that-is-long-enough", token, now.Add(2*time.Second)); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestSafeClientScheme(t *testing.T) {
	for _, value := range []string{"javascript://alert(1)", "https://example.com", "happ://add/\n"} {
		if _, err := safeClientScheme(value); err == nil {
			t.Fatalf("unsafe scheme %q accepted", value)
		}
	}
	if got, err := safeClientScheme("happ://add/"); err != nil || got != "happ://add/" {
		t.Fatalf("valid client scheme rejected: %q, %v", got, err)
	}
}

func TestConnectHandoffLaunchRedirectsImmediately(t *testing.T) {
	const secret = "a-test-secret-that-is-long-enough"
	now := time.Now()
	payload := connectHandoff{Scheme: "happ://add/", SubscriptionURL: "https://example.com/sub", ClientName: "Happ", ExpiresAt: now.Add(time.Minute).Unix()}
	token, err := sealConnectHandoff(secret, payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/connect/"+token+"/launch", nil)
	request.SetPathValue("token", token)
	response := httptest.NewRecorder()
	(&Server{Config: config.Config{AppSecret: secret}}).connectHandoffLaunch(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusFound)
	}
	if location := response.Header().Get("Location"); location != "happ://add/https://example.com/sub" {
		t.Fatalf("Location = %q, want expected client URL", location)
	}
	if cache := response.Header().Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store", cache)
	}
	if refresh := response.Header().Get("Refresh"); refresh != "1; url=/connect/"+token {
		t.Fatalf("Refresh = %q, want visible fallback", refresh)
	}
	if body := response.Body.String(); !strings.Contains(body, "TGS VPN") || !strings.Contains(body, "Открыть Happ") {
		t.Fatalf("redirect response is missing visible fallback: %s", body)
	}
}

func TestConnectHandoffPageProvidesMinimalFallback(t *testing.T) {
	const secret = "a-test-secret-that-is-long-enough"
	payload := connectHandoff{Scheme: "happ://add/", SubscriptionURL: "https://example.com/sub", ClientName: "Happ", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	token, err := sealConnectHandoff(secret, payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/connect/"+token, nil)
	request.SetPathValue("token", token)
	response := httptest.NewRecorder()
	(&Server{Config: config.Config{AppSecret: secret}}).connectHandoffPage(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, "happ://add/https://example.com/sub") || !strings.Contains(body, "TGS VPN") {
		t.Fatalf("fallback page is missing client action or branding: %s", body)
	}
	if strings.Contains(body, "location.href=") || strings.Contains(body, "<script") {
		t.Fatal("fallback page must not rely on a blocked scripted launch")
	}
}
