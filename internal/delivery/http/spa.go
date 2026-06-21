package apphttp

import (
	"io/fs"
	"net/http"
	"strings"
)

// NewSPAHandler раздаёт собранный React SPA из fs.FS.
// Статические файлы (assets/, *.js, *.css) отдаются напрямую.
// Любой путь, не совпадающий с файлом в FS, возвращает index.html —
// это позволяет react-router-dom обрабатывать клиентскую навигацию.
func NewSPAHandler(static fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Нормализуем путь: убираем ведущий слэш для fs.Open.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		serveIndex := false
		f, err := static.Open(path)
		if err != nil {
			// Файл не найден — отдаём index.html для клиентского роутера.
			serveIndex = true
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		} else {
			f.Close()
		}

		// index.html не кэшируем: иначе браузер застревает на старой версии
		// страницы, которая ссылается на уже несуществующие хеши ассетов.
		// Хешированные ассеты (assets/index-XXXX.js), наоборот, кэшируем надолго —
		// их имя меняется при каждой сборке, поэтому stale-версий не бывает.
		switch {
		case serveIndex || path == "index.html":
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		case strings.HasPrefix(path, "assets/"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		fileServer.ServeHTTP(w, r)
	})
}
