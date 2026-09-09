package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func signedInitData(t *testing.T, token string, authDate time.Time) string {
	t.Helper()
	values := url.Values{
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
		"query_id":  {"AAHdF6IQAAAAAN0XohDhrOrc"},
		"user":      {`{"id":424242,"first_name":"Test","username":"tester","language_code":"ru"}`},
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(parts, "\n")))
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return values.Encode()
}

func TestVerifyInitData(t *testing.T) {
	token := "123456:secret"
	raw := signedInitData(t, token, time.Now())
	user, err := verifyInitData(raw, token, time.Hour)
	if err != nil {
		t.Fatalf("verifyInitData returned error: %v", err)
	}
	if user.ID != 424242 || user.Username != "tester" {
		t.Fatalf("unexpected user: %#v", user)
	}

	tampered := strings.Replace(raw, "tester", "intruder", 1)
	if _, err = verifyInitData(tampered, token, time.Hour); err == nil {
		t.Fatal("tampered initData was accepted")
	}
	if _, err = verifyInitData(signedInitData(t, token, time.Now().Add(-2*time.Hour)), token, time.Hour); err == nil {
		t.Fatal("expired initData was accepted")
	}
}

func TestSessionRoundTripAndTamper(t *testing.T) {
	token := issueSession("a sufficiently long test secret", 424242)
	id, err := parseSession("a sufficiently long test secret", token)
	if err != nil || id != 424242 {
		t.Fatalf("session round trip failed: id=%d err=%v", id, err)
	}
	if _, err = parseSession("another sufficiently long secret", token); err == nil {
		t.Fatal("session with wrong signing secret was accepted")
	}
}
