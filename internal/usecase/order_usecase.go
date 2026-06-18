package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
)

// Пользовательские ошибки слоя Use-Case
var (
	ErrGenerationNotReady = errors.New("генерация еще не завершена, требуется повторный опрос")
)

type OrderUseCase struct {
	orderRepo domain.OrderRepository
	accRepo   domain.AccountRepository
	queue     domain.QueuePublisher
	provider  domain.MusicProvider
	log       *slog.Logger
}

func NewOrderUseCase(
	orderRepo domain.OrderRepository,
	accRepo domain.AccountRepository,
	queue domain.QueuePublisher,
	provider domain.MusicProvider,
	log *slog.Logger,
) *OrderUseCase {
	return &OrderUseCase{
		orderRepo: orderRepo,
		accRepo:   accRepo,
		queue:     queue,
		provider:  provider,
		log:       log,
	}
}

// ==========================================
// 1. ПОЛЬЗОВАТЕЛЬСКИЕ СЦЕНАРИИ (Вызываются из HTTP)
// ==========================================

// CreateOrder создает новый заказ. Возвращает готовый Order,
// на основе которого хендлер сгенерирует ссылку на оплату Robokassa.
func (uc *OrderUseCase) CreateOrder(ctx context.Context, email, phone, brief string, amountKopecks int64) (*domain.Order, error) {
	// Для простоты MVP используем UnixNano как уникальный ID счета (InvoiceID)
	invoiceID := time.Now().UnixNano()

	order, err := domain.NewOrder(invoiceID, email, phone, brief, amountKopecks)
	if err != nil {
		return nil, fmt.Errorf("ошибка валидации заказа: %w", err)
	}

	if err := uc.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("ошибка сохранения заказа: %w", err)
	}

	uc.log.Info("создан новый заказ", "order_id", order.ID(), "invoice_id", invoiceID)
	return order, nil
}

// HandlePaymentSuccess обрабатывает вебхук от кассы об успешной оплате.
func (uc *OrderUseCase) HandlePaymentSuccess(ctx context.Context, invoiceID int64) error {
	order, err := uc.orderRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("поиск заказа по invoice_id: %w", err)
	}

	// 1. Меняем статус оплаты в домене
	if err := order.MarkPaid(); err != nil {
		return fmt.Errorf("переход статуса оплаты: %w", err)
	}

	// 2. Ставим статус генерации "в очереди"
	if err := order.Enqueue(); err != nil {
		return fmt.Errorf("переход статуса генерации: %w", err)
	}

	// 3. Сохраняем агрегат в БД
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("обновление заказа: %w", err)
	}

	// 4. Отправляем задачу в Asynq на асинхронную генерацию
	if err := uc.queue.EnqueueGenerationTask(ctx, order.ID()); err != nil {
		uc.log.Error("ошибка постановки задачи в очередь", "order_id", order.ID(), "err", err)
		return err // Ошибка здесь критична, нужен механизм outbox в идеале, но для MVP сойдет
	}

	uc.log.Info("заказ успешно оплачен и поставлен в очередь", "order_id", order.ID())
	return nil
}

// ==========================================
// 2. ФОНОВЫЕ СЦЕНАРИИ (Вызываются воркерами Asynq)
// ==========================================

// ProcessGenerationTask берет оплаченный заказ, ищет свободный аккаунт и запускает генерацию.
func (uc *OrderUseCase) ProcessGenerationTask(ctx context.Context, orderID uuid.UUID) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("заказ не найден: %w", err)
	}

	// Проверка на идемпотентность: если задача уже в работе, скипаем
	if order.GenerationStatus() != domain.GenerationStatusQueued {
		uc.log.Warn("задача генерации пропущена (неверный статус)", "order_id", order.ID(), "status", order.GenerationStatus())
		return nil
	}

	// 1. Атомарно захватываем аккаунт (SKIP LOCKED из прошлого шага делает здесь магию!)
	account, err := uc.accRepo.FetchAndLockAvailable(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNoAvailableAccount) {
			uc.log.Warn("нет свободных аккаунтов, задача будет отложена (retry)", "order_id", orderID)
			return err // Возвращаем ошибку, чтобы Asynq сделал retry этой задачи позже
		}
		return fmt.Errorf("ошибка захвата аккаунта: %w", err)
	}

	// 2. Привязываем аккаунт к заказу
	if err := order.StartProcessing(account.ID()); err != nil {
		return fmt.Errorf("недопустимый статус для старта: %w", err)
	}

	// 3. Дергаем Suno API (через адаптер)
	req := domain.MusicGenerationRequest{
		Brief:        order.Brief(),
		Instrumental: false,
		TrackCount:   4, // Дефолт
	}
	sunoJobID, err := uc.provider.SubmitGeneration(ctx, req)

	if err != nil {
		// API упало: освобождаем аккаунт, начисляем штраф, заказ возвращаем в очередь
		uc.log.Error("ошибка API Suno", "account", account.Email(), "err", err)
		account.Release()
		account.RegisterFailure(3) // После 3 ошибок аккаунт улетит в Banned
		order.RequeueForRetry()

		_ = uc.accRepo.Update(ctx, account)
		_ = uc.orderRepo.Update(ctx, order)
		return fmt.Errorf("сбой провайдера: %w", err)
	}

	// 4. Всё ок: сохраняем заказ и ставим задачу на периодический опрос (Polling)
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return err
	}

	if err := uc.queue.EnqueueStatusCheckTask(ctx, order.ID(), sunoJobID); err != nil {
		return err
	}

	uc.log.Info("генерация успешно запущена", "order_id", order.ID(), "suno_job", sunoJobID, "account", account.Email())
	return nil
}

// CheckGenerationStatus периодически опрашивает Suno о готовности треков.
func (uc *OrderUseCase) CheckGenerationStatus(ctx context.Context, orderID uuid.UUID, providerJobID string) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}

	// Если заказ уже завершен (кто-то другой обработал), выходим
	if order.GenerationStatus() != domain.GenerationStatusProcessing {
		return nil
	}

	result, err := uc.provider.FetchResult(ctx, providerJobID)
	if err != nil {
		return fmt.Errorf("ошибка опроса статуса: %w", err)
	}

	// Если статус еще генерируется — возвращаем спец. ошибку, чтобы Asynq сделал Retry через минуту
	if result.Status == domain.MusicGenerationStatusPending || result.Status == domain.MusicGenerationStatusRunning {
		return ErrGenerationNotReady
	}

	// Загружаем аккаунт, чтобы списать токены и освободить его
	account, err := uc.accRepo.GetByID(ctx, *order.AssignedAccountID())
	if err != nil {
		return fmt.Errorf("ошибка загрузки аккаунта: %w", err)
	}

	switch result.Status {
	case domain.MusicGenerationStatusFailed:
		// Трек не сгенерировался
		order.Fail(result.Error)
		account.Release()
		// Можно добавить логику ретрая на другом аккаунте через RequeueForRetry
	case domain.MusicGenerationStatusCompleted:
		// УСПЕХ!

		// Конвертируем треки провайдера в доменные треки
		var domainTracks []domain.Track
		for i, pt := range result.Tracks {
			domainTracks = append(domainTracks, domain.Track{
				ID:          uuid.New(),
				Index:       i + 1,
				AudioURL:    pt.SourceURL,
				DurationSec: pt.DurationSec,
				SunoTrackID: pt.ExternalID,
			})
		}

		if err := order.Complete(domainTracks); err != nil {
			return fmt.Errorf("завершение заказа в домене: %w", err)
		}

		// Списываем токены и освобождаем аккаунт
		_ = account.ConsumeTokens(1) // Допустим, 1 запрос = 1 токен
		account.ResetFailures()
		account.Release()
	}

	// Сохраняем финальные состояния в БД
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("сохранение финального заказа: %w", err)
	}
	if err := uc.accRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("освобождение аккаунта: %w", err)
	}

	uc.log.Info("цикл генерации завершен", "order_id", order.ID(), "status", result.Status)
	return nil
}

// GetOrder возвращает заказ по ID для отображения клиенту.
func (uc *OrderUseCase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения заказа: %w", err)
	}
	return order, nil
}
