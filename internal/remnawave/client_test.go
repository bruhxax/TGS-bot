package remnawave

import "testing"

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
