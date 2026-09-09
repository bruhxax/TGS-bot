package server

import (
	"testing"
	"time"

	"tgs-bot/internal/store"
)

func TestDiscounted(t *testing.T) {
	tests := []struct {
		price   int64
		percent int
		want    int64
	}{
		{30_000, 0, 30_000},
		{30_000, 15, 25_500},
		{30_000, 100, 100},
		{99, 0, 100},
		{30_000, -10, 30_000},
	}
	for _, test := range tests {
		if got := discounted(test.price, test.percent); got != test.want {
			t.Errorf("discounted(%d, %d) = %d; want %d", test.price, test.percent, got, test.want)
		}
	}
}

func TestPreserveMaskedSecrets(t *testing.T) {
	current := map[string]any{
		"remnawave": map[string]any{"token": "real-token", "url": "https://old.example"},
		"yookassa":  map[string]any{"secret_key": "real-secret"},
	}
	incoming := map[string]any{
		"remnawave": map[string]any{"token": maskedSecret, "url": "https://new.example"},
		"yookassa":  map[string]any{"secret_key": "replacement"},
	}
	got := preserveMaskedSecrets(current, incoming)
	if got["remnawave"].(map[string]any)["token"] != "real-token" {
		t.Fatal("masked token was not preserved")
	}
	if got["remnawave"].(map[string]any)["url"] != "https://new.example" {
		t.Fatal("ordinary setting was unexpectedly reverted")
	}
	if got["yookassa"].(map[string]any)["secret_key"] != "replacement" {
		t.Fatal("explicit secret replacement was not saved")
	}
}

func TestEntitlementTraffic(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	activeUntil := now.Add(24 * time.Hour)
	expiredAt := now.Add(-24 * time.Hour)
	tests := []struct {
		name    string
		current int64
		status  string
		expires *time.Time
		gb      int
		mode    trafficMode
		want    int64
		update  bool
	}{
		{name: "days only keeps limited traffic", current: 100 * gigabyte, status: "ACTIVE", expires: &activeUntil, mode: trafficKeep, want: 100 * gigabyte, update: false},
		{name: "active plan adds allowance", current: 100 * gigabyte, status: "ACTIVE", expires: &activeUntil, gb: 50, mode: trafficPlan, want: 150 * gigabyte, update: true},
		{name: "expired plan replaces stale allowance", current: 100 * gigabyte, status: "ACTIVE", expires: &expiredAt, gb: 50, mode: trafficPlan, want: 50 * gigabyte, update: true},
		{name: "active unlimited remains unlimited", current: 0, status: "ACTIVE", expires: &activeUntil, gb: 50, mode: trafficPlan, want: 0, update: true},
		{name: "zero bonus does not enable unlimited", current: 100 * gigabyte, status: "ACTIVE", expires: &activeUntil, mode: trafficAdd, want: 100 * gigabyte, update: false},
		{name: "explicit unlimited", current: 100 * gigabyte, status: "ACTIVE", expires: &activeUntil, mode: trafficUnlimited, want: 0, update: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, update := entitlementTraffic(test.current, test.status, test.expires, test.gb, test.mode, now)
			if got != test.want || update != test.update {
				t.Fatalf("got (%d, %t), want (%d, %t)", got, update, test.want, test.update)
			}
		})
	}
}

func TestSyncRemoteReadsCurrentRemnawaveTrafficShape(t *testing.T) {
	user := store.User{TrafficUsedBytes: 1, DeviceLimit: 3}
	syncRemote(&user, map[string]any{
		"userTraffic":       map[string]any{"usedTrafficBytes": float64(58 * gigabyte)},
		"trafficLimitBytes": float64(100 * gigabyte),
		"hwidDeviceLimit":   nil,
	})

	if user.TrafficUsedBytes != 58*gigabyte {
		t.Fatalf("used traffic = %d; want %d", user.TrafficUsedBytes, 58*gigabyte)
	}
	if user.TrafficLimitBytes != 100*gigabyte {
		t.Fatalf("traffic limit = %d; want %d", user.TrafficLimitBytes, 100*gigabyte)
	}
	if user.DeviceLimit != 0 {
		t.Fatalf("device limit = %d; want 0 for unlimited", user.DeviceLimit)
	}
}

func TestSyncRemoteKeepsLegacyTrafficCompatibility(t *testing.T) {
	user := store.User{}
	syncRemote(&user, map[string]any{"usedTrafficBytes": float64(12 * gigabyte)})
	if user.TrafficUsedBytes != 12*gigabyte {
		t.Fatalf("used traffic = %d; want legacy value", user.TrafficUsedBytes)
	}
}
