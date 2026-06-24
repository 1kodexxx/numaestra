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
<meta name="twitter:title" content="TW def" />
<meta name="twitter:description" content="TW desc def" />
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
