package migrations

import "embed"

// FS содержит все SQL-файлы миграций, встроенные в бинарник при компиляции.
// Используется в cmd/server/main.go для запуска migrate.Run при старте сервиса.
//
//go:embed *.sql
var FS embed.FS
