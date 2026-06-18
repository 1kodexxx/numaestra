package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// ANSI-коды cyberpunk neon палитры. Продублированы относительно pkg/banner
// намеренно: pkg/logger и pkg/banner - независимые by design пакеты уровня pkg,
// и не должны зависеть друг от друга из-за общей палитры.
const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"

	neonCyan    = "\033[38;5;51m"
	neonMagenta = "\033[38;5;201m"
	neonPurple  = "\033[38;5;141m"
	neonGreen   = "\033[38;5;48m"
	neonAmber   = "\033[38;5;214m"
	neonGrey    = "\033[38;5;102m"
)

// levelStyle возвращает цвет и бейдж уровня лога в стиле неоновых вывесок.
func levelStyle(level slog.Level) (color, badge string) {
	switch {
	case level < slog.LevelInfo:
		return neonPurple, "DEBUG"
	case level < slog.LevelWarn:
		return neonGreen, "INFO "
	case level < slog.LevelError:
		return neonAmber, "WARN "
	default:
		return neonMagenta, "ERROR"
	}
}

// NeonHandler - реализация slog.Handler в стиле cyberpunk neon: цветные
// бейджи уровней, приглушённые таймстемпы, атрибуты как светящиеся
// key=value пары. Предназначен для интерактивной работы в терминале при
// локальной разработке; в production используется обычный JSON-хендлер
// (см. New в logger.go) - агрегаторы логов не понимают ANSI escape.
type NeonHandler struct {
	mu     *sync.Mutex
	w      io.Writer
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

// NewNeonHandler создаёт обработчик логов с неоновой подсветкой для вывода в w.
func NewNeonHandler(w io.Writer, level slog.Level) *NeonHandler {
	return &NeonHandler{
		mu:    &sync.Mutex{},
		w:     w,
		level: level,
	}
}

func (h *NeonHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *NeonHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	merged := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	copy(merged[len(h.attrs):], attrs)
	return &NeonHandler{mu: h.mu, w: h.w, level: h.level, attrs: merged, groups: h.groups}
}

func (h *NeonHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	merged := make([]string, len(h.groups)+1)
	copy(merged, h.groups)
	merged[len(h.groups)] = name
	return &NeonHandler{mu: h.mu, w: h.w, level: h.level, attrs: h.attrs, groups: merged}
}

func (h *NeonHandler) Handle(_ context.Context, r slog.Record) error {
	color, badge := levelStyle(r.Level)

	var b strings.Builder

	b.WriteString(neonGrey)
	b.WriteString(r.Time.Format("15:04:05.000"))
	b.WriteString(ansiReset)
	b.WriteByte(' ')

	b.WriteString(ansiBold)
	b.WriteString(color)
	b.WriteString("[")
	b.WriteString(badge)
	b.WriteString("]")
	b.WriteString(ansiReset)
	b.WriteByte(' ')

	b.WriteString(ansiBold)
	b.WriteString(r.Message)
	b.WriteString(ansiReset)

	for _, a := range h.attrs {
		writeAttr(&b, h.groups, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, h.groups, a)
		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func writeAttr(b *strings.Builder, groups []string, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	key := a.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}

	b.WriteByte(' ')
	b.WriteString(neonCyan)
	b.WriteString(key)
	b.WriteString(ansiReset)
	b.WriteString(ansiDim)
	b.WriteString("=")
	b.WriteString(ansiReset)
	b.WriteString(neonMagenta)
	fmt.Fprintf(b, "%v", a.Value.Any())
	b.WriteString(ansiReset)
}
