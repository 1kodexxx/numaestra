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

	// NotifyOrderFailed отправляет клиенту уведомление о неудаче генерации
	// и просьбу обратиться в поддержку. Вызывается из двух мест:
	// 1. CheckGenerationStatus — Suno вернул статус failed.
	// 2. FailGeneration — исчерпаны все ретраи Asynq-задачи.
	NotifyOrderFailed(ctx context.Context, n OrderFailedNotification) error

	// NotifyAdminFeedback отправляет клиенту сообщение администратора по заказу
	// (ответ на вопрос, уточнение деталей и т.п.). Email может быть пустым,
	// если у заказа указан только телефон — реализация молча пропускает отправку.
	NotifyAdminFeedback(ctx context.Context, n AdminFeedbackNotification) error

	// NotifyAccessLink отправляет клиенту ссылку на управление заказом (оплата, share).
	// Email должен быть непустым — вызывающий код проверяет совпадение с заказом.
	NotifyAccessLink(ctx context.Context, n AccessLinkNotification) error

	// NotifyAdmin отправляет администратору письмо о событии заказа (новая оплата,
	// готовое демо, провал генерации) на его собственный адрес (ADMIN_NOTIFY_EMAIL).
	// Получателя знает сама реализация, а не вызывающий код. Если админский адрес
	// не настроен — реализация молча пропускает отправку.
	NotifyAdmin(ctx context.Context, n AdminEventNotification) error
}

// AdminEventKind — тип админского события для NotifyAdmin.
type AdminEventKind string

const (
	AdminEventPaidOrder        AdminEventKind = "paid_order"        // заказ оплачен (продажа)
	AdminEventDemoReady        AdminEventKind = "demo_ready"        // готово бесплатное демо
	AdminEventDemoFailed       AdminEventKind = "demo_failed"       // бесплатное демо сорвалось (клиент может уйти, не оплатив)
	AdminEventGenerationFailed AdminEventKind = "generation_failed" // генерация оплаченного заказа сорвалась
)

// AdminEventNotification — данные письма администратору о событии заказа.
type AdminEventNotification struct {
	Kind          AdminEventKind
	OrderID       string
	InvoiceID     int64
	CustomerEmail string
	CustomerPhone string
	AmountKopecks int64
	Brief         string
	FailureReason string // для AdminEventGenerationFailed и AdminEventDemoFailed
}

// OrderCompleteNotification содержит данные для уведомления о завершении заказа.
type OrderCompleteNotification struct {
	OrderID     string
	AccessToken string // токен доступа клиента — используется для формирования magic-link
	Email       string
	Phone       string
	// TracksCount — число готовых версий (для текста письма). Прямые ссылки на
	// mp3 в письме не передаём: при presign они временные, основной путь к трекам —
	// status-ссылка на сайт.
	TracksCount int
}

// OrderFailedNotification содержит данные для уведомления о неудаче генерации.
type OrderFailedNotification struct {
	OrderID     string
	AccessToken string
	Email       string
	Phone       string
}

// AdminFeedbackNotification содержит данные письма с обратной связью администратора.
type AdminFeedbackNotification struct {
	OrderID string
	Email   string
	Message string
}

// AccessLinkNotification — письмо со ссылкой на страницу статуса с access_token.
type AccessLinkNotification struct {
	OrderID     string
	AccessToken string
	Email       string
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

func (n *LogNotifier) NotifyOrderFailed(_ context.Context, notification OrderFailedNotification) error {
	n.log.Warn("уведомление о провале генерации (stub — реальная отправка не настроена)",
		"order_id", notification.OrderID,
		"email", notification.Email,
		"phone", notification.Phone,
	)
	return nil
}

func (n *LogNotifier) NotifyAdminFeedback(_ context.Context, notification AdminFeedbackNotification) error {
	n.log.Info("обратная связь администратора по заказу (stub — реальная отправка не настроена)",
		"order_id", notification.OrderID,
		"email", notification.Email,
		"message", notification.Message,
	)
	return nil
}

func (n *LogNotifier) NotifyAccessLink(_ context.Context, notification AccessLinkNotification) error {
	n.log.Info("ссылка на управление заказом (stub — реальная отправка не настроена)",
		"order_id", notification.OrderID,
		"email", notification.Email,
		"access_token", notification.AccessToken,
	)
	return nil
}

func (n *LogNotifier) NotifyAdmin(_ context.Context, notification AdminEventNotification) error {
	n.log.Info("админское уведомление о заказе (stub — реальная отправка не настроена)",
		"kind", notification.Kind,
		"order_id", notification.OrderID,
		"invoice_id", notification.InvoiceID,
	)
	return nil
}
