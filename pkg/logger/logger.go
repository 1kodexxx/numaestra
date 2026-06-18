package logger

import (
	"log/slog"
	"os"
)

// New создаёт структурированный глобальный логгер.
// В "dev" используется цветной NeonHandler (уровень Debug) для удобства в терминале.
// В любых других окружениях (production, staging) - строгий JSON (уровень Info).
func New(env string) *slog.Logger {
	var handler slog.Handler

	if env == "dev" {
		handler = NewNeonHandler(os.Stdout, slog.LevelDebug)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	l := slog.New(handler)
	slog.SetDefault(l)

	return l
}
