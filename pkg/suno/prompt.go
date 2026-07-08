package suno

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Маркеры кодированного промпта (хранятся в suno_prompt и в brief конструктора).
// Порядок секций фиксирован: TAGS → DESC → LYRICS.
const (
	TagsMarker   = "#SUNO_TAGS#"
	DescMarker   = "#SUNO_DESC#"
	LyricsMarker = "#SUNO_LYRICS#"
)

// Стандартные ключи квиза, значения которых уходят в Suno tags (англ.).
// VOCAL первым — Suno сильнее учитывает начало tags.
var styleAnswerKeys = []string{"VOCAL", "GENRE", "MOOD", "TEMPO"}

// EncodedPrompt — разбор закодированного промпта на каналы Suno:
//   - Tags — стиль/жанр (поле tags в обоих режимах);
//   - Description — описание идеи для Inspiration Mode (Suno сам пишет текст);
//   - Lyrics — готовый текст клиента для Custom Mode (Suno поёт его дословно).
//
// Lyrics и Description взаимоисключающи по смыслу: если клиент прислал свои слова,
// генерация уходит в Custom Mode и описание не используется.
type EncodedPrompt struct {
	Tags        string
	Description string
	Lyrics      string
}

// EncodePrompt сериализует tags и description (Inspiration Mode) для хранения в БД.
func EncodePrompt(tags, description string) string {
	return EncodePromptWithLyrics(tags, description, "")
}

// EncodePromptWithLyrics сериализует tags, описание и (опционально) готовый текст
// клиента. Непустой lyrics добавляет секцию #SUNO_LYRICS# — сигнал Custom Mode.
func EncodePromptWithLyrics(tags, description, lyrics string) string {
	tags = strings.TrimSpace(tags)
	description = strings.TrimSpace(description)
	lyrics = strings.TrimSpace(lyrics)
	if tags == "" && description == "" && lyrics == "" {
		return ""
	}
	var b strings.Builder
	if tags != "" {
		b.WriteString(TagsMarker)
		b.WriteString(" ")
		b.WriteString(tags)
		b.WriteString("\n")
	}
	// DescMarker пишем всегда, когда нет текста, — сохраняем формат старых промптов
	// (даже с пустым описанием), чтобы DecodePrompt их распознавал как закодированные.
	if lyrics == "" {
		b.WriteString(DescMarker)
		b.WriteString("\n")
		b.WriteString(description)
		return b.String()
	}
	if description != "" {
		b.WriteString(DescMarker)
		b.WriteString("\n")
		b.WriteString(description)
		b.WriteString("\n")
	}
	b.WriteString(LyricsMarker)
	b.WriteString("\n")
	b.WriteString(lyrics)
	return b.String()
}

// DecodePrompt разбирает кодированный промпт. ok=false для legacy plain-text.
func DecodePrompt(s string) (EncodedPrompt, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return EncodedPrompt{}, false
	}
	if !strings.Contains(s, TagsMarker) && !strings.Contains(s, DescMarker) && !strings.Contains(s, LyricsMarker) {
		return EncodedPrompt{}, false
	}

	out := EncodedPrompt{
		Tags:        sectionBetween(s, TagsMarker, DescMarker, LyricsMarker),
		Description: sectionBetween(s, DescMarker, LyricsMarker),
		Lyrics:      sectionBetween(s, LyricsMarker),
	}
	if out.Tags == "" && out.Description == "" && out.Lyrics == "" {
		return EncodedPrompt{}, false
	}
	return out, true
}

// sectionBetween возвращает содержимое после маркера start до ближайшего из ends
// (или до конца строки). Пустую строку — если start отсутствует.
func sectionBetween(s, start string, ends ...string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	cut := len(rest)
	for _, e := range ends {
		if j := strings.Index(rest, e); j >= 0 && j < cut {
			cut = j
		}
	}
	return strings.TrimSpace(rest[:cut])
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

// TemplateSkipKeys — служебные ключи квиза: не подставляются в story-шаблон.
var TemplateSkipKeys = map[string]bool{
	"VOCAL": true, "GENRE": true, "MOOD": true, "TEMPO": true,
	"EXTRA": true, "CUSTOM_LYRICS": true,
}

// rePlaceholder — незаполненный плейсхолдер шаблона вида [MEET_STORY]: только
// верхний регистр, чтобы не задеть случайные [скобки] в тексте клиента.
var rePlaceholder = regexp.MustCompile(`\[[A-Z][A-Z0-9_]*\]`)

// StripUnfilledPlaceholders убирает из подставленного шаблона предложения, в
// которых остался неподставленный плейсхолдер [KEY]. Это происходит, когда вопрос
// квиза необязателен и клиент его не заполнил: без очистки литерал «[MEET_STORY]»
// ушёл бы в Suno и мог попасть прямо в текст песни. Предложение удаляется целиком
// (а не только плейсхолдер), чтобы не оставлять обрубков вроде «Как познакомились: .».
func StripUnfilledPlaceholders(s string) string {
	if !rePlaceholder.MatchString(s) {
		return s
	}
	parts := strings.Split(s, ". ")
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if rePlaceholder.MatchString(p) {
			continue
		}
		if strings.TrimSpace(p) == "" {
			continue
		}
		kept = append(kept, p)
	}
	res := strings.TrimSpace(strings.Join(kept, ". "))
	if res != "" && !strings.HasSuffix(res, ".") && !strings.HasSuffix(res, "!") && !strings.HasSuffix(res, "?") {
		res += "."
	}
	return res
}

// IsInstrumentalFromTags — true, если в tags есть instrumental как отдельный тег.
func IsInstrumentalFromTags(tags string) bool {
	for _, piece := range splitTagPieces(tags) {
		if strings.EqualFold(strings.TrimSpace(piece), "instrumental") {
			return true
		}
	}
	return false
}

// IsInstrumentalVocal — true для ответа VOCAL с instrumental (категорийный квиз).
func IsInstrumentalVocal(vocal string) bool {
	v := strings.ToLower(strings.TrimSpace(vocal))
	return v == "instrumental" || strings.Contains(v, "instrumental")
}

func containsCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

// BuildStyleTagsFromAnswers собирает Suno tags из ответов квиза (значения уже на англ.).
// Кириллические значения GENRE (свой жанр) не попадают в tags — их нужно добавить в description.
func BuildStyleTagsFromAnswers(answers map[string]string) string {
	tags, _ := BuildStyleTagsFromAnswersWithCustomGenres(answers)
	return tags
}

// BuildStyleTagsFromAnswersWithCustomGenres возвращает tags и жанры, перенесённые из GENRE в description.
func BuildStyleTagsFromAnswersWithCustomGenres(answers map[string]string) (string, []string) {
	var parts []string
	var customGenres []string
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
			if key == "GENRE" && containsCyrillic(piece) {
				customGenres = append(customGenres, piece)
				continue
			}
			seen[low] = struct{}{}
			parts = append(parts, piece)
		}
	}
	if len(parts) == 0 && len(customGenres) == 0 {
		return "russian pop, heartfelt", customGenres
	}
	if !IsInstrumentalFromTags(strings.Join(parts, ", ")) && !containsRussianLyricsHint(parts) {
		parts = append(parts, "russian lyrics")
	}
	return strings.Join(parts, ", "), customGenres
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
func FormatQuizDescription(categoryTitle, substituted string, extra string, instrumental bool) string {
	body := cleanSubstitutedTemplate(substituted)
	extra = strings.TrimSpace(extra)

	var b strings.Builder
	if instrumental {
		b.WriteString("Write a complete instrumental track. Instrumental track only. No vocals.\n\n")
	} else {
		b.WriteString("Write a complete song. All sung lyrics must be in Russian.\n\n")
	}
	if t := strings.TrimSpace(categoryTitle); t != "" {
		b.WriteString("Occasion / song type: ")
		b.WriteString(t)
		b.WriteString(".\n\n")
	}
	if instrumental {
		b.WriteString("What the customer wants to hear (use these facts for mood and theme):\n")
	} else {
		b.WriteString("What the customer wants to hear (use these facts in the lyrics):\n")
	}
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
	if instrumental {
		b.WriteString("Focus on memorable melody and emotional arc without any singing.")
	} else {
		b.WriteString("Make the chorus memorable and emotionally clear.")
	}
	return b.String()
}

// VocalRequirementLine — явная инструкция для Suno Inspiration Mode.
// Теги с типом вокала часто недостаточны; описание должно дублировать требование.
func VocalRequirementLine(vocal string) string {
	vocal = strings.TrimSpace(vocal)
	if vocal == "" {
		return ""
	}
	return "\n\nVocal requirement (mandatory, do not change): " + vocal + "."
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
//
// Если клиент прислал готовый текст (#SUNO_LYRICS#) — уходим в Custom Mode: Suno
// поёт ровно эти слова (поле prompt), стиль берётся из tags. Иначе Inspiration
// Mode: описание идёт в gpt_description_prompt, и Suno сам пишет текст.
func ResolveMusicInput(brief, fallbackStyle string, instrumental bool) MusicInput {
	tags := strings.TrimSpace(fallbackStyle)
	text := brief
	lyrics := ""
	encoded := false

	if enc, ok := DecodePrompt(brief); ok {
		encoded = true
		if tags == "" {
			tags = enc.Tags
		}
		text = enc.Description
		lyrics = enc.Lyrics
	}

	if !instrumental && IsInstrumentalFromTags(tags) {
		instrumental = true
	}

	in := MusicInput{Tags: tags, Instrumental: instrumental}
	switch {
	case lyrics != "" && !IsCompleteSongLyrics(lyrics):
		// Клиент прислал фразу/припев, а не полный текст. Custom Mode спел бы
		// ровно эти слова и оборвал песню (реальный кейс: «с любовью к папе» →
		// 30-секундный клип). Уходим в Inspiration Mode: Suno пишет полный текст,
		// обязуясь дословно вставить строки клиента.
		in.Description = weaveVerbatimLines(text, lyrics)
	case lyrics != "":
		// Custom Mode: точные слова клиента (полный текст песни).
		in.Lyrics = lyrics
	case encoded || !IsStructuredLyricsText(text):
		// Inspiration Mode: описание идеи, текст пишет Suno.
		in.Description = text
	default:
		// Legacy: свободный текст со структурой [Verse]/[Chorus] без маркеров.
		in.Lyrics = text
	}
	return in
}

// Пороги «полного текста песни»: короче — считаем фразой/припевом, который надо
// вплести в песню, а не петь как весь текст. Полный текст (2 куплета + припев)
// обычно ≥ 600 символов; 400/8 строк — консервативная нижняя граница. Разметка
// [Verse]/[Куплет] — сигнал осознанности, порог ниже, но объём обязателен:
// «[Припев] + одна строка» — всё ещё обрубок, а не песня.
const (
	minCompleteLyricsRunes = 400
	minCompleteLyricsLines = 8

	minStructuredLyricsRunes = 200
	minStructuredLyricsLines = 4
)

// IsCompleteSongLyrics — присланный текст тянет на ПОЛНУЮ песню для Custom Mode:
// достаточные объём и число строк; явная структура ([Verse]/[Куплет]...) снижает
// порог, но не отменяет его.
func IsCompleteSongLyrics(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lines := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			lines++
		}
	}
	runes := utf8.RuneCountInString(s)
	if IsStructuredLyricsText(s) {
		return runes >= minStructuredLyricsRunes && lines >= minStructuredLyricsLines
	}
	return runes >= minCompleteLyricsRunes && lines >= minCompleteLyricsLines
}

// weaveVerbatimLines дополняет описание требованием дословно вставить строки
// клиента в полный текст песни (Inspiration Mode).
func weaveVerbatimLines(description, lyrics string) string {
	instr := "The song MUST include the following customer lines verbatim, exactly as written (do not translate or alter them), woven naturally into complete full-length lyrics:\n" + strings.TrimSpace(lyrics)
	description = strings.TrimSpace(description)
	if description == "" {
		return instr
	}
	return description + "\n\n" + instr
}

// IsStructuredLyricsText — готовый текст песни с секциями (Custom Mode), не описание.
func IsStructuredLyricsText(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "[verse") ||
		strings.Contains(lower, "[chorus") ||
		strings.Contains(lower, "[куплет") ||
		strings.Contains(lower, "[припев")
}
