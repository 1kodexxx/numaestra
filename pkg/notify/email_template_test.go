package notify

import (
	"strings"
	"testing"
)

func TestEmailBrandHeader_WithPublicURL(t *testing.T) {
	n := NewSmtpNotifier("localhost", 587, "u", "p", "from@test.com", "Numaestra", "", "https://numaestra.ru")
	header := n.emailBrandHeader()

	if !strings.Contains(header, "https://numaestra.ru/email-logo.png") {
		t.Error("ожидали абсолютный URL логотипа")
	}
	if !strings.Contains(header, ">Numaestra</span>") {
		t.Error("ожидали название бренда")
	}
	if !strings.Contains(header, `href="https://numaestra.ru"`) {
		t.Error("ожидали ссылку на сайт")
	}
}

func TestEmailDocument_UnifiedLayout(t *testing.T) {
	n := NewSmtpNotifier("localhost", 587, "u", "p", "from@test.com", "Numaestra", "", "https://numaestra.ru")
	doc := n.emailDocument("Тест", emailHero("🎵", "Заголовок", "Подзаголовок"), "", "слоган")

	for _, want := range []string{
		"email-logo.png",
		"Numaestra",
		"#0f0f0f",
		"rgba(0,229,192",
		"Заголовок",
		"слоган",
		"Персональные песни на заказ",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("документ должен содержать %q", want)
		}
	}
}
