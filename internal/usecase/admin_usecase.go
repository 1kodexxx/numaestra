package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/robokassa"
)

// Refunder — порт инициации возврата платежа. Реализуется *robokassa.Client;
// интерфейс позволяет заменить реализацию в тестах без HTTP-запросов.
type Refunder interface {
	Refund(ctx context.Context, outSum string, invID int64) error
}

// AdminUseCase предоставляет операции управления для административного API:
// добавление и управление Suno-аккаунтами, просмотр заказов, инициация возвратов.
type AdminUseCase struct {
	orderRepo   domain.OrderRepository
	accountRepo domain.AccountRepository
	rk          Refunder
	log         *slog.Logger
}

// NewAdminUseCase создаёт AdminUseCase.
func NewAdminUseCase(
	orderRepo domain.OrderRepository,
	accountRepo domain.AccountRepository,
	rk Refunder,
	log *slog.Logger,
) *AdminUseCase {
	return &AdminUseCase{
		orderRepo:   orderRepo,
		accountRepo: accountRepo,
		rk:          rk,
		log:         log,
	}
}

// AddAccount добавляет новый Suno-аккаунт в пул.
// encryptedSession — сессия, зашифрованная перед передачей в API.
// maxConcurrent <= 0 использует значение по умолчанию (1).
func (uc *AdminUseCase) AddAccount(ctx context.Context, email, encryptedSession string, maxConcurrent int) (*domain.SunoAccount, error) {
	acc, err := domain.NewSunoAccount(email, encryptedSession, 100)
	if err != nil {
		return nil, fmt.Errorf("создание аккаунта: %w", err)
	}
	if maxConcurrent > 1 {
		if err := acc.SetMaxConcurrentTasks(maxConcurrent); err != nil {
			return nil, fmt.Errorf("установка лимита задач: %w", err)
		}
	}
	if err := uc.accountRepo.Create(ctx, acc); err != nil {
		return nil, fmt.Errorf("сохранение аккаунта: %w", err)
	}
	uc.log.Info("добавлен новый Suno-аккаунт", "account_id", acc.ID(), "email", acc.Email())
	return acc, nil
}

// ListAccounts возвращает все аккаунты пула.
func (uc *AdminUseCase) ListAccounts(ctx context.Context) ([]*domain.SunoAccount, error) {
	accounts, err := uc.accountRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("получение списка аккаунтов: %w", err)
	}
	return accounts, nil
}

// SetAccountStatus меняет статус аккаунта (active / cooldown / banned).
func (uc *AdminUseCase) SetAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
	switch status {
	case domain.AccountStatusActive, domain.AccountStatusCooldown, domain.AccountStatusBanned, domain.AccountStatusOutOfTokens:
		// допустимые статусы для ручного управления
	default:
		return fmt.Errorf("недопустимый статус %q для ручного управления", status)
	}
	if err := uc.accountRepo.SetStatus(ctx, id, status); err != nil {
		return fmt.Errorf("смена статуса аккаунта: %w", err)
	}
	uc.log.Info("статус аккаунта изменён вручную", "account_id", id, "new_status", status)
	return nil
}

// ListOrders возвращает страницу всех заказов и общее их количество.
// page и perPage — 1-индексированные (page=1 → первая страница).
func (uc *AdminUseCase) ListOrders(ctx context.Context, page, perPage int) ([]*domain.Order, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	total, err := uc.orderRepo.CountAll(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("подсчёт заказов: %w", err)
	}

	orders, err := uc.orderRepo.ListAll(ctx, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("получение списка заказов: %w", err)
	}
	return orders, total, nil
}

// GetOrder возвращает заказ по ID для детального просмотра в Admin API.
func (uc *AdminUseCase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("получение заказа: %w", err)
	}
	return order, nil
}

// RefundOrder инициирует возврат платежа через Robokassa API и обновляет статус заказа.
// Возврат допустим только для оплаченных заказов (payment_status = paid).
// Statuses "completed" тоже допустимы — payment_status у них тоже paid.
func (uc *AdminUseCase) RefundOrder(ctx context.Context, orderID uuid.UUID) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("получение заказа для возврата: %w", err)
	}

	if order.PaymentStatus() != domain.PaymentStatusPaid {
		return fmt.Errorf("возврат невозможен: статус оплаты %q (требуется paid)", order.PaymentStatus())
	}

	outSum := robokassa.FormatAmount(order.AmountKopecks())
	if err := uc.rk.Refund(ctx, outSum, order.InvoiceID()); err != nil {
		return fmt.Errorf("вызов Robokassa Refund API: %w", err)
	}

	if err := order.MarkRefunded(); err != nil {
		return fmt.Errorf("обновление статуса заказа: %w", err)
	}

	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("сохранение заказа после возврата: %w", err)
	}

	uc.log.Info("возврат платежа выполнен", "order_id", orderID, "amount", outSum)
	return nil
}
