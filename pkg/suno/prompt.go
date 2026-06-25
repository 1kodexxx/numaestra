package suno

import (
	"regexp"
	"strings"
)

// Маркеры кодированного промпта (хранятся в suno_prompt и в brief конструктора).
const (
	TagsMarker = "#SUNO_TAGS#"
	DescMarker = "#SUNO_DESC#"
)

// Стандартные ключи квиза, значения которых уходят в Suno tags (англ.).
var styleAnswerKeys = []string{"GENRE", "MOOD", "VOCAL", "TEMPO"}

// EncodedPrompt — разделение style tags и описания для Inspiration Mode Suno.
type EncodedPrompt struct {
	Tags        string
	Description string
}

// EncodePrompt сериализует tags и description в одну строку для хранения в БД.
func EncodePrompt(tags, description string) string {
	tags = strings.TrimSpace(tags)
	description = strings.TrimSpace(description)
	if tags == "" && description == "" {
		return ""
	}
	var b strings.Builder
	if tags != "" {
		b.WriteString(TagsMarker)
		b.WriteString(" ")
		b.WriteString(tags)
		b.WriteString("\n")
	}
	b.WriteString(DescMarker)
	b.WriteString("\n")
	b.WriteString(description)
	return b.String()
}

// DecodePrompt разбирает кодированный промпт. ok=false для legacy plain-text.
func DecodePrompt(s string) (EncodedPrompt, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return EncodedPrompt{}, false
	}
	if !strings.Contains(s, TagsMarker) && !strings.Contains(s, DescMarker) {
		return EncodedPrompt{}, false
	}

	out := EncodedPrompt{}
	rest := s
	if i := strings.Index(rest, TagsMarker); i >= 0 {
		rest = rest[i+len(TagsMarker):]
		if j := strings.Index(rest, DescMarker); j >= 0 {
			out.Tags = strings.TrimSpace(rest[:j])
			out.Description = strings.TrimSpace(rest[j+len(DescMarker):])
		} else {
			out.Tags = strings.TrimSpace(rest)
		}
	} else if j := strings.Index(rest, DescMarker); j >= 0 {
		out.Description = strings.TrimSpace(rest[j+len(DescMarker):])
	}
	if out.Description == "" && out.Tags == "" {
		return EncodedPrompt{}, false
	}
	return out, true
}

// ExtractTagsFromBrief возвращает tags из brief конструктора (#SUNO_TAGS#) или пустую строку.
func ExtractTagsFromBrief(brief string) string {
	if enc, ok := DecodePrompt(brief); ok && enc.Tags != "" {
		return enc.Tags
	}
	return ""
}

// BriefStoryForLLM убирает служебные маркеры и оставляет текст для LLM.
func BriefStoryForLLM(brief string) string {
	if enc, ok := DecodePrompt(brief); ok {
		return enc.Description
	}
	return brief
}

// BuildStyleTagsFromAnswers собирает Suno tags из ответов квиза (значения уже на англ.).
func BuildStyleTagsFromAnswers(answers map[string]string) string {
	var parts []string
	seen := make(map[string]struct{})
	for _, key := range styleAnswerKeys {
		v := strings.TrimSpace(answers[key])
		if v == "" {
			continue
		}
		for _, piece := range splitTagPieces(v) {
			low := strings.ToLower(piece)
			if _, ok := seen[low]; ok {
				continue
			}
			seen[low] = struct{}{}
			parts = append(parts, piece)
		}
	}
	if len(parts) == 0 {
		return "russian pop, heartfelt, male vocals"
	}
	if !containsRussianLyricsHint(parts) {
		parts = append(parts, "russian lyrics")
	}
	return strings.Join(parts, ", ")
}

func splitTagPieces(s string) []string {
	s = strings.ReplaceAll(s, ";", ",")
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsRussianLyricsHint(parts []string) bool {
	for _, p := range parts {
		low := strings.ToLower(p)
		if strings.Contains(low, "russian") {
			return true
		}
	}
	return false
}

var (
	reCreatePrefix = regexp.MustCompile(`(?is)^create\s+a\s+.+?(?:\.\s*)`)
	reLyricsLang   = regexp.MustCompile(`(?is)\s*the\s+lyrics\s+must\s+be\s+in\s+russian\s*(?:language)?\.?\s*`)
	reLyricsInRus  = regexp.MustCompile(`(?is)\s*lyrics\s+in\s+russian\.?\s*`)
	reTempoSuffix  = regexp.MustCompile(`(?is)\s*tempo\s+feel:\s*[^.]+\.?\s*`)
	reExtraSuffix  = regexp.MustCompile(`(?is)\s*optional\s+extra\s+details\s+for\s+lyrics:\s*.+$`)
)

// FormatQuizDescription превращает подставленный шаблон категории в понятное Suno-описание.
func FormatQuizDescription(categoryTitle, substituted string, extra string) string {
	body := cleanSubstitutedTemplate(substituted)
	extra = strings.TrimSpace(extra)

	var b strings.Builder
	b.WriteString("Write a complete song. All sung lyrics must be in Russian.\n\n")
	if t := strings.TrimSpace(categoryTitle); t != "" {
		b.WriteString("Occasion / song type: ")
		b.WriteString(t)
		b.WriteString(".\n\n")
	}
	b.WriteString("What the customer wants to hear (use these facts in the lyrics):\n")
	if body != "" {
		b.WriteString(body)
	} else {
		b.WriteString(substituted)
	}
	if extra != "" {
		b.WriteString("\n\nMust-include names, phrases, or details:\n")
		b.WriteString(extra)
	}
	b.WriteString("\n\nMatch the musical style from the separate Suno style tags (genre, mood, vocals, tempo). ")
	b.WriteString("Make the chorus memorable and emotionally clear.")
	return b.String()
}

func cleanSubstitutedTemplate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = reCreatePrefix.ReplaceAllString(s, "")
	s = reLyricsLang.ReplaceAllString(s, "")
	s = reLyricsInRus.ReplaceAllString(s, "")
	s = reTempoSuffix.ReplaceAllString(s, "")
	s = reExtraSuffix.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// ResolveMusicInput разбирает brief/промпт и возвращает поля для Suno API.
func ResolveMusicInput(brief, fallbackStyle string, instrumental bool) MusicInput {
	tags := strings.TrimSpace(fallbackStyle)
	text := brief

	if enc, ok := DecodePrompt(brief); ok {
		if tags == "" {
			tags = enc.Tags
		}
		text = enc.Description
	}

	in := MusicInput{Tags: tags, Instrumental: instrumental}
	if IsStructuredLyricsText(text) {
		in.Lyrics = text
	} else {
		in.Description = text
	}
	return in
}

// IsStructuredLyricsText — готовый текст песни с секциями (Custom Mode), не описание.
func IsStructuredLyricsText(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "[verse") ||
		strings.Contains(lower, "[chorus") ||
		strings.Contains(lower, "[куплет") ||
		strings.Contains(lower, "[припев")
}
