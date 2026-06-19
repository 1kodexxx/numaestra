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
		TrackURLs:   []string{"https://s3/1.mp3"},
		TracksCount: 1,
	})
	if err != nil {
		t.Fatalf("LogNotifier не должен возвращать ошибку: %v", err)
	}
}

func TestLogNotifier_ImplementsNotifier(t *testing.T) {
	var _ Notifier = NewLogNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
}
