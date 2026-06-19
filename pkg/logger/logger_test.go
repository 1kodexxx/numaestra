package logger

import (
	"log/slog"
	"testing"
)

func TestNew_Dev(t *testing.T) {
	l := New("dev")
	if l == nil {
		t.Fatal("логгер не должен быть nil")
	}
	if !l.Enabled(nil, slog.LevelDebug) {
		t.Error("в dev-режиме должен быть включён уровень Debug")
	}
}

func TestNew_Production(t *testing.T) {
	l := New("production")
	if l == nil {
		t.Fatal("логгер не должен быть nil")
	}
	if l.Enabled(nil, slog.LevelDebug) {
		t.Error("в production уровень Debug должен быть отключён")
	}
	if !l.Enabled(nil, slog.LevelInfo) {
		t.Error("в production уровень Info должен быть включён")
	}
}
