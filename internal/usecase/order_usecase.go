package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/metrics"
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

// TransactionManager — порт Unit of Work: выполняет переданную функцию в рамках
// одной транзакции БД. Позволяет UseCase атомарно сохранять несколько независимых
// агрегатов (Order и SunoAccount), не возлагая на репозиторий одного агрегата
// знание о таблицах другого.
type TransactionManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type OrderUseCase struct {
	orderRepo orderRepository
	accRepo   accountRepository
	queue     queuePublisher
	provider  musicProvider
	storage   domain.TrackStorage
	notifier  notify.Notifier
	llmClient openai.APIClient
	promptUC  PromptBuilder
	pricing   Pricing
	tx        TransactionManager
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
	promptUC PromptBuilder,
	pricing Pricing,
	tx TransactionManager,
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
		promptUC:  promptUC,
		pricing:   pricing,
		tx:        tx,
		log:       log,
	}
}

// saveOrderAndAccount атомарно сохраняет заказ и аккаунт в одной транзакции
// (Unit of Work). Заменяет прежний OrderRepository.SaveWithAccount, который
// нарушал границы агрегатов, обновляя таблицу аккаунтов из репозитория заказов.
func (uc *OrderUseCase) saveOrderAndAccount(ctx context.Context, order *domain.Order, account *domain.SunoAccount) error {
	return uc.tx.Do(ctx, func(ctx context.Context) error {
		if err := uc.orderRepo.Update(ctx, order); err != nil {
			return fmt.Errorf("сохранение заказа: %w", err)
		}
		if err := uc.accRepo.Update(ctx, account); err != nil {
			return fmt.Errorf("сохранение аккаунта: %w", err)
		}
		return nil
	})
}

// ==========================================
// 1. ПОЛЬЗОВАТЕЛЬСКИЕ СЦЕНАРИИ (Вызываются из HTTP)
// ==========================================

// ErrInvalidEmail возвращается, если email передан, но имеет некорректный формат.
var ErrInvalidEmail = errors.New("некорректный формат email")

func (uc *OrderUseCase) CreateOrder(ctx context.Context, email, phone, brief, plan, categoryID string, answers map[string]string) (*domain.Order, error) {
	// Проверяем формат email, если он передан. Поле необязательное (можно оставить пустым),
	// но если передано — должно соответствовать RFC 5322 (net/mail).
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidEmail, err)
		}
	}

	// Цену определяет сервер по выбранному тарифу, а НЕ клиент. Иначе сумму заказа
	// можно занизить до 1 копейки и пройти сверку в вебхуке оплаты.
	amountKopecks, err := uc.pricing.PriceFor(plan)
	if err != nil {
		return nil, fmt.Errorf("определение цены тарифа: %w", err)
	}

	// InvoiceID берём из PostgreSQL sequence — атомарно и без коллизий
	// при параллельных запросах или нескольких инстансах сервиса.
	invoiceID, err := uc.orderRepo.NextInvoiceID(ctx)
	if err != nil {
		return nil, fmt.Errorf("получение invoice_id: %w", err)
	}

	// Если передана категория и ответы квиза — строим готовый Suno-промпт
	// из шаблона категории. Иначе sunoPrompt остаётся пустым, и воркер
	// использует сырой brief (обратная совместимость).
	var sunoPrompt string
	if categoryID != "" && len(answers) > 0 {
		sunoPrompt, err = uc.promptUC.BuildFinalPrompt(ctx, categoryID, answers)
		if err != nil {
			// Не фатально: категория могла быть удалена. Продолжаем с пустым промптом.
			uc.log.Warn("не удалось построить промпт по категории, используем brief",
				"category_id", categoryID, "err", err)
			sunoPrompt = ""
			categoryID = ""
		}
	}

	order, err := domain.NewOrder(invoiceID, email, phone, brief, categoryID, sunoPrompt, amountKopecks)
	if err != nil {
		return nil, fmt.Errorf("ошибка валидации заказа: %w", err)
	}

	if err := uc.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("ошибка сохранения заказа: %w", err)
	}

	metrics.OrdersCreated.Inc()
	uc.log.Info("создан новый заказ", "order_id", order.ID(), "invoice_id", invoiceID, "category_id", categoryID)
	return order, nil
}

func (uc *OrderUseCase) HandlePaymentSuccess(ctx context.Context, invoiceID int64, paidKopecks int64) error {
	order, err := uc.orderRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("поиск заказа по invoice_id: %w", err)
	}

	// Идемпотентность: Robokassa повторяет доставку ResultURL до получения OK{InvId}.
	// Повторный вебхук для уже оплаченного заказа — это НЕ ошибка: трактуем как успех,
	// иначе бесконечные ретраи Robokassa будут получать 500.
	if order.PaymentStatus() == domain.PaymentStatusPaid {
		uc.log.Info("повторная доставка вебхука для уже оплаченного заказа — идемпотентно ОК",
			"order_id", order.ID(), "invoice_id", invoiceID)
		return nil
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

	// Условный апдейт (WHERE payment_status='pending') защищает от гонки двух
	// параллельных доставок вебхука: только одна из них реально переведёт заказ
	// в paid+queued и поставит задачу. Остальные получат applied=false и выйдут
	// идемпотентно — без второй задачи генерации и двойного расхода кредитов Suno.
	applied, err := uc.orderRepo.ApplyPaymentSuccess(ctx, order)
	if err != nil {
		return fmt.Errorf("сохранение оплаты заказа: %w", err)
	}
	if !applied {
		uc.log.Info("оплата уже обработана параллельной доставкой — постановку задачи пропускаем",
			"order_id", order.ID(), "invoice_id", invoiceID)
		return nil
	}

	if err := uc.queue.EnqueueGenerationTask(ctx, order.ID()); err != nil {
		uc.log.Error("ошибка постановки задачи в очередь", "order_id", order.ID(), "err", err)
		return err
	}

	metrics.PaymentsReceived.Inc()
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
	metrics.ActiveWorkerSlots.Inc()

	// 3. Обогащение ТЗ через LLM.
	// Если заказ создан через квиз — sunoPrompt уже содержит готовый структурированный
	// промпт из шаблона категории. В этом случае LLM не нужен — передаём промпт напрямую в Suno.
	// Если заказ создан через "свободный" бриф — обогащаем через LLM как прежде.
	var lyrics string
	if order.SunoPrompt() != "" {
		lyrics = order.SunoPrompt()
		uc.log.Info("используем готовый промпт из квиза", "order_id", order.ID(), "category_id", order.CategoryID())
	} else {
		// Fallback на сырой Brief недопустим: бриф может быть до domain.MaxBriefLength
		// символов, а у Suno промпт ограничен сильнее — такой запрос провайдер отбросит,
		// и заказ всё равно упадёт. Поэтому при недоступности LLM мы НЕ генерируем
		// "как-нибудь", а освобождаем аккаунт и возвращаем заказ в очередь: Asynq
		// повторит задачу с backoff, пока LLM не поднимется.
		uc.log.Info("генерация текста песни через LLM", "order_id", order.ID())
		var err error
		lyrics, err = uc.llmClient.GenerateLyrics(ctx, order.Brief())
		if err != nil {
			metrics.LLMErrors.Inc()
			uc.log.Error("LLM недоступен — возвращаем заказ в очередь для повторной попытки",
				"order_id", order.ID(), "err", err)
			// LLM-сбой не вина аккаунта: НЕ инкрементируем failureCount, только
			// освобождаем слот и откатываем заказ в Queued.
			account.ReleaseSlot()
			metrics.ActiveWorkerSlots.Dec()
			order.RequeueForRetry()
			if saveErr := uc.saveOrderAndAccount(ctx, order, account); saveErr != nil {
				uc.log.Error("не удалось откатить заказ после ошибки LLM", "order_id", order.ID(), "err", saveErr)
			}
			return fmt.Errorf("генерация текста LLM недоступна: %w", err)
		}
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
		metrics.SunoAPIErrors.Inc()
		uc.log.Error("ошибка API Suno", "account", account.Email(), "err", err)
		// Порядок важен: сначала регистрируем ошибку (она может перевести аккаунт в Banned),
		// только потом Release — иначе Release выставит Active, а RegisterFailure затрёт его Banned.
		account.RegisterFailure(3)
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		order.RequeueForRetry()

		if saveErr := uc.saveOrderAndAccount(ctx, order, account); saveErr != nil {
			uc.log.Error("не удалось откатить заказ и аккаунт после ошибки Suno",
				"order_id", order.ID(), "account_id", account.ID(), "err", saveErr)
		}
		return fmt.Errorf("сбой провайдера: %w", err)
	}

	// 5. Атомарно сохраняем заказ (Processing) и состояние аккаунта в одной транзакции
	// (Unit of Work). Слот аккаунта остаётся занятым на всё время поллинга и
	// освобождается позже в CheckGenerationStatus (Complete/Fail) либо в
	// FailGeneration при исчерпании ретраев. Единая транзакция исключает
	// рассинхронизацию агрегатов.
	if err := uc.saveOrderAndAccount(ctx, order, account); err != nil {
		// Транзакция откатилась — слот аккаунта в БД не занят. Asynq сделает retry
		// задачи и снова попробует захватить аккаунт.
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
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
	case domain.MusicGenerationStatusCompleted:
		var domainTracks []domain.Track
		for i, pt := range result.Tracks {
			// Перезаливаем трек в собственное S3-хранилище. Временные ссылки Suno
			// протухают через несколько часов, поэтому загрузка ОБЯЗАТЕЛЬНА: при
			// ошибке S3 мы НЕ завершаем заказ временной ссылкой (через пару часов
			// она отдаст 403), а возвращаем ошибку — Asynq повторит задачу с
			// backoff. Заказ остаётся в processing, слот аккаунта не освобождается.
			// Повторная загрузка идемпотентна: тот же S3-ключ перезаписывается.
			s3Key := fmt.Sprintf("tracks/%s/%d.mp3", order.ID(), i+1)
			permanentURL, uploadErr := uc.storage.UploadFromURL(ctx, pt.SourceURL, s3Key, "audio/mpeg")
			if uploadErr != nil {
				uc.log.Error("ошибка загрузки трека в S3, задача будет повторена",
					"order_id", order.ID(), "track_index", i+1, "err", uploadErr)
				return fmt.Errorf("загрузка трека %d в постоянное хранилище: %w", i+1, uploadErr)
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

		metrics.OrdersCompleted.Inc()
		_ = account.ConsumeTokens(1)
		account.ResetFailures()
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
	}

	// Атомарно сохраняем финальный статус заказа и освобождаем слот аккаунта в
	// одной транзакции (Unit of Work). Два отдельных Update могли бы оставить
	// слот занятым навсегда, если первый прошёл, а второй упал.
	if err := uc.saveOrderAndAccount(ctx, order, account); err != nil {
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

// FailGeneration переводит заказ в окончательный отказ и освобождает захваченный
// аккаунт. Вызывается из терминального обработчика Asynq, когда у задачи исчерпаны
// все ретраи: иначе аккаунт навсегда застрянет в Busy, а заказ — в processing.
// Операция идемпотентна: для уже завершённого/упавшего заказа ничего не делает.
func (uc *OrderUseCase) FailGeneration(ctx context.Context, orderID uuid.UUID, reason string) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("заказ не найден: %w", err)
	}

	switch order.GenerationStatus() {
	case domain.GenerationStatusCompleted, domain.GenerationStatusFailed:
		// Терминальное состояние уже достигнуто — повторный вызов безвреден.
		return nil
	}

	if err := order.Fail(reason); err != nil {
		return fmt.Errorf("перевод заказа в failed: %w", err)
	}
	metrics.OrdersFailed.Inc()

	// Если аккаунт был захвачен — освобождаем его атомарно вместе с заказом,
	// чтобы он не остался Busy навсегда.
	if accountID := order.AssignedAccountID(); accountID != nil {
		account, err := uc.accRepo.GetByID(ctx, *accountID)
		if err != nil {
			uc.log.Error("не удалось загрузить аккаунт для освобождения при провале",
				"order_id", orderID, "account_id", *accountID, "err", err)
			// Аккаунт не нашли — сохраняем хотя бы статус заказа.
			if updErr := uc.orderRepo.Update(ctx, order); updErr != nil {
				return fmt.Errorf("сохранение упавшего заказа: %w", updErr)
			}
			return nil
		}
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		if err := uc.saveOrderAndAccount(ctx, order, account); err != nil {
			return fmt.Errorf("атомарное сохранение провала заказа и освобождения аккаунта: %w", err)
		}
		uc.log.Warn("заказ переведён в failed, аккаунт освобождён",
			"order_id", orderID, "account", account.Email(), "reason", reason)
		return nil
	}

	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("сохранение упавшего заказа: %w", err)
	}
	uc.log.Warn("заказ переведён в failed (аккаунт не был захвачен)", "order_id", orderID, "reason", reason)
	return nil
}

func (uc *OrderUseCase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения заказа: %w", err)
	}
	return order, nil
}

func (uc *OrderUseCase) ListOrdersByEmail(ctx context.Context, email string, limit, offset int) ([]*domain.Order, error) {
	if email == "" {
		return nil, fmt.Errorf("email не может быть пустым")
	}
	orders, err := uc.orderRepo.ListByCustomerEmail(ctx, email, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения заказов: %w", err)
	}
	return orders, nil
}

func (uc *OrderUseCase) ListOrdersByPhone(ctx context.Context, phone string, limit, offset int) ([]*domain.Order, error) {
	if phone == "" {
		return nil, fmt.Errorf("phone не может быть пустым")
	}
	orders, err := uc.orderRepo.ListByCustomerPhone(ctx, phone, limit, offset)
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
