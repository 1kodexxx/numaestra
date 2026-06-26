package suno

import (
	"strings"
	"testing"
)

func TestEncodeDecodePrompt_RoundTrip(t *testing.T) {
	raw := EncodePrompt("modern pop, male vocals", "Write a Russian love song about Ivan and Maria.")
	got, ok := DecodePrompt(raw)
	if !ok {
		t.Fatal("ожидали декодирование")
	}
	if got.Tags != "modern pop, male vocals" {
		t.Errorf("tags: %q", got.Tags)
	}
	if got.Description == "" {
		t.Error("пустое описание")
	}
}

func TestDecodePrompt_LegacyPlainText(t *testing.T) {
	_, ok := DecodePrompt("Create a happy pop song about birthday")
	if ok {
		t.Error("legacy plain text не должен декодироваться")
	}
}

func TestBuildStyleTagsFromAnswers_DedupesRussian(t *testing.T) {
	tags := BuildStyleTagsFromAnswers(map[string]string{
		"GENRE": "modern pop",
		"MOOD":  "emotional, touching",
		"VOCAL": "male vocals",
		"TEMPO": "slow tempo",
	})
	if tags == "" {
		t.Fatal("пустые tags")
	}
	if countSubstring(tags, "russian") > 1 {
		t.Errorf("дублирование russian: %q", tags)
	}
}

func TestFormatQuizDescription_StripsBoilerplate(t *testing.T) {
	subst := "Create a emotional modern pop song with male vocals. The lyrics must be in Russian language. The song is about: Groom Ivan and bride Maria."
	desc := FormatQuizDescription("Свадьба", subst, "фраза «навсегда»")
	if strings.Contains(desc, "Create a") {
		t.Errorf("бойлерплейт не убран: %q", desc)
	}
	if !strings.Contains(desc, "Ivan") {
		t.Errorf("факты потеряны: %q", desc)
	}
	if !strings.Contains(desc, "навсегда") {
		t.Errorf("extra не добавлен: %q", desc)
	}
}

func TestVocalRequirementLine(t *testing.T) {
	if VocalRequirementLine("") != "" {
		t.Error("пустой вокал не должен добавлять строку")
	}
	line := VocalRequirementLine("female vocals")
	if !strings.Contains(line, "female vocals") || !strings.Contains(line, "mandatory") {
		t.Errorf("ожидали явное требование вокала: %q", line)
	}
}

func TestResolveMusicInput_EncodedInspiration(t *testing.T) {
	brief := EncodePrompt("pop ballad", "Russian birthday song for Kolya.")
	in := ResolveMusicInput(brief, "", false)
	if in.Tags != "pop ballad" {
		t.Errorf("tags: %q", in.Tags)
	}
	if in.Description == "" || in.Lyrics != "" {
		t.Errorf("ожидали Inspiration Mode: %+v", in)
	}
}

func TestResolveMusicInput_LyricsCustomMode(t *testing.T) {
	lyrics := "[Verse 1]\nСтрока\n[Chorus]\nПрипев"
	in := ResolveMusicInput(lyrics, "rock", false)
	if in.Lyrics == "" || in.Description != "" {
		t.Errorf("ожидали Custom Mode: %+v", in)
	}
	if in.Tags != "rock" {
		t.Errorf("tags: %q", in.Tags)
	}
}

func countSubstring(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func TestExtractTagsFromBrief(t *testing.T) {
	encoded := EncodePrompt("pop, upbeat", "Песня на день рождения")
	tags := ExtractTagsFromBrief(encoded)
	if !strings.Contains(tags, "pop") {
		t.Errorf("ожидали 'pop' в tags, получили %q", tags)
	}

	// Без тегов → пустая строка.
	onlyDesc := EncodePrompt("", "Только описание")
	if ExtractTagsFromBrief(onlyDesc) != "" {
		t.Error("ожидали пустую строку при отсутствии тегов")
	}

	// Голый текст (не encoded) → пустая строка.
	if ExtractTagsFromBrief("обычный текст") != "" {
		t.Error("ожидали пустую строку для неencoded текста")
	}
}

func TestBriefStoryForLLM(t *testing.T) {
	encoded := EncodePrompt("pop", "История для LLM")
	story := BriefStoryForLLM(encoded)
	if story != "История для LLM" {
		t.Errorf("BriefStoryForLLM: %q", story)
	}

	// Голый текст возвращается как есть.
	raw := "просто текст"
	if BriefStoryForLLM(raw) != raw {
		t.Errorf("ожидали вернуть сырой текст: %q", BriefStoryForLLM(raw))
	}
}
