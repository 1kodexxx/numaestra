package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func newTestHandler(buf *bytes.Buffer, level slog.Level) *NeonHandler {
	return NewNeonHandler(buf, level)
}

func TestNeonHandler_Enabled(t *testing.T) {
	h := NewNeonHandler(&bytes.Buffer{}, slog.LevelInfo)
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Debug не должен быть включён при пороге Info")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("Error должен быть включён при пороге Info")
	}
}

func TestNeonHandler_Handle_WritesMessageAndAttrs(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(newTestHandler(&buf, slog.LevelDebug))
	l.Info("заказ создан", "order_id", "abc-123")

	out := buf.String()
	if !strings.Contains(out, "заказ создан") {
		t.Error("вывод должен содержать сообщение")
	}
	if !strings.Contains(out, "order_id") || !strings.Contains(out, "abc-123") {
		t.Errorf("вывод должен содержать атрибут order_id=abc-123: %s", out)
	}
	if !strings.Contains(out, "[INFO ]") {
		t.Errorf("вывод должен содержать бейдж уровня: %s", out)
	}
}

func TestNeonHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(newTestHandler(&buf, slog.LevelDebug))
	l := base.With("service", "numaestra")
	l.Info("старт")

	if !strings.Contains(buf.String(), "service") || !strings.Contains(buf.String(), "numaestra") {
		t.Errorf("атрибуты из With должны попадать в вывод: %s", buf.String())
	}
}

func TestNeonHandler_WithGroup_Prefixes(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(newTestHandler(&buf, slog.LevelDebug))
	l := base.WithGroup("db")
	l.Info("query", "rows", 5)

	if !strings.Contains(buf.String(), "db.rows") {
		t.Errorf("ключ должен иметь префикс группы db.: %s", buf.String())
	}
}

func TestNeonHandler_NestedGroupAttr(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(newTestHandler(&buf, slog.LevelDebug))
	l.Info("событие", slog.Group("http", slog.Int("status", 200)))

	if !strings.Contains(buf.String(), "http.status") {
		t.Errorf("вложенная группа должна давать ключ http.status: %s", buf.String())
	}
}

func TestNeonHandler_LevelBadges(t *testing.T) {
	cases := []struct {
		level slog.Level
		badge string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO "},
		{slog.LevelWarn, "WARN "},
		{slog.LevelError, "ERROR"},
	}
	for _, tt := range cases {
		var buf bytes.Buffer
		l := slog.New(newTestHandler(&buf, slog.LevelDebug))
		l.Log(context.Background(), tt.level, "msg")
		if !strings.Contains(buf.String(), "["+tt.badge+"]") {
			t.Errorf("уровень %v: ожидали бейдж %q, вывод: %s", tt.level, tt.badge, buf.String())
		}
	}
}

func TestNeonHandler_EmptyGroupReturnsSelf(t *testing.T) {
	h := NewNeonHandler(&bytes.Buffer{}, slog.LevelInfo)
	if h.WithGroup("") != h {
		t.Error("WithGroup с пустым именем должен возвращать тот же handler")
	}
	if h.WithAttrs(nil) != h {
		t.Error("WithAttrs с пустым списком должен возвращать тот же handler")
	}
}
