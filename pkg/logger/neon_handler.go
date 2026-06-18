package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

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
	neonRed     = "\033[38;5;196m" // Добавлен красный для явных ошибок
)

func levelStyle(level slog.Level) (color, badge string) {
	switch {
	case level < slog.LevelInfo:
		return neonPurple, "DEBUG"
	case level < slog.LevelWarn:
		return neonGreen, "INFO "
	case level < slog.LevelError:
		return neonAmber, "WARN "
	default:
		return neonRed, "ERROR"
	}
}

type NeonHandler struct {
	mu     *sync.Mutex
	w      io.Writer
	level  slog.Level
	attrs  []slog.Attr
	prefix string // ОПТИМИЗАЦИЯ: храним уже склеенный префикс группы, а не срез строк
}

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
	merged := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	merged = append(merged, attrs...)
	return &NeonHandler{mu: h.mu, w: h.w, level: h.level, attrs: merged, prefix: h.prefix}
}

func (h *NeonHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	// ОПТИМИЗАЦИЯ: склеиваем префикс 1 раз при создании группы, а не на каждом логе
	return &NeonHandler{mu: h.mu, w: h.w, level: h.level, attrs: h.attrs, prefix: h.prefix + name + "."}
}

func (h *NeonHandler) Handle(_ context.Context, r slog.Record) error {
	color, badge := levelStyle(r.Level)

	var b strings.Builder
	b.Grow(128) // ОПТИМИЗАЦИЯ: предвыделяем память, чтобы избежать реаллокаций при конкатенации

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
		writeAttr(&b, h.prefix, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, h.prefix, a)
		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func writeAttr(b *strings.Builder, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve() // ВАЖНО: резолвим ленивые значения (LogValuer)
	if a.Equal(slog.Attr{}) {
		return
	}

	// ВАЖНО: правильная обработка вложенных групп
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return
		}
		newPrefix := prefix + a.Key + "."
		for _, ga := range attrs {
			writeAttr(b, newPrefix, ga)
		}
		return
	}

	key := prefix + a.Key

	// Умная раскраска: если это ошибка, подсвечиваем значение ярко
	valColor := neonMagenta
	if key == "error" || key == "err" || strings.HasSuffix(key, ".error") {
		valColor = neonAmber
	}

	b.WriteByte(' ')
	b.WriteString(neonCyan)
	b.WriteString(key)
	b.WriteString(ansiReset)
	b.WriteString(ansiDim)
	b.WriteString("=")
	b.WriteString(ansiReset)
	b.WriteString(valColor)
	fmt.Fprintf(b, "%v", a.Value.Any())
	b.WriteString(ansiReset)
}
