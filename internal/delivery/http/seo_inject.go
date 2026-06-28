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

	"github.com/numaestra/numaestra/internal/domain"
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

// exampleProvider — порт чтения примеров готовых работ для SEO: ListActive нужен
// SeoHandler (sitemap), Get — SEOInjector (превью отдельного примера по /examples/{id}).
type exampleProvider interface {
	ListActive(ctx context.Context) ([]*domain.Example, error)
	Get(ctx context.Context, id string) (*domain.Example, error)
}

type SEOInjector struct {
	template    string
	promptUC    usecase.PromptBuilder
	rater       reviewRater     // nil → AggregateRating не добавляется
	examples    exampleProvider // nil → /examples/{id} получает общий SEO главной
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

// WithExamples включает персональный SEO (title/description/canonical) для страниц
// отдельных примеров /examples/{id}. Без него такие страницы получают общий SEO
// главной — неверный canonical и заголовок, страница не индексируется отдельно.
func (s *SEOInjector) WithExamples(e exampleProvider) *SEOInjector {
	s.examples = e
	return s
}

const seoSiteName = "Numaestra"

// ogImageVersion — query-параметр к og-image.png; при замене картинки увеличьте,
// чтобы Telegram и другие мессенджеры перекачали превью (кэш ~12 ч).
const ogImageVersion = "2"

func ogImageURL(baseURL string) string {
	return baseURL + "/og-image.png?v=" + ogImageVersion
}

var (
	reTitle     = regexp.MustCompile(`(?s)<title>.*?</title>`)
	reDesc      = regexp.MustCompile(`<meta name="description" content="[^"]*"\s*/?>`)
	reRobots    = regexp.MustCompile(`<meta name="robots" content="[^"]*"\s*/?>`)
	reOgTitle   = regexp.MustCompile(`<meta property="og:title" content="[^"]*"\s*/?>`)
	reOgDesc    = regexp.MustCompile(`<meta property="og:description" content="[^"]*"\s*/?>`)
	reOgImage   = regexp.MustCompile(`<meta property="og:image" content="[^"]*"\s*/?>`)
	reTwTitle   = regexp.MustCompile(`<meta name="twitter:title" content="[^"]*"\s*/?>`)
	reTwDesc    = regexp.MustCompile(`<meta name="twitter:description" content="[^"]*"\s*/?>`)
	reTwImage   = regexp.MustCompile(`<meta name="twitter:image" content="[^"]*"\s*/?>`)
	reRootEmpty = regexp.MustCompile(`<div id="root">\s*</div>`)
)

// seoData — вычисленный SEO под маршрут.
type seoData struct {
	title       string
	description string
	canonical   string
	noindex     bool
	ogImage     string   // картинка превью (относительная/абсолютная); пусто → дефолтный og-image.png
	jsonLD      []string // блоки JSON-LD (каждый — готовый JSON без тега <script>)
	body        string   // HTML-блок для вставки в #root
}

// socialPreviewBots — подстроки User-Agent краулеров, которые строят карточки-превью
// ссылок (а НЕ индексируют для поиска). Им страницы шеринга/статуса отдаём без
// noindex: VK, в отличие от Telegram, может не строить карточку для noindex-страниц.
// Поисковики (Googlebot/YandexBot) сюда не входят — они по-прежнему видят noindex,
// поэтому приватные capability-ссылки в поиск не попадают.
var socialPreviewBots = []string{
	"vkshare", "vkontakte", "telegrambot", "whatsapp", "facebookexternalhit",
	"facebot", "twitterbot", "slackbot", "discordbot", "pinterest",
	"skypeuripreview", "viber", "redditbot", "linkedinbot", "odklbot", "ok.ru",
}

// isSocialPreviewBot сообщает, что запрос пришёл от краулера-построителя превью.
func isSocialPreviewBot(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	for _, b := range socialPreviewBots {
		if strings.Contains(ua, b) {
			return true
		}
	}
	return false
}

// Render возвращает HTML index.html, дополненный SEO под путь. При любой ошибке
// (нет шаблонных маркеров, категория не найдена) безопасно отдаёт исходный шаблон.
func (s *SEOInjector) Render(ctx context.Context, path, baseURL string) string {
	return s.RenderForUA(ctx, path, baseURL, "")
}

// RenderForUA — как Render, но учитывает User-Agent: краулерам-превью (VK, Telegram
// и т.п.) noindex-страницы шеринга отдаются без noindex, чтобы они надёжно строили
// карточку. Поисковые боты и обычные пользователи noindex видят как прежде.
func (s *SEOInjector) RenderForUA(ctx context.Context, path, baseURL, userAgent string) string {
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
	// Краулеру-превью noindex не выставляем — иначе VK может не построить карточку.
	if data.noindex && !isSocialPreviewBot(userAgent) {
		out = reRobots.ReplaceAllLiteralString(out, `<meta name="robots" content="noindex, nofollow" />`)
	}

	// Telegram / VK / WhatsApp требуют абсолютный HTTPS URL и растр (PNG/JPEG), не SVG.
	// Страницы категорий и примеров отдают собственную обложку (data.ogImage) —
	// карточка-превью и картинка в выдаче релевантны конкретной странице, а не общая.
	var ogImage string
	if baseURL != "" {
		ogImage = ogImageURL(baseURL)
		if data.ogImage != "" {
			ogImage = absoluteURL(baseURL, data.ogImage)
		}
		out = reOgImage.ReplaceAllLiteralString(out, `<meta property="og:image" content="`+attr(ogImage)+`" />`)
		out = reTwImage.ReplaceAllLiteralString(out, `<meta name="twitter:image" content="`+attr(ogImage)+`" />`)
	}

	// canonical + og:url + JSON-LD — перед </head>.
	var head strings.Builder
	if data.canonical != "" {
		fmt.Fprintf(&head, `<link rel="canonical" href="%s" />`+"\n    ", attr(data.canonical))
		fmt.Fprintf(&head, `<meta property="og:url" content="%s" />`+"\n    ", attr(data.canonical))
	}
	if ogImage != "" {
		fmt.Fprintf(&head, `<meta property="og:image:secure_url" content="%s" />`+"\n    ", attr(ogImage))
		fmt.Fprintf(&head, `<link rel="image_src" href="%s" />`+"\n    ", attr(ogImage))
	}
	for _, block := range data.jsonLD {
		if block != "" {
			fmt.Fprintf(&head, `<script type="application/ld+json">%s</script>`+"\n    ", block)
		}
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
	case strings.HasPrefix(path, "/examples/"):
		id := strings.TrimPrefix(path, "/examples/")
		return s.exampleData(ctx, id, baseURL)
	case strings.HasPrefix(path, "/legal/"):
		slug := strings.TrimPrefix(path, "/legal/")
		return s.legalData(slug, baseURL)
	case path == "/reviews":
		return seoData{
			title:       "Отзывы о Numaestra — что говорят клиенты",
			description: "Реальные отзывы людей, заказавших персональную песню в Numaestra. Оставьте свой отзыв — без регистрации.",
			canonical:   baseURL + "/reviews",
			body:        `<main><h1>Отзывы о Numaestra</h1><p>Что говорят люди, заказавшие персональную песню. Поделитесь и вы — регистрация не нужна.</p></main>`,
		}
	case path == "/how-it-works":
		return seoData{
			title:       "Как это работает — демо бесплатно, песня за 10 минут | " + seoSiteName,
			description: "Как Numaestra создаёт персональную песню: соберите запрос, послушайте бесплатное демо и платите, только если понравилось. 4 версии трека за ~10 минут, без регистрации, один платёж.",
			canonical:   baseURL + "/how-it-works",
			jsonLD:      []string{faqJSONLD(s.faqItems())},
			body:        s.howItWorksBody(),
		}
	case strings.HasPrefix(path, "/s/"), strings.HasPrefix(path, "/status/"):
		// Страницы шеринга песни (/s/{id}) и статуса заказа (/status/{id}).
		// noindex — это не контент для поиска, а capability-ссылка конкретного
		// пользователя; но полный OG-блок (title/description/og:url/og:image)
		// нужен для красивой карточки-превью при шеринге в VK/Telegram/WhatsApp.
		// VK строит сниппет только при наличии og:url, поэтому canonical задаём
		// всегда — он же даёт og:url в Render. Краулеры соцсетей читают OG
		// независимо от robots=noindex, так что превью и приватность не конфликтуют.
		return seoData{
			title:       "Послушайте мою песню от Numaestra 🎵",
			description: "Персональная песня, созданная нейросетью под конкретный повод. Послушайте — и закажите свою за 10 минут в AI-студии Numaestra.",
			canonical:   baseURL + path,
			noindex:     true,
		}
	case path == "/status":
		// Экран поиска заказа без id — не для поиска и не для шеринга.
		return seoData{noindex: true}
	case strings.HasPrefix(path, "/admin"):
		// Служебные экраны админки не индексируем.
		return seoData{noindex: true}
	default:
		return s.homeData(ctx, baseURL)
	}
}

func (s *SEOInjector) homeData(ctx context.Context, baseURL string) seoData {
	d := seoData{
		canonical: baseURL + "/",
		jsonLD:    []string{s.organizationJSONLD(ctx, baseURL), websiteJSONLD(baseURL)},
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
	coverAbs := absoluteURL(baseURL, cat.CoverImageURL())

	var b strings.Builder
	fmt.Fprintf(&b, `<main><h1>%s — песня на заказ нейросетью</h1>`, text(cat.Title()))
	fmt.Fprintf(&b, `<p>%s</p>`, text(desc))
	fmt.Fprintf(&b, `<p>Цена: %s ₽ · 4 уникальные версии · готово за 10 минут.</p>`, s.priceRubles)
	b.WriteString(`</main>`)

	breadcrumb := breadcrumbJSONLD([]crumb{
		{name: "Главная", url: baseURL + "/"},
		{name: cat.Title(), url: canonical},
	})

	return seoData{
		title:       title,
		description: desc,
		canonical:   canonical,
		ogImage:     cat.CoverImageURL(),
		jsonLD:      []string{s.productJSONLD(cat.Title(), desc, canonical, coverAbs), breadcrumb},
		body:        b.String(),
	}
}

// exampleData строит SEO для страницы конкретного примера /examples/{id}. Без
// этого случая такие URL получали бы canonical и title главной страницы — Google
// видел бы десятки страниц с одинаковым canonical и не индексировал ни одну как
// отдельный пример.
func (s *SEOInjector) exampleData(ctx context.Context, id, baseURL string) seoData {
	canonical := baseURL + "/examples/" + id
	if s.examples == nil {
		return seoData{canonical: canonical}
	}
	ex, err := s.examples.Get(ctx, id)
	if err != nil || ex == nil {
		return seoData{canonical: canonical}
	}

	title := ex.Title() + " — пример песни на заказ | " + seoSiteName
	desc := ex.Description()
	if desc == "" {
		desc = "Послушайте пример персональной песни «" + ex.Title() + "», созданной AI-студией Numaestra."
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<main><h1>%s</h1><p>%s</p></main>`, text(ex.Title()), text(desc))

	breadcrumb := breadcrumbJSONLD([]crumb{
		{name: "Главная", url: baseURL + "/"},
		{name: "Примеры", url: baseURL + "/examples"},
		{name: ex.Title(), url: canonical},
	})
	jsonLD := []string{breadcrumb}
	// AudioObject — пример это реальный аудиотрек; разметка помогает Google понять
	// тип контента и может дать аудио-результат в выдаче.
	if ex.AudioURL() != "" {
		jsonLD = append(jsonLD, audioObjectJSONLD(ex.Title(), desc, ex.AudioURL(), absoluteURL(baseURL, ex.CoverURL())))
	}

	return seoData{
		title:       title,
		description: desc,
		canonical:   canonical,
		ogImage:     ex.CoverURL(),
		jsonLD:      jsonLD,
		body:        b.String(),
	}
}

// legalDocs — заголовки и описания юридических страниц /legal/{slug} для
// серверного SEO. Содержимое самих документов рендерится фронтендом
// (frontend/src/pages/legal/legalContent.ts) — здесь только то, что нужно
// поисковику до исполнения JS; при изменении текста там держите эти строки в
// синхроне.
var legalDocs = map[string]struct{ title, description string }{
	"offer":     {"Публичная оферта", "Договор оказания услуг по созданию персональной песни сервисом Numaestra."},
	"refund":    {"Политика возврата", "Условия возврата средств за услугу создания персональной песни."},
	"copyright": {"Права на треки", "Какие права получает заказчик на созданные песни."},
	"privacy":   {"Политика конфиденциальности", "Как Numaestra обрабатывает персональные данные пользователей."},
	"consent":   {"Согласие на обработку персональных данных", "Согласие на обработку персональных данных в соответствии с 152-ФЗ."},
	"contacts":  {"Реквизиты и контакты", "Реквизиты исполнителя и контакты службы поддержки Numaestra."},
}

func (s *SEOInjector) legalData(slug, baseURL string) seoData {
	canonical := baseURL + "/legal/" + slug
	doc, ok := legalDocs[slug]
	if !ok {
		return seoData{canonical: canonical}
	}
	return seoData{
		title:       doc.title + " | " + seoSiteName,
		description: doc.description,
		canonical:   canonical,
		body:        `<main><h1>` + text(doc.title) + `</h1><p>` + text(doc.description) + `</p></main>`,
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

func (s *SEOInjector) productJSONLD(title, desc, url, image string) string {
	p := map[string]any{
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
	}
	// image обязателен для расширенного результата Product в Google. Добавляем только
	// абсолютную ссылку (у категории это обложка); иначе разметка без картинки.
	if image != "" {
		p["image"] = image
	}
	return mustJSON(p)
}

// websiteJSONLD — сущность WebSite для главной: помогает поисковику связать домен
// с брендом и языком (потенциально — sitelinks).
func websiteJSONLD(baseURL string) string {
	return mustJSON(map[string]any{
		"@context":   "https://schema.org",
		"@type":      "WebSite",
		"name":       seoSiteName,
		"url":        baseURL + "/",
		"inLanguage": "ru-RU",
	})
}

// crumb — звено навигационной цепочки для BreadcrumbList.
type crumb struct {
	name string
	url  string
}

// breadcrumbJSONLD строит BreadcrumbList — «хлебные крошки» в выдаче Google
// (Главная › Категория › …), повышающие кликабельность сниппета.
func breadcrumbJSONLD(items []crumb) string {
	list := make([]map[string]any, 0, len(items))
	for i, it := range items {
		list = append(list, map[string]any{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     it.name,
			"item":     it.url,
		})
	}
	return mustJSON(map[string]any{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": list,
	})
}

// audioObjectJSONLD описывает аудио-пример (готовую песню) для поисковика.
func audioObjectJSONLD(name, desc, contentURL, thumbnail string) string {
	a := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "AudioObject",
		"name":        name,
		"description": desc,
		"contentUrl":  contentURL,
		"encodingFormat": "audio/mpeg",
	}
	if thumbnail != "" {
		a["thumbnailUrl"] = thumbnail
	}
	return mustJSON(a)
}

// qa — пара «вопрос-ответ» для FAQPage.
type qa struct {
	q string
	a string
}

// faqJSONLD строит FAQPage — частые вопросы прямо в выдаче Google (большая площадь
// сниппета). Контент тех же Q&A показан и пользователю на /how-it-works (требование
// Google: ответы должны быть видимы на странице).
func faqJSONLD(items []qa) string {
	q := make([]map[string]any, 0, len(items))
	for _, it := range items {
		q = append(q, map[string]any{
			"@type": "Question",
			"name":  it.q,
			"acceptedAnswer": map[string]any{
				"@type": "Answer",
				"text":  it.a,
			},
		})
	}
	return mustJSON(map[string]any{
		"@context":   "https://schema.org",
		"@type":      "FAQPage",
		"mainEntity": q,
	})
}

// faqItems — единый источник Q&A для FAQPage-разметки и серверного блока на
// /how-it-works. Держите в синхроне с видимым FAQ на фронте (HowItWorksPage).
func (s *SEOInjector) faqItems() []qa {
	return []qa{
		{"Сколько стоит песня на заказ?", "Один платёж " + s.priceRubles + " ₽ за 4 уникальные версии трека. Без подписок, доплат и скрытых платежей."},
		{"Можно ли послушать песню до оплаты?", "Да. После заполнения заявки мы бесплатно генерируем демо — самый яркий фрагмент будущей песни. Вы платите, только если понравилось."},
		{"Сколько времени занимает создание песни?", "Около 10 минут после оплаты. Нейросеть создаёт сразу 4 версии — вы выбираете лучшую."},
		{"Нужна ли регистрация?", "Нет. Достаточно описать повод и оставить email — ссылка на готовый трек придёт на почту."},
		{"В каком виде я получу песню?", "4 готовые версии в полном качестве, без водяного знака. Слушайте онлайн, скачивайте и делитесь — ссылки остаются у вас навсегда."},
	}
}

// howItWorksBody — серверный HTML /how-it-works: описание процесса плюс видимый FAQ,
// совпадающий с FAQPage-разметкой (Google требует, чтобы ответы были на странице).
func (s *SEOInjector) howItWorksBody() string {
	var b strings.Builder
	b.WriteString(`<main><h1>Как это работает</h1>`)
	b.WriteString(`<p>Сначала послушайте, потом платите. Соберите запрос (категория или конструктор: жанр, настроение, темп, вокал), опишите повод и оставьте email. Через пару минут на странице заказа появится бесплатное демо — самый яркий фрагмент будущей песни. Понравилось — оплачиваете один раз, и нейросеть выдаёт 4 уникальные версии без водяного знака за ~10 минут. Слушайте, скачивайте и делитесь — ссылки остаются у вас навсегда.</p>`)
	b.WriteString(`<h2>Частые вопросы</h2>`)
	for _, it := range s.faqItems() {
		fmt.Fprintf(&b, `<h3>%s</h3><p>%s</p>`, text(it.q), text(it.a))
	}
	b.WriteString(`</main>`)
	return b.String()
}

// absoluteURL приводит ссылку к абсолютной: внешние (http/https) возвращает как
// есть, относительные дополняет базовым URL хоста запроса. Пустую строку — как есть.
func absoluteURL(baseURL, u string) string {
	u = strings.TrimSpace(u)
	if u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if baseURL == "" {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return baseURL + u
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
