package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tgs-bot/internal/config"
)

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

func TestConnectHandoffPageLaunchesClient(t *testing.T) {
	const secret = "a-test-secret-that-is-long-enough"
	now := time.Now()
	payload := connectHandoff{Scheme: "happ://add/", SubscriptionURL: "https://example.com/sub", ClientName: "Happ", ExpiresAt: now.Add(time.Minute).Unix()}
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
	if !strings.Contains(body, "happ://add/https://example.com/sub") || !strings.Contains(body, "location.href=") {
		t.Fatalf("handoff page does not launch the expected client: %s", body)
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "script-src 'nonce-") {
		t.Fatal("handoff page must restrict inline scripts with a nonce")
	}
}
