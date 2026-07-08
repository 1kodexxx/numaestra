package suno

import (
	"strings"
	"testing"
)

func TestStripUnfilledPlaceholders(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "drops sentence with leftover placeholder",
			in:   "Celebrating 5 years together. How they met: [MEET_STORY].",
			want: "Celebrating 5 years together.",
		},
		{
			name: "keeps fully filled text untouched",
			in:   "Celebrating 5 years together. How they met: at university.",
			want: "Celebrating 5 years together. How they met: at university.",
		},
		{
			name: "removes multiple leftover sentences",
			in:   "About [NAME1] and [NAME2]. Best moment: at sea. Memory: [MEMORY].",
			want: "Best moment: at sea.",
		},
		{
			name: "ignores lowercase brackets in user text",
			in:   "Песня про [кота] и собаку.",
			want: "Песня про [кота] и собаку.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripUnfilledPlaceholders(tc.in); got != tc.want {
				t.Errorf("StripUnfilledPlaceholders(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEncodePromptWithLyrics_RoundTrip(t *testing.T) {
	raw := EncodePromptWithLyrics("rap, male vocals", "Андеграунд про город", "[Verse]\nГде-то под утро висим\n[Chorus]\nСтарается моя Россия")
	enc, ok := DecodePrompt(raw)
	if !ok {
		t.Fatal("ожидали декодирование")
	}
	if enc.Tags != "rap, male vocals" {
		t.Errorf("tags: %q", enc.Tags)
	}
	if enc.Description != "Андеграунд про город" {
		t.Errorf("description: %q", enc.Description)
	}
	if !strings.Contains(enc.Lyrics, "Старается моя Россия") || !strings.Contains(enc.Lyrics, "[Verse]") {
		t.Errorf("lyrics не извлечены целиком: %q", enc.Lyrics)
	}
}

func TestResolveMusicInput_CustomModeOnLyrics(t *testing.T) {
	raw := EncodePromptWithLyrics("rap, russian lyrics", "описание", "[Verse]\nмой текст\n[Chorus]\nприпев")
	in := ResolveMusicInput(raw, "", false)
	if in.Lyrics == "" {
		t.Fatal("ожидали Custom Mode: текст клиента в Lyrics")
	}
	if in.Description != "" {
		t.Errorf("в Custom Mode описание не должно отправляться, получили: %q", in.Description)
	}
	if in.Tags != "rap, russian lyrics" {
		t.Errorf("tags должны сохраниться как стиль: %q", in.Tags)
	}
	if !strings.Contains(in.Lyrics, "мой текст") {
		t.Errorf("в Lyrics должен быть текст клиента дословно: %q", in.Lyrics)
	}
}

func TestResolveMusicInput_InspirationWhenNoLyrics(t *testing.T) {
	// Обратная совместимость: закодированный промпт без #SUNO_LYRICS# → Inspiration Mode.
	raw := EncodePrompt("pop", "Write a song about spring")
	in := ResolveMusicInput(raw, "", false)
	if in.Lyrics != "" {
		t.Errorf("без текста клиента Lyrics должен быть пустым, получили: %q", in.Lyrics)
	}
	if in.Description == "" {
		t.Error("ожидали Inspiration Mode: описание в Description")
	}
}

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
	desc := FormatQuizDescription("Свадьба", subst, "фраза «навсегда»", false)
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

// Реальный кейс первого покупателя: в поле «свой текст» вписана одна фраза
// («с любовью к папе») — Custom Mode спел бы ровно её и оборвал песню на ~30с.
// Короткий текст должен уйти в Inspiration Mode как обязательные дословные строки.
func TestResolveMusicInput_ShortLyricsWovenIntoDescription(t *testing.T) {
	raw := EncodePromptWithLyrics("pop", "Песня для папы на юбилей", "с любовью к папе")
	in := ResolveMusicInput(raw, "", false)
	if in.Lyrics != "" {
		t.Fatalf("короткая фраза не должна уходить в Custom Mode: %+v", in)
	}
	if !strings.Contains(in.Description, "с любовью к папе") {
		t.Errorf("фраза клиента должна попасть в описание дословно: %q", in.Description)
	}
	if !strings.Contains(in.Description, "verbatim") {
		t.Errorf("описание должно требовать дословную вставку строк: %q", in.Description)
	}
	if !strings.Contains(in.Description, "Песня для папы на юбилей") {
		t.Errorf("исходное описание должно сохраниться: %q", in.Description)
	}
}

// Короткая фраза без описания: инструкция с дословными строками не должна потеряться.
func TestResolveMusicInput_ShortLyricsWithoutDescription(t *testing.T) {
	raw := EncodePromptWithLyrics("pop", "", "лучший папа на свете")
	in := ResolveMusicInput(raw, "", false)
	if in.Lyrics != "" || !strings.Contains(in.Description, "лучший папа на свете") {
		t.Errorf("ожидали Inspiration Mode с фразой клиента в описании: %+v", in)
	}
}

// Длинный неструктурированный текст (полная песня без [Verse]-разметки) —
// по-прежнему Custom Mode.
func TestResolveMusicInput_LongPlainLyricsStayCustom(t *testing.T) {
	line := "Эта строчка полноценной песни про родного папу и нашу семью"
	full := strings.Repeat(line+"\n", 10)
	raw := EncodePromptWithLyrics("pop", "", full)
	in := ResolveMusicInput(raw, "", false)
	if in.Lyrics == "" || in.Description != "" {
		t.Errorf("полный текст должен остаться в Custom Mode: %+v", in)
	}
}

func TestIsCompleteSongLyrics(t *testing.T) {
	longLine := "Полноценная строка текста песни о самом дорогом человеке"
	cases := []struct {
		name   string
		lyrics string
		want   bool
	}{
		{"пустая строка", "", false},
		{"одна фраза", "с любовью к папе", false},
		{"короткий припев", "Папа, ты мой герой\nПапа, ты всегда со мной", false},
		{"структура делает полным", "[Verse]\nСтрока\n[Chorus]\nПрипев", true},
		{"куплет-маркер кириллицей", "[Куплет]\nСтрока", true},
		{"длинный текст без структуры", strings.Repeat(longLine+"\n", 10), true},
		{"длинный, но мало строк", strings.Repeat(longLine+" ", 10), false},
	}
	for _, tc := range cases {
		if got := IsCompleteSongLyrics(tc.lyrics); got != tc.want {
			t.Errorf("%s: IsCompleteSongLyrics = %v, want %v", tc.name, got, tc.want)
		}
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

func TestIsInstrumentalFromTags(t *testing.T) {
	cases := []struct {
		tags string
		want bool
	}{
		{"pop, instrumental, slow tempo", true},
		{"instrumental", true},
		{"INSTRUMENTAL", true},
		{"male vocals, modern pop", false},
		{"non-instrumental rock", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsInstrumentalFromTags(tc.tags); got != tc.want {
			t.Errorf("IsInstrumentalFromTags(%q) = %v, want %v", tc.tags, got, tc.want)
		}
	}
}

func TestResolveMusicInput_EncodedKeepsInspirationWithVerseInCustomLyrics(t *testing.T) {
	brief := EncodePrompt("pop", "Write a song...\n\nMust-use lyrics:\n[Verse 1]\nПривет")
	in := ResolveMusicInput(brief, "", false)
	if in.Description == "" || in.Lyrics != "" {
		t.Errorf("encoded промпт с [Verse 1] должен остаться в Inspiration Mode: %+v", in)
	}
}

func TestFormatQuizDescription_Instrumental(t *testing.T) {
	desc := FormatQuizDescription("Свадьба", "Soft melody", "", true)
	if strings.Contains(desc, "lyrics must be in Russian") {
		t.Errorf("instrumental не должен требовать русский вокал: %q", desc)
	}
	if !strings.Contains(desc, "Instrumental track only") {
		t.Errorf("ожидали instrumental-инструкцию: %q", desc)
	}
}

func TestBuildStyleTagsFromAnswers_FiltersCyrillicGenre(t *testing.T) {
	tags, custom := BuildStyleTagsFromAnswersWithCustomGenres(map[string]string{
		"GENRE": "modern pop, Трэп",
		"VOCAL": "male vocals",
	})
	if strings.Contains(tags, "Трэп") {
		t.Errorf("кириллица не должна быть в tags: %q", tags)
	}
	if len(custom) != 1 || custom[0] != "Трэп" {
		t.Errorf("ожидали custom genre Трэп, получили %v", custom)
	}
}

func TestBuildStyleTagsFromAnswers_InstrumentalSkipsRussianLyrics(t *testing.T) {
	tags := BuildStyleTagsFromAnswers(map[string]string{
		"GENRE": "electronic",
		"VOCAL": "instrumental",
	})
	if strings.Contains(strings.ToLower(tags), "russian lyrics") {
		t.Errorf("instrumental не должен добавлять russian lyrics: %q", tags)
	}
}
