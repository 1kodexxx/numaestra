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
	log      *slog.Logger
}

func NewSeoHandler(promptUC usecase.PromptBuilder, log *slog.Logger) *SeoHandler {
	return &SeoHandler{promptUC: promptUC, log: log}
}

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
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /api/\n\nSitemap: %s/sitemap.xml\n", base)
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

	if cats, err := h.promptUC.GetAllCategories(r.Context()); err != nil {
		h.log.Error("sitemap: не удалось загрузить категории", "err", err)
	} else {
		for _, c := range cats {
			add(base+"/category/"+c.ID(), "0.8", "monthly")
		}
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
