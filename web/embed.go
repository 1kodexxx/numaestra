// Package web содержит статические файлы фронтенда, встроенные в бинарник через go:embed.
package web

import "embed"

// FS — встроенная файловая система со всеми статическими ресурсами SPA.
//
//go:embed index.html
var FS embed.FS
