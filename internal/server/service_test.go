package server

import "testing"

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
