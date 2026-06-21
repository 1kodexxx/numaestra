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

// categoryCacheInvalidator — порт сброса кеша каталога категорий.
// Реализуется *PromptUseCase; позволяет AdminUseCase не зависеть от его
// конкретной реализации и не тащить кеш в тесты, где он не нужен (nil-safe).
type categoryCacheInvalidator interface {
	InvalidateCache()
}

// AdminUseCase предоставляет операции управления для административного API:
// добавление и управление Suno-аккаунтами, просмотр заказов, инициация возвратов,
// CRUD категорий и вопросов квиза.
type AdminUseCase struct {
	orderRepo     domain.OrderRepository
	accountRepo   domain.AccountRepository
	categoryRepo  domain.CategoryRepository
	rk            Refunder
	categoryCache categoryCacheInvalidator
	log           *slog.Logger
}

// NewAdminUseCase создаёт AdminUseCase.
func NewAdminUseCase(
	orderRepo domain.OrderRepository,
	accountRepo domain.AccountRepository,
	categoryRepo domain.CategoryRepository,
	rk Refunder,
	categoryCache categoryCacheInvalidator,
	log *slog.Logger,
) *AdminUseCase {
	return &AdminUseCase{
		orderRepo:     orderRepo,
		accountRepo:   accountRepo,
		categoryRepo:  categoryRepo,
		rk:            rk,
		categoryCache: categoryCache,
		log:           log,
	}
}

func (uc *AdminUseCase) invalidateCategoryCache() {
	if uc.categoryCache != nil {
		uc.categoryCache.InvalidateCache()
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

// ==========================================
// Категории и вопросы квиза (server-driven UI)
// ==========================================

// CreateCategory создаёт новую категорию каталога (например, новый повод для
// песни или "general"/"freeform" — категорию-заглушку для свободного сценария
// без жёсткого набора вопросов).
func (uc *AdminUseCase) CreateCategory(ctx context.Context, id, title, description, coverImageURL string, seoTags []string, basePromptTemplate string) (*domain.Category, error) {
	category, err := domain.NewCategory(id, title, description, coverImageURL, seoTags, basePromptTemplate)
	if err != nil {
		return nil, fmt.Errorf("валидация категории: %w", err)
	}
	if err := uc.categoryRepo.Create(ctx, category); err != nil {
		return nil, err
	}
	uc.invalidateCategoryCache()
	uc.log.Info("admin: создана категория", "category_id", id, "title", title)
	return category, nil
}

// UpdateCategory обновляет изменяемые поля существующей категории.
func (uc *AdminUseCase) UpdateCategory(ctx context.Context, id, title, description, coverImageURL string, seoTags []string, basePromptTemplate string) (*domain.Category, error) {
	category, err := uc.categoryRepo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.ErrCategoryNotFound
	}
	if err := category.UpdateDetails(title, description, coverImageURL, seoTags, basePromptTemplate); err != nil {
		return nil, fmt.Errorf("валидация категории: %w", err)
	}
	if err := uc.categoryRepo.Update(ctx, category); err != nil {
		return nil, err
	}
	uc.invalidateCategoryCache()
	uc.log.Info("admin: обновлена категория", "category_id", id)
	return category, nil
}

// DeleteCategory удаляет категорию вместе со всеми её вопросами (каскадно).
// Уже созданные заказы с этой category_id не затрагиваются — у них уже есть
// готовый sunoPrompt, сформированный на момент заказа.
func (uc *AdminUseCase) DeleteCategory(ctx context.Context, id string) error {
	if err := uc.categoryRepo.Delete(ctx, id); err != nil {
		return err
	}
	uc.invalidateCategoryCache()
	uc.log.Info("admin: удалена категория", "category_id", id)
	return nil
}

// AddQuestion добавляет новый вопрос квиза к категории.
func (uc *AdminUseCase) AddQuestion(ctx context.Context, categoryID string, stepNumber int, questionText, uiType, mappingKey string, isRequired bool, options []domain.Option) (domain.Question, error) {
	q, err := domain.NewQuestion(stepNumber, questionText, uiType, mappingKey, isRequired, options)
	if err != nil {
		return domain.Question{}, fmt.Errorf("валидация вопроса: %w", err)
	}
	saved, err := uc.categoryRepo.AddQuestion(ctx, categoryID, q)
	if err != nil {
		return domain.Question{}, err
	}
	uc.invalidateCategoryCache()
	uc.log.Info("admin: добавлен вопрос", "category_id", categoryID, "question_id", saved.ID)
	return saved, nil
}

// UpdateQuestion перезаписывает вопрос квиза (включая полную замену вариантов ответов).
func (uc *AdminUseCase) UpdateQuestion(ctx context.Context, categoryID string, questionID, stepNumber int, questionText, uiType, mappingKey string, isRequired bool, options []domain.Option) error {
	q, err := domain.NewQuestion(stepNumber, questionText, uiType, mappingKey, isRequired, options)
	if err != nil {
		return fmt.Errorf("валидация вопроса: %w", err)
	}
	q.ID = questionID
	if err := uc.categoryRepo.UpdateQuestion(ctx, categoryID, q); err != nil {
		return err
	}
	uc.invalidateCategoryCache()
	uc.log.Info("admin: обновлён вопрос", "category_id", categoryID, "question_id", questionID)
	return nil
}

// DeleteQuestion удаляет вопрос квиза.
func (uc *AdminUseCase) DeleteQuestion(ctx context.Context, categoryID string, questionID int) error {
	if err := uc.categoryRepo.DeleteQuestion(ctx, categoryID, questionID); err != nil {
		return err
	}
	uc.invalidateCategoryCache()
	uc.log.Info("admin: удалён вопрос", "category_id", categoryID, "question_id", questionID)
	return nil
}
