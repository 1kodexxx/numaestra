package apphttp

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/numaestra/numaestra/internal/usecase"
)

// SeoHandler отдаёт robots.txt и sitemap.xml. Абсолютные URL строятся по хосту
// запроса (с учётом X-Forwarded-* за обратным прокси), поэтому работает при любом
// домене без хардкода. Sitemap включает динамические страницы категорий из БД.
type SeoHandler struct {
	promptUC usecase.PromptBuilder
	examples exampleProvider // nil → примеры не включаются в sitemap
	log      *slog.Logger
}

func NewSeoHandler(promptUC usecase.PromptBuilder, log *slog.Logger) *SeoHandler {
	return &SeoHandler{promptUC: promptUC, log: log}
}

// WithExamples включает страницы примеров (/examples/{id}) в sitemap.xml.
func (h *SeoHandler) WithExamples(e exampleProvider) *SeoHandler {
	h.examples = e
	return h
}

// legalSitemapSlugs — публичные юридические страницы /legal/{slug}, индексируемые
// поисковиками. Должны соответствовать слагам в legalDocs (seo_inject.go) и
// frontend/src/pages/legal/legalContent.ts.
var legalSitemapSlugs = []string{"offer", "refund", "copyright", "privacy", "consent", "contacts"}

func (h *SeoHandler) baseURL(r *http.Request) string {
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

// Robots — GET /robots.txt
func (h *SeoHandler) Robots(w http.ResponseWriter, r *http.Request) {
	base := h.baseURL(r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprintf(w, "User-agent: TelegramBot\nAllow: /\n\nUser-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /api/\n\nSitemap: %s/sitemap.xml\n", base)
}

// Sitemap — GET /sitemap.xml
func (h *SeoHandler) Sitemap(w http.ResponseWriter, r *http.Request) {
	base := h.baseURL(r)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	add := func(loc, priority, freq string) {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>")
		b.WriteString(xmlEscape(loc))
		b.WriteString("</loc>\n")
		b.WriteString("    <changefreq>")
		b.WriteString(freq)
		b.WriteString("</changefreq>\n")
		b.WriteString("    <priority>")
		b.WriteString(priority)
		b.WriteString("</priority>\n")
		b.WriteString("  </url>\n")
	}

	add(base+"/", "1.0", "weekly")
	add(base+"/how-it-works", "0.7", "monthly")
	add(base+"/reviews", "0.6", "weekly")
	add(base+"/examples", "0.6", "weekly")

	if cats, err := h.promptUC.GetAllCategories(r.Context()); err != nil {
		h.log.Error("sitemap: не удалось загрузить категории", "err", err)
	} else {
		for _, c := range cats {
			add(base+"/category/"+c.ID(), "0.8", "monthly")
		}
	}

	if h.examples != nil {
		if exs, err := h.examples.ListActive(r.Context()); err != nil {
			h.log.Error("sitemap: не удалось загрузить примеры", "err", err)
		} else {
			for _, e := range exs {
				add(base+"/examples/"+e.ID(), "0.5", "monthly")
			}
		}
	}

	for _, slug := range legalSitemapSlugs {
		add(base+"/legal/"+slug, "0.3", "yearly")
	}

	b.WriteString(`</urlset>` + "\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(b.String()))
}

func xmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;",
	).Replace(s)
}
