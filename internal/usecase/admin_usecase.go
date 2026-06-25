package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/robokassa"
)

// accountInitialTokenBalance — стартовое значение локального счётчика token_balance
// для аккаунтов, добавляемых через админку. Большое значение, т.к. при единой
// модели TTAPI это не реальный баланс провайдера, а лишь soft-лимитер пула.
const accountInitialTokenBalance = 1_000_000

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
	notifier      notify.Notifier
	queue         domain.QueuePublisher // nil → перегенерация недоступна
	log           *slog.Logger
}

// WithQueue включает перегенерацию заказов (постановку задачи генерации в очередь).
func (uc *AdminUseCase) WithQueue(q domain.QueuePublisher) *AdminUseCase {
	uc.queue = q
	return uc
}

// NewAdminUseCase создаёт AdminUseCase.
func NewAdminUseCase(
	orderRepo domain.OrderRepository,
	accountRepo domain.AccountRepository,
	categoryRepo domain.CategoryRepository,
	rk Refunder,
	categoryCache categoryCacheInvalidator,
	notifier notify.Notifier,
	log *slog.Logger,
) *AdminUseCase {
	return &AdminUseCase{
		orderRepo:     orderRepo,
		accountRepo:   accountRepo,
		categoryRepo:  categoryRepo,
		rk:            rk,
		categoryCache: categoryCache,
		notifier:      notifier,
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
	// token_balance — локальный счётчик-ограничитель, а НЕ реальный баланс провайдера.
	// При единой модели авторизации TTAPI (общий SUNO_API_KEY) реальные кредиты живут
	// в кабинете TTAPI; здесь баланс лишь не даёт аккаунту «закончиться» искусственно,
	// поэтому стартуем с большого значения (иначе аккаунт ушёл бы в out_of_tokens
	// после сотни заказов на пустом месте).
	acc, err := domain.NewSunoAccount(email, encryptedSession, accountInitialTokenBalance)
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
// При активации (active) маршрутизируется через ResetAccount: иначе остаточный
// failure_count или cooldown_until сделали бы реактивацию хрупкой — первый же
// сбой снова забанил бы аккаунт, а просроченная пауза не снялась бы.
func (uc *AdminUseCase) SetAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
	switch status {
	case domain.AccountStatusActive:
		// Реактивация без обнуления слотов: счётчик concurrent_tasks может отражать
		// реально идущие генерации. Для «застрявших» слотов есть отдельный ResetAccount.
		if err := uc.accountRepo.ResetAccount(ctx, id, false); err != nil {
			return fmt.Errorf("реактивация аккаунта: %w", err)
		}
		uc.log.Info("аккаунт реактивирован вручную", "account_id", id)
		return nil
	case domain.AccountStatusCooldown, domain.AccountStatusBanned, domain.AccountStatusOutOfTokens:
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

// ResetAccount полностью «достаёт» зависший аккаунт: статус active, сброс
// failure_count и cooldown_until, обнуление concurrent_tasks (освобождение
// «утёкших» после краша воркера слотов). Это та самая кнопка в админке для
// аккаунта, который завис с занятыми слотами или ушёл в бан из-за серии сбоев.
func (uc *AdminUseCase) ResetAccount(ctx context.Context, id uuid.UUID) error {
	if err := uc.accountRepo.ResetAccount(ctx, id, true); err != nil {
		return fmt.Errorf("сброс аккаунта: %w", err)
	}
	uc.log.Info("аккаунт сброшен и возвращён в пул вручную", "account_id", id)
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

// RegenerateOrder повторно ставит оплаченный, но упавший заказ в очередь генерации.
// Нужен, когда клиент заплатил, а генерация сорвалась (например, был пуст пул
// аккаунтов или провайдер вернул ошибку) — чтобы не оставлять клиента без трека.
func (uc *AdminUseCase) RegenerateOrder(ctx context.Context, orderID uuid.UUID) error {
	if uc.queue == nil {
		return fmt.Errorf("очередь генерации недоступна")
	}
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("получение заказа для перегенерации: %w", err)
	}
	if err := order.Regenerate(); err != nil {
		return fmt.Errorf("перегенерация недоступна: %w", err)
	}
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("сохранение заказа: %w", err)
	}
	if err := uc.queue.EnqueueGenerationTask(ctx, orderID); err != nil {
		return fmt.Errorf("постановка задачи генерации: %w", err)
	}
	uc.log.Info("admin: заказ отправлен на перегенерацию", "order_id", orderID)
	return nil
}

// SendOrderFeedback фиксирует сообщение администратора по заказу и отправляет
// его клиенту на email. Сохраняется в БД даже если письмо не дошло (например,
// SMTP временно недоступен) — переписку всё равно нужно видеть в админке;
// ошибка отправки письма возвращается отдельно, чтобы админ узнал о сбое.
func (uc *AdminUseCase) SendOrderFeedback(ctx context.Context, orderID uuid.UUID, message string) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("получение заказа: %w", err)
	}

	if err := order.SetAdminFeedback(message); err != nil {
		return fmt.Errorf("валидация сообщения: %w", err)
	}

	if err := uc.orderRepo.SetAdminFeedback(ctx, orderID, order.AdminFeedback(), *order.AdminFeedbackAt()); err != nil {
		return fmt.Errorf("сохранение обратной связи: %w", err)
	}

	if err := uc.notifier.NotifyAdminFeedback(ctx, notify.AdminFeedbackNotification{
		OrderID: orderID.String(),
		Email:   order.CustomerEmail(),
		Message: message,
	}); err != nil {
		uc.log.Error("не удалось отправить письмо с обратной связью клиенту",
			"order_id", orderID, "err", err)
		return fmt.Errorf("отправка письма клиенту: %w", err)
	}

	uc.log.Info("обратная связь по заказу отправлена клиенту", "order_id", orderID)
	return nil
}

// ==========================================
// Категории и вопросы квиза (server-driven UI)
// ==========================================

// ListCategories возвращает все категории (без вопросов) для списка в админке.
// В отличие от PromptUseCase.GetAllCategories — без кеша и с base_prompt_template,
// который скрыт в публичном MarshalJSON, но нужен администратору для редактирования.
func (uc *AdminUseCase) ListCategories(ctx context.Context) ([]*domain.Category, error) {
	categories, err := uc.categoryRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("получение списка категорий: %w", err)
	}
	return categories, nil
}

// GetCategory возвращает категорию со всеми вопросами для формы редактирования в админке.
func (uc *AdminUseCase) GetCategory(ctx context.Context, id string) (*domain.Category, error) {
	category, err := uc.categoryRepo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.ErrCategoryNotFound
	}
	return category, nil
}

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
func (uc *AdminUseCase) AddQuestion(ctx context.Context, categoryID string, stepNumber int, questionText, uiType, mappingKey string, isRequired bool, optionSource string, config domain.QuestionConfig, options []domain.Option) (domain.Question, error) {
	q, err := domain.NewQuestion(stepNumber, questionText, uiType, mappingKey, isRequired, optionSource, config, options)
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
func (uc *AdminUseCase) UpdateQuestion(ctx context.Context, categoryID string, questionID, stepNumber int, questionText, uiType, mappingKey string, isRequired bool, optionSource string, config domain.QuestionConfig, options []domain.Option) error {
	q, err := domain.NewQuestion(stepNumber, questionText, uiType, mappingKey, isRequired, optionSource, config, options)
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
