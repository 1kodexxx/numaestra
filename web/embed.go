// Package web содержит собранный React-фронтенд, встроенный в бинарник через go:embed.
// Файлы генерируются командой `make frontend-build` (npm run build в frontend/).
package web

import "embed"

// FS — встроенная файловая система со статическими ресурсами React SPA.
// out/ — директория, которую Vite записывает при сборке.
// images/ — SVG-заглушки обложек категорий, раздаваемые по /images/*.
//
//go:embed out images
var FS embed.FS
