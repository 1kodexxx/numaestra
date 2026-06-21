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

		f, err := static.Open(path)
		if err != nil {
			// Файл не найден — отдаём index.html для клиентского роутера.
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		} else {
			f.Close()
		}

		fileServer.ServeHTTP(w, r)
	})
}
