package logger

import (
	"log/slog"
	"os"
)

// New создаёт структурированный логгер. В production используется JSON
// (удобно агрегировать в ELK/Loki), в dev - человекочитаемый текстовый формат.
func New(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "dev" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}
