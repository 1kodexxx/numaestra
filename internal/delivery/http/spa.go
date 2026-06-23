package apphttp

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
)

// NewSPAHandler раздаёт собранный React SPA из fs.FS.
// Статические файлы (assets/, *.js, *.css) отдаются напрямую.
// Любой путь, не совпадающий с файлом в FS, возвращает index.html —
// это позволяет react-router-dom обрабатывать клиентскую навигацию.
//
// seo (опционально) внедряет серверный SEO (title/meta/OG/JSON-LD + блок текста)
// в index.html под конкретный маршрут — чтобы поисковики видели контент без JS.
// Если seo == nil, index.html отдаётся как есть.
func NewSPAHandler(static fs.FS, seo *SEOInjector) http.Handler {
	fileServer := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origPath := r.URL.Path
		// Нормализуем путь: убираем ведущий слэш для fs.Open.
		path := strings.TrimPrefix(origPath, "/")
		if path == "" {
			path = "index.html"
		}

		serveIndex := false
		if f, err := static.Open(path); err != nil {
			// Файл не найден — отдаём index.html для клиентского роутера.
			serveIndex = true
		} else {
			f.Close()
		}

		// index.html не кэшируем: иначе браузер застревает на старой версии,
		// ссылающейся на уже несуществующие хеши ассетов.
		if serveIndex || path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			if seo != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = io.WriteString(w, seo.Render(r.Context(), origPath, requestBaseURL(r)))
				return
			}
			// Без инъектора — отдаём index.html напрямую через FileServer.
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Хешированные ассеты (assets/index-XXXX.js) кэшируем надолго — их имя
		// меняется при каждой сборке, поэтому stale-версий не бывает.
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}
