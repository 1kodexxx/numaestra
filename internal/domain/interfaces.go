package domain

import (
	"context"

	"github.com/google/uuid"
)

// AccountRepository - контракт для работы с хранилищем аккаунтов Suno.
type AccountRepository interface {
	// FetchAndLockAvailable атомарно находит и блокирует один свободный аккаунт
	// (в PostgreSQL-реализации - через "SELECT ... FOR UPDATE SKIP LOCKED" в транзакции),
	// переводит его в статус Busy и возвращает вызывающей стороне. Это критическая
	// операция для предотвращения race condition при параллельных запросах на генерацию.
	// Если свободных аккаунтов нет, возвращает ErrNoAvailableAccount.
	FetchAndLockAvailable(ctx context.Context) (*SunoAccount, error)

	GetByID(ctx context.Context, id uuid.UUID) (*SunoAccount, error)
	Create(ctx context.Context, account *SunoAccount) error
	Update(ctx context.Context, account *SunoAccount) error
	ListByStatus(ctx context.Context, status AccountStatus) ([]*SunoAccount, error)
}

// OrderRepository - контракт для персистентности заказов.
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
	GetByInvoiceID(ctx context.Context, invoiceID int64) (*Order, error)
	Update(ctx context.Context, order *Order) error
	ListByCustomerEmail(ctx context.Context, email string) ([]*Order, error)
}

// QueuePublisher - контракт для постановки задач в очередь (реализация поверх Asynq).
// Передаются только идентификаторы агрегатов, а не их данные: это исключает
// рассинхронизацию состояния между моментом постановки задачи и её обработкой воркером.
type QueuePublisher interface {
	// EnqueueGenerationTask ставит задачу на генерацию музыки для указанного заказа.
	EnqueueGenerationTask(ctx context.Context, orderID uuid.UUID) error

	// EnqueueStatusCheckTask ставит задачу на опрос статуса генерации в Suno (polling).
	EnqueueStatusCheckTask(ctx context.Context, orderID uuid.UUID, sunoJobID string) error
}
