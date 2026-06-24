package apphttp

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
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
// reviewRater отдаёт агрегат опубликованных отзывов для AggregateRating в JSON-LD.
type reviewRater interface {
	RatingStats(ctx context.Context) (count int, avg float64, err error)
}

type SEOInjector struct {
	template    string
	promptUC    usecase.PromptBuilder
	rater       reviewRater // nil → AggregateRating не добавляется
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

// WithReviews включает AggregateRating (средняя оценка отзывов) в Organization-разметку на главной.
func (s *SEOInjector) WithReviews(r reviewRater) *SEOInjector {
	s.rater = r
	return s
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
		fmt.Fprintf(&head, `<link rel="canonical" href="%s" />`+"\n    ", attr(data.canonical))
		fmt.Fprintf(&head, `<meta property="og:url" content="%s" />`+"\n    ", attr(data.canonical))
	}
	if data.jsonLD != "" {
		fmt.Fprintf(&head, `<script type="application/ld+json">%s</script>`+"\n    ", data.jsonLD)
	}
	if head.Len() > 0 {
		out = strings.Replace(out, "</head>", head.String()+"</head>", 1)
	}

	// Серверный контент внутри #root: краулеры читают его из HTML-исходника, а
	// визуально он скрыт off-screen (visually-hidden) — поэтому не мелькает у
	// пользователя перед монтированием React, который заменяет содержимое #root.
	if data.body != "" {
		wrapped := `<div id="seo-prerender" style="` + srOnlyStyle + `">` + data.body + `</div>`
		out = reRootEmpty.ReplaceAllLiteralString(out, `<div id="root">`+wrapped+`</div>`)
	}

	return out
}

// srOnlyStyle прячет элемент визуально, оставляя его в DOM и доступным для
// поисковых краулеров и скринридеров (стандартный приём «visually-hidden»).
const srOnlyStyle = "position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0"

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
	case path == "/reviews":
		return seoData{
			title:       "Отзывы о Numaestra — что говорят клиенты",
			description: "Реальные отзывы людей, заказавших персональную песню в Numaestra. Оставьте свой отзыв — без регистрации.",
			canonical:   baseURL + "/reviews",
			body:        `<main><h1>Отзывы о Numaestra</h1><p>Что говорят люди, заказавшие персональную песню. Поделитесь и вы — регистрация не нужна.</p></main>`,
		}
	case path == "/how-it-works":
		return seoData{
			title:       "Как это работает — заказать песню за 6 шагов | " + seoSiteName,
			description: "Пошаговый процесс заказа персональной песни в Numaestra: от выбора повода до 4 готовых версий трека за 10 минут. Без регистрации, один платёж 2000 ₽.",
			canonical:   baseURL + "/how-it-works",
			body:        `<main><h1>Как это работает</h1><p>От идеи до готовой песни за 6 шагов: выбор повода, бриф, контакты, оплата, генерация нейросетью и 4 готовые версии за 10 минут.</p></main>`,
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
		jsonLD:    s.organizationJSONLD(ctx, baseURL),
	}

	var b strings.Builder
	b.WriteString(`<main><h1>Песня, написанная лично для вас</h1>`)
	fmt.Fprintf(&b, `<p>AI-студия Numaestra создаёт уникальную песню под ваш повод. Опишите идею — получите 4 готовые версии трека за 10 минут. Один платёж %s ₽, без подписок.</p>`, s.priceRubles)

	if cats, err := s.promptUC.GetAllCategories(ctx); err == nil && len(cats) > 0 {
		b.WriteString(`<h2>Категории песен на заказ</h2><ul>`)
		for _, c := range cats {
			fmt.Fprintf(&b, `<li><a href="%s">%s</a></li>`, attr(baseURL+"/category/"+c.ID()), text(c.Title()))
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
	fmt.Fprintf(&b, `<main><h1>%s — песня на заказ нейросетью</h1>`, text(cat.Title()))
	fmt.Fprintf(&b, `<p>%s</p>`, text(desc))
	fmt.Fprintf(&b, `<p>Цена: %s ₽ · 4 уникальные версии · готово за 10 минут.</p>`, s.priceRubles)
	b.WriteString(`</main>`)

	return seoData{
		title:       title,
		description: desc,
		canonical:   canonical,
		jsonLD:      s.productJSONLD(cat.Title(), desc, canonical),
		body:        b.String(),
	}
}

func (s *SEOInjector) organizationJSONLD(ctx context.Context, baseURL string) string {
	org := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Organization",
		"name":        seoSiteName,
		"url":         baseURL + "/",
		"logo":        baseURL + "/favicon.svg",
		"description": "AI-студия персональных песен на заказ.",
	}
	// AggregateRating — звёзды в выдаче. Добавляем только при наличии отзывов
	// (reviewCount ≥ 1), иначе разметка невалидна.
	if s.rater != nil {
		if count, avg, err := s.rater.RatingStats(ctx); err == nil && count > 0 {
			org["aggregateRating"] = map[string]any{
				"@type":       "AggregateRating",
				"ratingValue": fmt.Sprintf("%.1f", avg),
				"reviewCount": strconv.Itoa(count),
				"bestRating":  "5",
				"worstRating": "1",
			}
		}
	}
	return mustJSON(org)
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
