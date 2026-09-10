package server

import "testing"

func TestValidateSubpageSetting(t *testing.T) {
	valid := map[string]any{
		"include_builtins": false,
		"clients": []any{map[string]any{
			"id": "my-client", "name": "My Client", "scheme": "myclient://add/",
			"install_url": "https://example.com/download", "enabled": true,
			"featured": true, "all_platforms": false, "platforms": []any{"ios", "android"},
		}},
	}
	if err := validateSubpageSetting(valid); err != nil {
		t.Fatalf("valid subpage setting was rejected: %v", err)
	}

	unsafe := map[string]any{
		"include_builtins": false,
		"clients": []any{map[string]any{
			"id": "unsafe", "name": "Unsafe", "scheme": "javascript://run/",
			"enabled": true, "all_platforms": true,
		}},
	}
	if err := validateSubpageSetting(unsafe); err == nil {
		t.Fatal("unsafe client scheme was accepted")
	}

	empty := map[string]any{"include_builtins": false, "clients": []any{}}
	if err := validateSubpageSetting(empty); err == nil {
		t.Fatal("subpage without any enabled client was accepted")
	}
}
