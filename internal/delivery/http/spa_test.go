package apphttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAHandler_ServesAssetAndFallback(t *testing.T) {
	static := fstest.MapFS{
		"index.html":              {Data: []byte("<html><head></head><body>app</body></html>")},
		"assets/app.js":           {Data: []byte("console.log('ok')")},
		"assets/index-abc123.css": {Data: []byte("body{}")},
	}
	inj := newSEOInjector(&stubPromptBuilder{})
	h := NewSPAHandler(static, inj)

	t.Run("unknown route gets SEO index", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/reviews", nil)
		req.Host = "numaestra.ru"
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache, must-revalidate" {
			t.Errorf("Cache-Control = %q", cc)
		}
		body, _ := io.ReadAll(rec.Body)
		if !strings.Contains(string(body), "Отзывы о Numaestra") {
			t.Error("ожидали SEO-инъекцию для /reviews")
		}
	})

	t.Run("asset gets long cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Errorf("Cache-Control = %q", cc)
		}
	})

	t.Run("without SEO serves raw index", func(t *testing.T) {
		raw := NewSPAHandler(static, nil)
		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		rec := httptest.NewRecorder()
		raw.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		if !strings.Contains(string(body), "<body>app</body>") {
			t.Error("без SEO должен отдаваться исходный index.html")
		}
	})
}
