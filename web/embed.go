// Package web содержит собранный React-фронтенд, встроенный в бинарник через go:embed.
// Файлы генерируются командой `make frontend-build` (npm run build в frontend/).
package web

import "embed"

// FS — встроенная файловая система со статическими ресурсами React SPA.
// Корень — директория out/, которую Vite записывает при сборке.
//
//go:embed out
var FS embed.FS
