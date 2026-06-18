package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/openai"
)

// Пользовательские ошибки слоя Use-Case
var (
	ErrGenerationNotReady = errors.New("генерация еще не завершена, требуется повторный опрос")
)

type OrderUseCase struct {
	orderRepo orderRepository
	accRepo   accountRepository
	queue     queuePublisher
	provider  musicProvider
	llmClient openai.APIClient
	log       *slog.Logger
}

// Алиасы для интерфейсов, чтобы сократить код (или используйте напрямую domain.*)
type orderRepository = domain.OrderRepository
type accountRepository = domain.AccountRepository
type queuePublisher = domain.QueuePublisher
type musicProvider = domain.MusicProvider

func NewOrderUseCase(
	orderRepo domain.OrderRepository,
	accRepo domain.AccountRepository,
	queue domain.QueuePublisher,
	provider domain.MusicProvider,
	llmClient openai.APIClient,
	log *slog.Logger,
) *OrderUseCase {
	return &OrderUseCase{
		orderRepo: orderRepo,
		accRepo:   accRepo,
		queue:     queue,
		provider:  provider,
		llmClient: llmClient,
		log:       log,
	}
}

// ==========================================
// 1. ПОЛЬЗОВАТЕЛЬСКИЕ СЦЕНАРИИ (Вызываются из HTTP)
// ==========================================

func (uc *OrderUseCase) CreateOrder(ctx context.Context, email, phone, brief string, amountKopecks int64) (*domain.Order, error) {
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

func (uc *OrderUseCase) HandlePaymentSuccess(ctx context.Context, invoiceID int64) error {
	order, err := uc.orderRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("поиск заказа по invoice_id: %w", err)
	}

	if err := order.MarkPaid(); err != nil {
		return fmt.Errorf("переход статуса оплаты: %w", err)
	}

	if err := order.Enqueue(); err != nil {
		return fmt.Errorf("переход статуса генерации: %w", err)
	}

	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("обновление заказа: %w", err)
	}

	if err := uc.queue.EnqueueGenerationTask(ctx, order.ID()); err != nil {
		uc.log.Error("ошибка постановки задачи в очередь", "order_id", order.ID(), "err", err)
		return err
	}

	uc.log.Info("заказ успешно оплачен и поставлен в очередь", "order_id", order.ID())
	return nil
}

// ==========================================
// 2. ФОНОВЫЕ СЦЕНАРИИ (Вызываются воркерами Asynq)
// ==========================================

func (uc *OrderUseCase) ProcessGenerationTask(ctx context.Context, orderID uuid.UUID) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("заказ не найден: %w", err)
	}

	if order.GenerationStatus() != domain.GenerationStatusQueued {
		uc.log.Warn("задача генерации пропущена (неверный статус)", "order_id", order.ID(), "status", order.GenerationStatus())
		return nil
	}

	// 1. Атомарно захватываем аккаунт
	account, err := uc.accRepo.FetchAndLockAvailable(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNoAvailableAccount) {
			uc.log.Warn("нет свободных аккаунтов, задача будет отложена (retry)", "order_id", orderID)
			return err
		}
		return fmt.Errorf("ошибка захвата аккаунта: %w", err)
	}

	// 2. Привязываем аккаунт к заказу
	if err := order.StartProcessing(account.ID()); err != nil {
		return fmt.Errorf("недопустимый статус для старта: %w", err)
	}

	// 3. Обогащение ТЗ через LLM
	uc.log.Info("генерация текста песни через LLM", "order_id", order.ID())
	lyrics, err := uc.llmClient.GenerateLyrics(ctx, order.Brief())
	if err != nil {
		uc.log.Error("ошибка генерации текста, используется fallback к исходному брифу", "err", err)
		lyrics = order.Brief()
	} else {
		uc.log.Info("текст успешно сгенерирован")
	}

	// 4. Отправка структурированного запроса в Suno API
	req := domain.MusicGenerationRequest{
		Brief:        lyrics,
		Instrumental: false,
		TrackCount:   4,
	}
	sunoJobID, err := uc.provider.SubmitGeneration(ctx, req)

	if err != nil {
		uc.log.Error("ошибка API Suno", "account", account.Email(), "err", err)
		account.Release()
		account.RegisterFailure(3)
		order.RequeueForRetry()

		_ = uc.accRepo.Update(ctx, account)
		_ = uc.orderRepo.Update(ctx, order)
		return fmt.Errorf("сбой провайдера: %w", err)
	}

	// 5. Фиксация состояния и инициация поллинга
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return err
	}

	if err := uc.queue.EnqueueStatusCheckTask(ctx, order.ID(), sunoJobID); err != nil {
		return err
	}

	uc.log.Info("генерация успешно запущена", "order_id", order.ID(), "suno_job", sunoJobID, "account", account.Email())
	return nil
}

func (uc *OrderUseCase) CheckGenerationStatus(ctx context.Context, orderID uuid.UUID, providerJobID string) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}

	if order.GenerationStatus() != domain.GenerationStatusProcessing {
		return nil
	}

	result, err := uc.provider.FetchResult(ctx, providerJobID)
	if err != nil {
		return fmt.Errorf("ошибка опроса статуса: %w", err)
	}

	if result.Status == domain.MusicGenerationStatusPending || result.Status == domain.MusicGenerationStatusRunning {
		return ErrGenerationNotReady
	}

	account, err := uc.accRepo.GetByID(ctx, *order.AssignedAccountID())
	if err != nil {
		return fmt.Errorf("ошибка загрузки аккаунта: %w", err)
	}

	switch result.Status {
	case domain.MusicGenerationStatusFailed:
		order.Fail(result.Error)
		account.Release()
	case domain.MusicGenerationStatusCompleted:
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

		_ = account.ConsumeTokens(1)
		account.ResetFailures()
		account.Release()
	}

	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("сохранение финального заказа: %w", err)
	}
	if err := uc.accRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("освобождение аккаунта: %w", err)
	}

	uc.log.Info("цикл генерации завершен", "order_id", order.ID(), "status", result.Status)
	return nil
}

func (uc *OrderUseCase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения заказа: %w", err)
	}
	return order, nil
}
