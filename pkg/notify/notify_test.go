package notify

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestLogNotifier_NotifyOrderComplete(t *testing.T) {
	n := NewLogNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := n.NotifyOrderComplete(context.Background(), OrderCompleteNotification{
		OrderID:     "order-1",
		Email:       "user@example.com",
		TracksCount: 1,
	})
	if err != nil {
		t.Fatalf("LogNotifier не должен возвращать ошибку: %v", err)
	}
}

func TestLogNotifier_ImplementsNotifier(t *testing.T) {
	var _ Notifier = NewLogNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestLogNotifier_NotifyOrderFailed(t *testing.T) {
	n := NewLogNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := n.NotifyOrderFailed(context.Background(), OrderFailedNotification{
		OrderID: "order-1", Email: "user@example.com",
	})
	if err != nil {
		t.Fatalf("LogNotifier.NotifyOrderFailed не должен возвращать ошибку: %v", err)
	}
}

func TestLogNotifier_NotifyAdminFeedback(t *testing.T) {
	n := NewLogNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := n.NotifyAdminFeedback(context.Background(), AdminFeedbackNotification{
		OrderID: "order-1", Email: "user@example.com", Message: "Привет!",
	})
	if err != nil {
		t.Fatalf("LogNotifier.NotifyAdminFeedback не должен возвращать ошибку: %v", err)
	}
}

func TestLogNotifier_NotifyAccessLink(t *testing.T) {
	n := NewLogNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := n.NotifyAccessLink(context.Background(), AccessLinkNotification{
		OrderID: "order-1", Email: "user@example.com", AccessToken: "tok123",
	})
	if err != nil {
		t.Fatalf("LogNotifier.NotifyAccessLink не должен возвращать ошибку: %v", err)
	}
}
