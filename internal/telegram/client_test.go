package telegram

import "testing"

func TestRenderHTMLEscapesTemplateValues(t *testing.T) {
	got := RenderHTML("Ответ в <b>{subject}</b>", map[string]string{"subject": `<важный & срочный>`})
	want := "Ответ в <b>&lt;важный &amp; срочный&gt;</b>"
	if got != want {
		t.Fatalf("RenderHTML() = %q; want %q", got, want)
	}
}

func TestStyledWebAppButton(t *testing.T) {
	button := StyledWebAppButton("Открыть", "https://example.com", "primary", "123456")
	if button["style"] != "primary" || button["icon_custom_emoji_id"] != "123456" {
		t.Fatalf("styled button fields were not preserved: %#v", button)
	}
	if _, ok := button["web_app"]; !ok {
		t.Fatalf("web_app target is missing: %#v", button)
	}
}
