package apphttp

import (
	"context"
	"strings"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

const seoTestTemplate = `<!DOCTYPE html><html lang="ru"><head>
<title>Numaestra — персональная песня на заказ за 10 минут</title>
<meta name="description" content="дефолтное описание" />
<meta name="robots" content="index, follow, max-image-preview:large" />
<meta property="og:title" content="OG def" />
<meta property="og:description" content="OG desc def" />
<meta property="og:image" content="/og-image.png" />
<meta name="twitter:title" content="TW def" />
<meta name="twitter:description" content="TW desc def" />
<meta name="twitter:image" content="/og-image.png" />
</head><body><div id="root"></div></body></html>`

func newSEOInjector(pb *stubPromptBuilder) *SEOInjector {
	return NewSEOInjector([]byte(seoTestTemplate), pb, 200000, discardAdminLogger())
}

func TestSEOInjector_Home(t *testing.T) {
	cats := []*domain.Category{
		domain.RestoreCategory(domain.CategorySnapshot{ID: "wedding", Title: "Свадьба"}),
		domain.RestoreCategory(domain.CategorySnapshot{ID: "bday", Title: "День рождения"}),
	}
	inj := newSEOInjector(&stubPromptBuilder{categories: cats})
	html := inj.Render(context.Background(), "/", "https://numaestra.ru")

	wants := []string{
		`<h1>Песня, написанная лично для вас</h1>`,
		`"@type":"Organization"`,
		`<link rel="canonical" href="https://numaestra.ru/" />`,
		`href="https://numaestra.ru/category/wedding"`,
		`>Свадьба<`,
		`2000 ₽`,
	}
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("главная: ожидали подстроку %q в HTML", w)
		}
	}
}

type stubRater struct {
	count int
	avg   float64
}

func (s stubRater) RatingStats(context.Context) (int, float64, error) { return s.count, s.avg, nil }

func TestSEOInjector_HomeAggregateRating(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{}).WithReviews(stubRater{count: 12, avg: 4.83})
	html := inj.Render(context.Background(), "/", "https://numaestra.ru")

	for _, w := range []string{`"@type":"AggregateRating"`, `"ratingValue":"4.8"`, `"reviewCount":"12"`, `"bestRating":"5"`} {
		if !strings.Contains(html, w) {
			t.Errorf("ожидали %q в JSON-LD главной", w)
		}
	}
}

func TestSEOInjector_HomeNoRatingWhenEmpty(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{}).WithReviews(stubRater{count: 0})
	html := inj.Render(context.Background(), "/", "https://numaestra.ru")

	if strings.Contains(html, "AggregateRating") {
		t.Error("без отзывов AggregateRating не должен добавляться")
	}
}

func TestSEOInjector_Category(t *testing.T) {
	cat := domain.RestoreCategory(domain.CategorySnapshot{
		ID: "wedding", Title: "Свадьба", Description: "Песня на вашу свадьбу",
	})
	inj := newSEOInjector(&stubPromptBuilder{wizard: cat})
	html := inj.Render(context.Background(), "/category/wedding", "https://numaestra.ru")

	wants := []string{
		`<title>Свадьба — заказать песню нейросетью за 10 минут | Numaestra</title>`,
		`<meta name="description" content="Песня на вашу свадьбу" />`,
		`<meta property="og:title" content="Свадьба — заказать песню нейросетью за 10 минут | Numaestra" />`,
		`"@type":"Product"`,
		`"price":"2000"`,
		`"priceCurrency":"RUB"`,
		`schema.org/InStock`,
		`<link rel="canonical" href="https://numaestra.ru/category/wedding" />`,
		`<h1>Свадьба — песня на заказ нейросетью</h1>`,
	}
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("категория: ожидали подстроку %q в HTML", w)
		}
	}
}

func TestSEOInjector_CategoryNotFound_Fallback(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{wizardErr: context.DeadlineExceeded})
	html := inj.Render(context.Background(), "/category/missing", "https://numaestra.ru")

	// Без спец-SEO, но валидный HTML с canonical и без Product-разметки.
	if strings.Contains(html, `"@type":"Product"`) {
		t.Error("для несуществующей категории не должно быть Product-разметки")
	}
	if !strings.Contains(html, `<div id="root">`) {
		t.Error("HTML должен остаться валидным с #root")
	}
}

func TestSEOInjector_AdminNoindex(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/admin/login", "https://numaestra.ru")

	if !strings.Contains(html, `<meta name="robots" content="noindex, nofollow" />`) {
		t.Error("служебные страницы должны быть noindex")
	}
}

func TestSEOInjector_Examples(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/examples", "https://numaestra.ru")

	if !strings.Contains(html, "Примеры песен на заказ") {
		t.Error("страница примеров должна иметь свой title")
	}
	if !strings.Contains(html, "<h1>Примеры готовых работ</h1>") {
		t.Error("страница примеров должна иметь серверный h1")
	}
}

func TestSEOInjector_EscapesAndJSONLDSafe(t *testing.T) {
	cat := domain.RestoreCategory(domain.CategorySnapshot{
		ID: "x", Title: `Тест <script>`, Description: `Опис </script> "кавычки"`,
	})
	inj := newSEOInjector(&stubPromptBuilder{wizard: cat})
	html := inj.Render(context.Background(), "/category/x", "https://numaestra.ru")

	// Тег <script> из данных не должен попасть в HTML «как есть».
	if strings.Contains(html, "<script>Тест") || strings.Contains(html, `</script> "кавычки"`) {
		t.Error("пользовательские данные должны экранироваться")
	}
}

func TestSEOInjector_HomeOgImageMeta(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/", "https://numaestra.ru")

	for _, want := range []string{
		`property="og:image" content="https://numaestra.ru/og-image.png?v=2"`,
		`property="og:image:secure_url" content="https://numaestra.ru/og-image.png?v=2"`,
		`rel="image_src" href="https://numaestra.ru/og-image.png?v=2"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("главная должна содержать %q", want)
		}
	}
}

// --- /examples/{id} и /legal/{slug} ---

type stubExampleProvider struct {
	active  []*domain.Example
	byID    *domain.Example
	getErr  error
	listErr error
}

func (s stubExampleProvider) ListActive(context.Context) ([]*domain.Example, error) {
	return s.active, s.listErr
}

func (s stubExampleProvider) Get(_ context.Context, _ string) (*domain.Example, error) {
	return s.byID, s.getErr
}

func TestSEOInjector_ExampleDetail(t *testing.T) {
	ex, err := domain.NewExample("wedding-1", "Свадьба Лены и Антона", "wedding", "Тёплая свадебная песня", "joy", "https://s3/audio.mp3", "https://s3/cover.webp", 1, true)
	if err != nil {
		t.Fatalf("NewExample: %v", err)
	}
	inj := newSEOInjector(&stubPromptBuilder{}).WithExamples(stubExampleProvider{byID: ex})
	html := inj.Render(context.Background(), "/examples/wedding-1", "https://numaestra.ru")

	wants := []string{
		`<title>Свадьба Лены и Антона — пример песни на заказ | Numaestra</title>`,
		`<meta name="description" content="Тёплая свадебная песня" />`,
		`<link rel="canonical" href="https://numaestra.ru/examples/wedding-1" />`,
		`<h1>Свадьба Лены и Антона</h1>`,
	}
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("пример: ожидали подстроку %q в HTML", w)
		}
	}
}

func TestSEOInjector_ExampleDetail_NotFound_Fallback(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{}).WithExamples(stubExampleProvider{getErr: domain.ErrExampleNotFound})
	html := inj.Render(context.Background(), "/examples/missing", "https://numaestra.ru")

	if !strings.Contains(html, `<link rel="canonical" href="https://numaestra.ru/examples/missing" />`) {
		t.Error("несуществующий пример должен хотя бы получить корректный canonical")
	}
}

func TestSEOInjector_ExampleDetail_WithoutProvider_FallsBackToHome(t *testing.T) {
	// Без WithExamples() страница примера не должна выдавать canonical главной —
	// иначе все /examples/{id} канонизируются на "/", и ни один не индексируется.
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/examples/wedding-1", "https://numaestra.ru")

	if !strings.Contains(html, `<link rel="canonical" href="https://numaestra.ru/examples/wedding-1" />`) {
		t.Error("без провайдера примеров canonical всё равно должен указывать на саму страницу")
	}
}

func TestSEOInjector_LegalPage(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/legal/privacy", "https://numaestra.ru")

	wants := []string{
		`<title>Политика конфиденциальности | Numaestra</title>`,
		`<meta name="description" content="Как Numaestra обрабатывает персональные данные пользователей." />`,
		`<link rel="canonical" href="https://numaestra.ru/legal/privacy" />`,
		`<h1>Политика конфиденциальности</h1>`,
	}
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("legal: ожидали подстроку %q в HTML", w)
		}
	}
	// Юридические страницы должны индексироваться (это не служебный экран).
	if strings.Contains(html, `<meta name="robots" content="noindex, nofollow" />`) {
		t.Error("страница политики конфиденциальности не должна быть noindex")
	}
}

func TestSEOInjector_LegalPage_UnknownSlug_Fallback(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/legal/unknown-doc", "https://numaestra.ru")

	if !strings.Contains(html, `<link rel="canonical" href="https://numaestra.ru/legal/unknown-doc" />`) {
		t.Error("неизвестный slug должен хотя бы получить корректный canonical")
	}
}

func TestSEOInjector_ShareSong(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/s/8e250afe-894e-4d70-8a64-54bdc474117a", "https://numaestra.ru")

	if !strings.Contains(html, "Послушайте мою песню от Numaestra") {
		t.Error("страница шеринга должна иметь своё og:title")
	}
	if !strings.Contains(html, `<meta name="robots" content="noindex, nofollow" />`) {
		t.Error("страница шеринга должна быть noindex")
	}
	if !strings.Contains(html, `href="https://numaestra.ru/s/8e250afe-894e-4d70-8a64-54bdc474117a"`) {
		t.Error("canonical должен указывать на конкретную ссылку шеринга")
	}
	if !strings.Contains(html, `property="og:url" content="https://numaestra.ru/s/8e250afe-894e-4d70-8a64-54bdc474117a"`) {
		t.Error("og:url обязателен для построения сниппета в VK")
	}
	wantImg := `property="og:image" content="https://numaestra.ru/og-image.png?v=2"`
	if !strings.Contains(html, wantImg) {
		t.Errorf("og:image должен быть абсолютным PNG для превью в мессенджерах, ожидали %q", wantImg)
	}
	if !strings.Contains(html, `property="og:image:secure_url" content="https://numaestra.ru/og-image.png?v=2"`) {
		t.Error("og:image:secure_url обязателен для HTTPS-превью в Telegram")
	}
}

// Страница статуса заказа (/status/{id}) — частая ссылка для шеринга. Должна
// получать полную OG-карточку (og:title/description/og:url/og:image), оставаясь
// noindex для Google.
func TestSEOInjector_StatusOrderShareCard(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/status/1044cd49-6f73-4b43-b88c-7f3ba64c44d2", "https://numaestra.ru")

	if !strings.Contains(html, `property="og:title" content="Послушайте мою песню от Numaestra`) {
		t.Error("страница статуса должна иметь песенный og:title для красивого превью")
	}
	if !strings.Contains(html, `property="og:url" content="https://numaestra.ru/status/1044cd49-6f73-4b43-b88c-7f3ba64c44d2"`) {
		t.Error("og:url обязателен — без него VK не строит карточку")
	}
	if !strings.Contains(html, `property="og:image" content="https://numaestra.ru/og-image.png?v=2"`) {
		t.Error("og:image должен присутствовать для превью")
	}
	if !strings.Contains(html, `<meta name="robots" content="noindex, nofollow" />`) {
		t.Error("приватная страница статуса должна оставаться noindex для Google")
	}
}

// Экран поиска заказа без id (/status) не шерится — только noindex, без OG-карточки.
func TestSEOInjector_StatusLookupNoindex(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	html := inj.Render(context.Background(), "/status", "https://numaestra.ru")

	if !strings.Contains(html, `<meta name="robots" content="noindex, nofollow" />`) {
		t.Error("экран поиска заказа должен быть noindex")
	}
	if strings.Contains(html, "Послушайте мою песню") {
		t.Error("экран поиска без id не должен получать песенную OG-карточку")
	}
}

// Краулеру-превью (VK/Telegram) страница шеринга отдаётся БЕЗ noindex — иначе VK
// не строит карточку. При этом OG-данные и og:url остаются на месте.
func TestSEOInjector_PreviewBotSkipsNoindex(t *testing.T) {
	inj := newSEOInjector(&stubPromptBuilder{})
	path := "/s/1044cd49-6f73-4b43-b88c-7f3ba64c44d2"

	vkHTML := inj.RenderForUA(context.Background(), path, "https://numaestra.ru", "Mozilla/5.0 (compatible; vkShare; +http://vk.com/dev/Share)")
	if strings.Contains(vkHTML, `content="noindex, nofollow"`) {
		t.Error("краулеру VK страница шеринга не должна отдаваться с noindex")
	}
	if !strings.Contains(vkHTML, `property="og:url" content="https://numaestra.ru/s/1044cd49-6f73-4b43-b88c-7f3ba64c44d2"`) {
		t.Error("og:url должен остаться для построения карточки")
	}
	if !strings.Contains(vkHTML, `property="og:image"`) {
		t.Error("og:image должен остаться для краулера превью")
	}

	// Googlebot и обычные пользователи по-прежнему видят noindex (приватность от поиска).
	googleHTML := inj.RenderForUA(context.Background(), path, "https://numaestra.ru", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	if !strings.Contains(googleHTML, `content="noindex, nofollow"`) {
		t.Error("Googlebot должен видеть noindex на приватной странице шеринга")
	}
}
