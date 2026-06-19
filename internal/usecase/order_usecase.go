package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/openai"
)

// Пользовательские ошибки слоя Use-Case
var (
	ErrGenerationNotReady = errors.New("генерация еще не завершена, требуется повторный опрос")
	// ErrPaymentAmountMismatch возникает, когда оплаченная по вебхуку сумма
	// не совпадает с суммой заказа. Защищает от подмены суммы при валидной подписи.
	ErrPaymentAmountMismatch = errors.New("оплаченная сумма не совпадает с суммой заказа")
)

type OrderUseCase struct {
	orderRepo orderRepository
	accRepo   accountRepository
	queue     queuePublisher
	provider  musicProvider
	storage   domain.TrackStorage
	notifier  notify.Notifier
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
	storage domain.TrackStorage,
	notifier notify.Notifier,
	llmClient openai.APIClient,
	log *slog.Logger,
) *OrderUseCase {
	return &OrderUseCase{
		orderRepo: orderRepo,
		accRepo:   accRepo,
		queue:     queue,
		provider:  provider,
		storage:   storage,
		notifier:  notifier,
		llmClient: llmClient,
		log:       log,
	}
}

// ==========================================
// 1. ПОЛЬЗОВАТЕЛЬСКИЕ СЦЕНАРИИ (Вызываются из HTTP)
// ==========================================

func (uc *OrderUseCase) CreateOrder(ctx context.Context, email, phone, brief string, amountKopecks int64) (*domain.Order, error) {
	// InvoiceID берём из PostgreSQL sequence — атомарно и без коллизий
	// при параллельных запросах или нескольких инстансах сервиса.
	invoiceID, err := uc.orderRepo.NextInvoiceID(ctx)
	if err != nil {
		return nil, fmt.Errorf("получение invoice_id: %w", err)
	}

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

func (uc *OrderUseCase) HandlePaymentSuccess(ctx context.Context, invoiceID int64, paidKopecks int64) error {
	order, err := uc.orderRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("поиск заказа по invoice_id: %w", err)
	}

	// Подпись вебхука уже проверена в слое доставки, но это не гарантирует,
	// что оплачена именно сумма заказа. Сверяем явно, чтобы исключить подмену суммы.
	if paidKopecks != order.AmountKopecks() {
		uc.log.Warn("сумма оплаты не совпадает с суммой заказа",
			"order_id", order.ID(), "invoice_id", invoiceID,
			"expected_kopecks", order.AmountKopecks(), "paid_kopecks", paidKopecks)
		return ErrPaymentAmountMismatch
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
		// Порядок важен: сначала регистрируем ошибку (она может перевести аккаунт в Banned),
		// только потом Release — иначе Release выставит Active, а RegisterFailure затрёт его Banned.
		account.RegisterFailure(3)
		account.Release()
		order.RequeueForRetry()

		if saveErr := uc.accRepo.Update(ctx, account); saveErr != nil {
			uc.log.Error("не удалось сохранить состояние аккаунта после ошибки Suno", "account_id", account.ID(), "err", saveErr)
		}
		if saveErr := uc.orderRepo.Update(ctx, order); saveErr != nil {
			uc.log.Error("не удалось откатить заказ в Queued после ошибки Suno", "order_id", order.ID(), "err", saveErr)
		}
		return fmt.Errorf("сбой провайдера: %w", err)
	}

	// 5. Атомарно сохраняем заказ (Processing + sunoJobID) и освобождаем аккаунт.
	// Важно: если сохранить только заказ и упасть — аккаунт навсегда останется в Busy.
	// Единственная транзакция гарантирует консистентность обоих агрегатов.
	if err := uc.orderRepo.SaveWithAccount(ctx, order, account); err != nil {
		// Аккаунт в БД всё ещё Busy, а транзакция откатилась — аккаунт не утёк.
		// Asynq сделает retry задачи и снова попробует захватить аккаунт.
		return fmt.Errorf("атомарное сохранение заказа и аккаунта: %w", err)
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
			// Перезаливаем трек в собственное S3-хранилище.
			// Временные ссылки Suno протухают через несколько часов —
			// клиент не сможет скачать трек позже без этого шага.
			s3Key := fmt.Sprintf("tracks/%s/%d.mp3", order.ID(), i+1)
			permanentURL, uploadErr := uc.storage.UploadFromURL(ctx, pt.SourceURL, s3Key, "audio/mpeg")
			if uploadErr != nil {
				// Падение загрузки в S3 не должно ронять весь цикл:
				// сохраняем исходную ссылку Suno как fallback и логируем.
				uc.log.Error("не удалось загрузить трек в S3, используем временную ссылку",
					"order_id", order.ID(), "track_index", i+1, "err", uploadErr)
				permanentURL = pt.SourceURL
			}
			domainTracks = append(domainTracks, domain.Track{
				ID:          uuid.New(),
				Index:       i + 1,
				AudioURL:    permanentURL,
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

	// Атомарно сохраняем финальный статус заказа и освобождаем аккаунт.
	// Два отдельных Update создали бы тот же Busy-leak что и в ProcessGenerationTask:
	// если Update заказа прошёл, а Update аккаунта упал — аккаунт застрянет в Busy навсегда.
	if err := uc.orderRepo.SaveWithAccount(ctx, order, account); err != nil {
		return fmt.Errorf("атомарное сохранение финального статуса: %w", err)
	}

	// Уведомляем клиента о готовности. Ошибка уведомления не роняет задачу —
	// треки уже сохранены и доступны через API.
	if result.Status == domain.MusicGenerationStatusCompleted {
		var trackURLs []string
		for _, t := range order.Tracks() {
			trackURLs = append(trackURLs, t.AudioURL)
		}
		if notifyErr := uc.notifier.NotifyOrderComplete(ctx, notify.OrderCompleteNotification{
			OrderID:     order.ID().String(),
			Email:       order.CustomerEmail(),
			Phone:       order.CustomerPhone(),
			TrackURLs:   trackURLs,
			TracksCount: len(trackURLs),
		}); notifyErr != nil {
			uc.log.Error("ошибка отправки уведомления", "order_id", order.ID(), "err", notifyErr)
		}
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

func (uc *OrderUseCase) ListOrdersByEmail(ctx context.Context, email string) ([]*domain.Order, error) {
	if email == "" {
		return nil, fmt.Errorf("email не может быть пустым")
	}
	orders, err := uc.orderRepo.ListByCustomerEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения заказов: %w", err)
	}
	return orders, nil
}

func (uc *OrderUseCase) ListOrdersByPhone(ctx context.Context, phone string) ([]*domain.Order, error) {
	if phone == "" {
		return nil, fmt.Errorf("phone не может быть пустым")
	}
	orders, err := uc.orderRepo.ListByCustomerPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения заказов по телефону: %w", err)
	}
	return orders, nil
}

func (uc *OrderUseCase) GetOrderByToken(ctx context.Context, token string) (*domain.Order, error) {
	if token == "" {
		return nil, domain.ErrOrderUnauthorized
	}
	order, err := uc.orderRepo.GetByAccessToken(ctx, token)
	if err != nil {
		return nil, domain.ErrOrderUnauthorized
	}
	return order, nil
}
