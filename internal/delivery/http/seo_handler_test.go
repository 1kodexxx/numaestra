package apphttp

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

func newTestSeoHandler(pb usecase.PromptBuilder) *SeoHandler {
	return NewSeoHandler(pb, discardAdminLogger())
}

func TestSeoHandler_Robots(t *testing.T) {
	h := newTestSeoHandler(&stubPromptBuilder{})
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	req.Host = "numaestra.ru"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	h.Robots(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"User-agent: TelegramBot",
		"Allow: /",
		"Disallow: /admin",
		"Disallow: /api/",
		"Sitemap: https://numaestra.ru/sitemap.xml",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt должен содержать %q", want)
		}
	}
}

func TestSeoHandler_Sitemap_WithCategories(t *testing.T) {
	cats := []*domain.Category{
		domain.RestoreCategory(domain.CategorySnapshot{ID: "wedding", Title: "Свадьба"}),
	}
	h := newTestSeoHandler(&stubPromptBuilder{categories: cats})
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	h.Sitemap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/xml; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	body, _ := io.ReadAll(rec.Body)
	s := string(body)
	for _, want := range []string{
		`<?xml version="1.0"`,
		"<loc>https://example.com/</loc>",
		"<loc>https://example.com/how-it-works</loc>",
		"<loc>https://example.com/reviews</loc>",
		"<loc>https://example.com/category/wedding</loc>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("sitemap должен содержать %q", want)
		}
	}
}

func TestSeoHandler_Sitemap_CategoryLoadError(t *testing.T) {
	h := newTestSeoHandler(&stubPromptBuilder{getErr: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "numaestra.ru"
	rec := httptest.NewRecorder()

	h.Sitemap(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "<loc>http://numaestra.ru/</loc>") {
		t.Errorf("sitemap должен отдавать статические URL даже при ошибке категорий, got: %s", body)
	}
	if strings.Contains(body, "/category/") {
		t.Error("при ошибке категории не должны попадать в sitemap")
	}
}

func TestXmlEscape(t *testing.T) {
	in := `a&b<c>d"e'f`
	got := xmlEscape(in)
	want := `a&amp;b&lt;c&gt;d&quot;e&apos;f`
	if got != want {
		t.Errorf("xmlEscape = %q, want %q", got, want)
	}
}

func TestRequestBaseURL_ForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "internal:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "numaestra.ru")

	if got := requestBaseURL(req); got != "https://numaestra.ru" {
		t.Errorf("requestBaseURL = %q", got)
	}
}

func TestRequestBaseURL_TLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://numaestra.ru/", nil)
	req.TLS = &tls.ConnectionState{}
	req.Host = "numaestra.ru"

	got := requestBaseURL(req)
	if got != "https://numaestra.ru" {
		t.Errorf("requestBaseURL with TLS = %q", got)
	}
}
