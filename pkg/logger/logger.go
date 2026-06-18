package logger

import (
	"log/slog"
	"os"
)

// New создаёт структурированный логгер. В production используется обычный
// JSON (агрегаторы типа ELK/Loki не понимают ANSI escape), в dev - неоновый
// цветной хендлер для приятной работы глазами в терминале.
func New(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "dev" {
		level = slog.LevelDebug
	}

	var handler slog.Handler
	switch env {
	case "production":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	default:
		handler = NewNeonHandler(os.Stdout, level)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}
