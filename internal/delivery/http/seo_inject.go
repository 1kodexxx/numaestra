package apphttp

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/numaestra/numaestra/internal/usecase"
)

// requestBaseURL строит абсолютный базовый URL по хосту запроса с учётом
// X-Forwarded-* за обратным прокси (Caddy). Общая для SEO-инъектора и SeoHandler.
func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = fh
	}
	return scheme + "://" + host
}

// SEOInjector внедряет в index.html серверный SEO под конкретный маршрут: title,
// meta description, canonical, Open Graph, JSON-LD и блок реального текста внутри
// #root. Поисковики (особенно Яндекс, слабее исполняющий JS) видят полный HTML с
// контентом на первом байте — без ожидания загрузки React. React при монтировании
// заменяет содержимое #root, поэтому для пользователей ничего не меняется.
//
// Динамические категории (из БД) работают сразу: данные берутся из usecase в
// момент запроса, без пересборки фронтенда.
type SEOInjector struct {
	template    string
	promptUC    usecase.PromptBuilder
	priceRubles string
	log         *slog.Logger
}

func NewSEOInjector(indexHTML []byte, promptUC usecase.PromptBuilder, priceKopecks int64, log *slog.Logger) *SEOInjector {
	return &SEOInjector{
		template:    string(indexHTML),
		promptUC:    promptUC,
		priceRubles: fmt.Sprintf("%d", priceKopecks/100),
		log:         log,
	}
}

const seoSiteName = "Numaestra"

var (
	reTitle     = regexp.MustCompile(`(?s)<title>.*?</title>`)
	reDesc      = regexp.MustCompile(`<meta name="description" content="[^"]*"\s*/?>`)
	reRobots    = regexp.MustCompile(`<meta name="robots" content="[^"]*"\s*/?>`)
	reOgTitle   = regexp.MustCompile(`<meta property="og:title" content="[^"]*"\s*/?>`)
	reOgDesc    = regexp.MustCompile(`<meta property="og:description" content="[^"]*"\s*/?>`)
	reTwTitle   = regexp.MustCompile(`<meta name="twitter:title" content="[^"]*"\s*/?>`)
	reTwDesc    = regexp.MustCompile(`<meta name="twitter:description" content="[^"]*"\s*/?>`)
	reRootEmpty = regexp.MustCompile(`<div id="root">\s*</div>`)
)

// seoData — вычисленный SEO под маршрут.
type seoData struct {
	title       string
	description string
	canonical   string
	noindex     bool
	jsonLD      string // готовый JSON (без тега)
	body        string // HTML-блок для вставки в #root
}

// Render возвращает HTML index.html, дополненный SEO под путь. При любой ошибке
// (нет шаблонных маркеров, категория не найдена) безопасно отдаёт исходный шаблон.
func (s *SEOInjector) Render(ctx context.Context, path, baseURL string) string {
	data := s.dataFor(ctx, path, baseURL)
	out := s.template

	if data.title != "" {
		out = reTitle.ReplaceAllLiteralString(out, "<title>"+html.EscapeString(data.title)+"</title>")
		out = reOgTitle.ReplaceAllLiteralString(out, `<meta property="og:title" content="`+attr(data.title)+`" />`)
		out = reTwTitle.ReplaceAllLiteralString(out, `<meta name="twitter:title" content="`+attr(data.title)+`" />`)
	}
	if data.description != "" {
		out = reDesc.ReplaceAllLiteralString(out, `<meta name="description" content="`+attr(data.description)+`" />`)
		out = reOgDesc.ReplaceAllLiteralString(out, `<meta property="og:description" content="`+attr(data.description)+`" />`)
		out = reTwDesc.ReplaceAllLiteralString(out, `<meta name="twitter:description" content="`+attr(data.description)+`" />`)
	}
	if data.noindex {
		out = reRobots.ReplaceAllLiteralString(out, `<meta name="robots" content="noindex, nofollow" />`)
	}

	// canonical + og:url + JSON-LD — перед </head>.
	var head strings.Builder
	if data.canonical != "" {
		head.WriteString(`<link rel="canonical" href="` + attr(data.canonical) + `" />` + "\n    ")
		head.WriteString(`<meta property="og:url" content="` + attr(data.canonical) + `" />` + "\n    ")
	}
	if data.jsonLD != "" {
		head.WriteString(`<script type="application/ld+json">` + data.jsonLD + `</script>` + "\n    ")
	}
	if head.Len() > 0 {
		out = strings.Replace(out, "</head>", head.String()+"</head>", 1)
	}

	// Серверный контент внутри #root — React заменит его при монтировании.
	if data.body != "" {
		out = reRootEmpty.ReplaceAllLiteralString(out, `<div id="root">`+data.body+`</div>`)
	}

	return out
}

// dataFor вычисляет SEO под конкретный маршрут.
func (s *SEOInjector) dataFor(ctx context.Context, path, baseURL string) seoData {
	path = "/" + strings.Trim(path, "/")

	switch {
	case path == "/" || path == "/catalog":
		return s.homeData(ctx, baseURL)
	case strings.HasPrefix(path, "/category/"):
		id := strings.TrimPrefix(path, "/category/")
		return s.categoryData(ctx, id, baseURL)
	case path == "/examples":
		return seoData{
			title:       "Примеры песен на заказ — послушать работы | " + seoSiteName,
			description: "Послушайте примеры персональных песен, созданных Numaestra: свадьба, юбилей, признание в любви и другие поводы. Закажите свою песню за 10 минут.",
			canonical:   baseURL + "/examples",
			body:        `<main><h1>Примеры готовых работ</h1><p>Послушайте, какие песни Numaestra создаёт под разные поводы.</p></main>`,
		}
	case strings.HasPrefix(path, "/admin"), strings.HasPrefix(path, "/status"):
		// Служебные экраны не индексируем.
		return seoData{noindex: true}
	default:
		return s.homeData(ctx, baseURL)
	}
}

func (s *SEOInjector) homeData(ctx context.Context, baseURL string) seoData {
	d := seoData{
		canonical: baseURL + "/",
		jsonLD:    s.organizationJSONLD(baseURL),
	}

	var b strings.Builder
	b.WriteString(`<main><h1>Песня, написанная лично для вас</h1>`)
	b.WriteString(`<p>AI-студия Numaestra создаёт уникальную песню под ваш повод. Опишите идею — получите 4 готовые версии трека за 10 минут. Один платёж ` + s.priceRubles + ` ₽, без подписок.</p>`)

	if cats, err := s.promptUC.GetAllCategories(ctx); err == nil && len(cats) > 0 {
		b.WriteString(`<h2>Категории песен на заказ</h2><ul>`)
		for _, c := range cats {
			b.WriteString(`<li><a href="` + attr(baseURL+"/category/"+c.ID()) + `">` + text(c.Title()) + `</a></li>`)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</main>`)
	d.body = b.String()
	return d
}

func (s *SEOInjector) categoryData(ctx context.Context, id, baseURL string) seoData {
	cat, err := s.promptUC.GetCategoryWizard(ctx, id)
	if err != nil || cat == nil {
		// Категория не найдена — отдаём как обычную страницу без спец-SEO.
		return seoData{canonical: baseURL + "/category/" + id}
	}

	title := cat.Title() + " — заказать песню нейросетью за 10 минут | " + seoSiteName
	desc := cat.Description()
	if desc == "" {
		desc = "Закажите персональную песню в категории «" + cat.Title() + "» — 4 версии трека за 10 минут от AI-студии Numaestra."
	}
	canonical := baseURL + "/category/" + id

	var b strings.Builder
	b.WriteString(`<main><h1>` + text(cat.Title()) + ` — песня на заказ нейросетью</h1>`)
	b.WriteString(`<p>` + text(desc) + `</p>`)
	b.WriteString(`<p>Цена: ` + s.priceRubles + ` ₽ · 4 уникальные версии · готово за 10 минут.</p>`)
	b.WriteString(`</main>`)

	return seoData{
		title:       title,
		description: desc,
		canonical:   canonical,
		jsonLD:      s.productJSONLD(cat.Title(), desc, canonical),
		body:        b.String(),
	}
}

func (s *SEOInjector) organizationJSONLD(baseURL string) string {
	return mustJSON(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Organization",
		"name":        seoSiteName,
		"url":         baseURL + "/",
		"logo":        baseURL + "/favicon.svg",
		"description": "AI-студия персональных песен на заказ.",
	})
}

func (s *SEOInjector) productJSONLD(title, desc, url string) string {
	return mustJSON(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Product",
		"name":        "Песня на заказ: " + title,
		"description": desc,
		"brand":       map[string]any{"@type": "Brand", "name": seoSiteName},
		"offers": map[string]any{
			"@type":         "Offer",
			"price":         s.priceRubles,
			"priceCurrency": "RUB",
			"availability":  "https://schema.org/InStock",
			"url":           url,
		},
	})
}

// attr экранирует строку для подстановки в HTML-атрибут (внутри двойных кавычек).
func attr(s string) string { return html.EscapeString(s) }

// text экранирует строку для подстановки в текстовый узел HTML.
func text(s string) string { return html.EscapeString(s) }

// mustJSON сериализует значение; JSON-теги схемы безопасны для вставки в <script>.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	// Защита от преждевременного закрытия тега <script> внутри строковых значений.
	return strings.ReplaceAll(string(b), "</", `<\/`)
}
