// Package notify определяет интерфейс уведомлений клиента и предоставляет
// заглушку-логгер для использования до подключения реального провайдера.
//
// Чтобы подключить SMTP, SendGrid, SMS и т.п. — реализуйте Notifier
// и передайте в NewOrderUseCase вместо NewLogNotifier.
package notify

import (
	"context"
	"log/slog"
)

// Notifier — порт отправки уведомлений клиенту.
// Реализации: LogNotifier (заглушка), SmtpNotifier, SmsNotifier и т.д.
type Notifier interface {
	// NotifyOrderComplete отправляет клиенту уведомление о готовности треков.
	// email и phone могут быть пустыми — реализация сама выбирает канал.
	NotifyOrderComplete(ctx context.Context, n OrderCompleteNotification) error
}

// OrderCompleteNotification содержит данные для уведомления о завершении заказа.
type OrderCompleteNotification struct {
	OrderID     string
	AccessToken string   // токен доступа клиента — используется для формирования magic-link
	Email       string
	Phone       string
	TrackURLs   []string // постоянные ссылки на треки в S3
	TracksCount int
}

// LogNotifier — заглушка: логирует уведомление вместо реальной отправки.
// Используется в dev-окружении и как fallback до подключения провайдера.
type LogNotifier struct {
	log *slog.Logger
}

// NewLogNotifier создаёт заглушку-логгер.
func NewLogNotifier(log *slog.Logger) *LogNotifier {
	return &LogNotifier{log: log}
}

func (n *LogNotifier) NotifyOrderComplete(_ context.Context, notification OrderCompleteNotification) error {
	n.log.Info("уведомление о готовности заказа (stub — реальная отправка не настроена)",
		"order_id", notification.OrderID,
		"email", notification.Email,
		"phone", notification.Phone,
		"tracks_count", notification.TracksCount,
	)
	return nil
}
