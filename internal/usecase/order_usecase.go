package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/metrics"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/openai"
)

// quizLyricVariantSuffix — инструкция для Suno Inspiration Mode: второй запрос
// с тем же стилем, но другими русскими словами и припевом.
const quizLyricVariantSuffix = "\n\n[IMPORTANT: This is alternative version #2. Write completely different Russian lyrics and a different chorus. Keep the same theme, mood, and musical style as version #1, but do NOT repeat lines or phrases from version #1.]"

// Пользовательские ошибки слоя Use-Case
var (
	ErrGenerationNotReady = errors.New("генерация еще не завершена, требуется повторный опрос")
	// ErrPaymentAmountMismatch возникает, когда оплаченная по вебхуку сумма
	// не совпадает с суммой заказа. Защищает от подмены суммы при валидной подписи.
	ErrPaymentAmountMismatch = errors.New("оплаченная сумма не совпадает с суммой заказа")
	// ErrPaymentWindowExpired возникает, когда вебхук оплаты поступает спустя
	// слишком долгое время после создания заказа. Защищает от реплея старых
	// вебхуков с валидной подписью.
	ErrPaymentWindowExpired = errors.New("платёжное окно заказа истекло")
)

// maxPaymentWindow — максимальное время от создания заказа до принятия оплаты.
// Robokassa завершает оплату за секунды-минуты; 72 часа — щедрый запас для
// медленных банков и мобильного интернета, при этом отсекает месячные реплеи.
const maxPaymentWindow = 72 * time.Hour

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
	// priceKopecks — фиксированная серверная цена заказа (4 версии песни).
	// Клиент не может повлиять на сумму — иначе её можно занизить и пройти
	// сверку в вебхуке оплаты (см. CreateOrder).
	priceKopecks int64
	tx           TransactionManager
	log          *slog.Logger
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
	priceKopecks int64,
	tx TransactionManager,
	log *slog.Logger,
) *OrderUseCase {
	return &OrderUseCase{
		orderRepo:    orderRepo,
		accRepo:      accRepo,
		queue:        queue,
		provider:     provider,
		storage:      storage,
		notifier:     notifier,
		llmClient:    llmClient,
		promptUC:     promptUC,
		priceKopecks: priceKopecks,
		tx:           tx,
		log:          log,
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

// ErrInvalidPhone возвращается, если телефон передан, но имеет некорректный формат.
var ErrInvalidPhone = errors.New("некорректный формат телефона")

// phoneRe принимает российские и международные номера в форматах:
// +7XXXXXXXXXX, 8XXXXXXXXXX, +380XXXXXXXXX и т.п.
// Минимум 7 цифр, максимум 15 (E.164), допускаются пробелы, дефисы, скобки.
var phoneRe = regexp.MustCompile(`^\+?[\d\s\-\(\)]{7,20}$`)

func (uc *OrderUseCase) CreateOrder(ctx context.Context, email, phone, brief, categoryID string, answers map[string]string) (*domain.Order, error) {
	// Проверяем формат email, если он передан. Поле необязательное (можно оставить пустым),
	// но если передано — должно соответствовать RFC 5322 (net/mail).
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidEmail, err)
		}
	}
	// Проверяем формат телефона, если он передан.
	if phone != "" && !phoneRe.MatchString(phone) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPhone, phone)
	}

	// Цена фиксированная и определяется сервером (4 версии песни за один платёж,
	// без тарифов и подписок) — клиент не передаёт и не может повлиять на сумму.
	// Иначе её можно было бы занизить до 1 копейки и пройти сверку в вебхуке оплаты.
	amountKopecks := uc.priceKopecks

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

	// Проверка платёжного окна: отклоняем оплату заказов старше maxPaymentWindow.
	// Robokassa завершает оплату за минуты; сверхстарый вебхук с валидной подписью —
	// признак реплея захваченного запроса, а не легитимной повторной доставки.
	if age := time.Since(order.CreatedAt()); age > maxPaymentWindow {
		uc.log.Warn("вебхук отклонён: платёжное окно истекло",
			"order_id", order.ID(), "invoice_id", invoiceID,
			"order_age", age.Round(time.Minute))
		return ErrPaymentWindowExpired
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

	if err := uc.saveOrderAndAccount(ctx, order, account); err != nil {
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		return fmt.Errorf("сохранение старта обработки: %w", err)
	}

	// 3. Два разных текста песни: квиз — два промпта; свободный бриф — LLM.
	if order.SunoPrompt() == "" {
		order.UpdateGenerationProgress(domain.GenerationPhaseLyrics, 15, 0)
		if err := uc.orderRepo.Update(ctx, order); err != nil {
			uc.log.Warn("не удалось сохранить прогресс (lyrics)", "order_id", order.ID(), "err", err)
		}
	}
	briefs, err := uc.buildGenerationBriefs(ctx, order)
	if err != nil {
		metrics.LLMErrors.Inc()
		uc.log.Error("LLM недоступен — возвращаем заказ в очередь для повторной попытки",
			"order_id", order.ID(), "err", err)
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		_ = order.RequeueForRetry()
		if saveErr := uc.saveOrderAndAccount(ctx, order, account); saveErr != nil {
			uc.log.Error("не удалось откатить заказ после ошибки LLM", "order_id", order.ID(), "err", saveErr)
		}
		return fmt.Errorf("генерация текста LLM недоступна: %w", err)
	}
	uc.log.Info("подготовлены варианты текста для генерации", "order_id", order.ID(), "variants", len(briefs))

	order.UpdateGenerationProgress(domain.GenerationPhaseSubmitting, 25, 0)
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		uc.log.Warn("не удалось сохранить прогресс (submitting)", "order_id", order.ID(), "err", err)
	}

	// 4. Отправка структурированного запроса в Suno API.
	// Продукт фиксированный — 4 версии песни на заказ, без тарифов.
	req := domain.MusicGenerationRequest{
		Briefs:       briefs,
		Instrumental: false,
		TrackCount:   domain.DefaultTrackCount,
	}
	sunoJobID, err := uc.provider.SubmitGeneration(ctx, req)

	if err != nil {
		// Протухшая сессия — однозначный сигнал: аккаунт нужно немедленно вывести из
		// ротации без накопления счётчика ошибок. Заказ уходит обратно в очередь, чтобы
		// следующий свободный аккаунт подхватил его. Администратор обновляет сессию вручную.
		if errors.Is(err, domain.ErrProviderSessionExpired) {
			uc.log.Error("сессия Suno-аккаунта недействительна — аккаунт заблокирован до обновления сессии",
				"account_id", account.ID(), "email", account.Email())
			account.BanForInvalidSession()
			account.ReleaseSlot()
			metrics.ActiveWorkerSlots.Dec()
			_ = order.RequeueForRetry()
			if saveErr := uc.saveOrderAndAccount(ctx, order, account); saveErr != nil {
				uc.log.Error("не удалось сохранить состояние после обнаружения протухшей сессии",
					"order_id", order.ID(), "err", saveErr)
			}
			return fmt.Errorf("сессия провайдера недействительна: %w", err)
		}

		metrics.SunoAPIErrors.Inc()
		uc.log.Error("ошибка API Suno — аккаунт переведён в короткую паузу (cooldown), без блокировки",
			"account", account.Email(), "err", err, "cooldown", providerErrorCooldown)
		// Транзиентная ошибка провайдера (TTAPI 5xx, сеть, rate-limit) НЕ должна
		// банить аккаунт: при пуле из одного аккаунта это останавливало бы все
		// заказы. Ставим короткую самовосстанавливающуюся паузу — по её истечении
		// аккаунт сам вернётся в ротацию. Заказ откатываем в очередь; Asynq
		// повторит задачу с backoff. Перманентный бан остаётся только за 401
		// (ErrProviderSessionExpired выше) — это однозначный сигнал о невалидных
		// учётных данных, требующий вмешательства администратора.
		account.Throttle(providerErrorCooldown)
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		_ = order.RequeueForRetry()

		if saveErr := uc.saveOrderAndAccount(ctx, order, account); saveErr != nil {
			uc.log.Error("не удалось откатить заказ и аккаунт после ошибки Suno",
				"order_id", order.ID(), "account_id", account.ID(), "err", saveErr)
		}
		return fmt.Errorf("сбой провайдера: %w", err)
	}

	order.UpdateGenerationProgress(domain.GenerationPhaseGenerating, 30, 0)

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
		if result.ProgressPercent > 0 {
			order.UpdateGenerationProgress(domain.GenerationPhaseGenerating, result.ProgressPercent, result.ClipsReady)
			if err := uc.orderRepo.Update(ctx, order); err != nil {
				uc.log.Warn("не удалось сохранить прогресс генерации", "order_id", order.ID(), "err", err)
			}
		}
		return ErrGenerationNotReady
	}

	account, err := uc.accRepo.GetByID(ctx, *order.AssignedAccountID())
	if err != nil {
		return fmt.Errorf("ошибка загрузки аккаунта: %w", err)
	}

	switch result.Status {
	case domain.MusicGenerationStatusFailed:
		_ = order.Fail(result.Error)
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
	case domain.MusicGenerationStatusCompleted:
		var domainTracks []domain.Track
		totalTracks := len(result.Tracks)
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
			uploadPct := 85 + ((i+1)*14)/max(totalTracks, 1)
			order.UpdateGenerationProgress(domain.GenerationPhaseUploading, uploadPct, i+1)
			if err := uc.orderRepo.Update(ctx, order); err != nil {
				uc.log.Warn("не удалось сохранить прогресс загрузки", "order_id", order.ID(), "err", err)
			}
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
			AccessToken: order.AccessToken(),
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

// ==========================================
// 3. ФОНОВОЕ ОБСЛУЖИВАНИЕ (Вызывается по расписанию)
// ==========================================

// stuckOrderThreshold — время, после которого заказ в статусе processing считается
// застрявшим. 15 минут покрывают максимальный таймаут одной задачи (90с generate +
// 3мин status check) с запасом на медленный S3 и LLM.
const stuckOrderThreshold = 15 * time.Minute

// stuckQueuedThreshold — время, после которого ОПЛАЧЕННЫЙ заказ в статусе queued
// считается «осиротевшим»: задача генерации потеряна или исчерпала ретраи. Берём
// меньше processing-порога: в норме воркер забирает задачу за секунды, поэтому
// 5 минут простоя в queued — уже аномалия, требующая повторной постановки задачи.
const stuckQueuedThreshold = 5 * time.Minute

// providerErrorCooldown — длительность самовосстанавливающейся паузы аккаунта
// после транзиентной ошибки провайдера. Достаточно короткая, чтобы единственный
// аккаунт быстро вернулся в ротацию, и достаточно длинная, чтобы не молотить
// упавший TTAPI в цикле.
const providerErrorCooldown = 60 * time.Second

// RecoverStuckOrders находит заказы, застрявшие в processing после краша пода/воркера,
// и переводит их обратно в queued, освобождая захваченный слот аккаунта.
// Операция идемпотентна: повторный вызов для уже восстановленного заказа ничего не сделает.
func (uc *OrderUseCase) RecoverStuckOrders(ctx context.Context) error {
	threshold := time.Now().UTC().Add(-stuckOrderThreshold)
	orders, err := uc.orderRepo.ListStuckProcessing(ctx, threshold)
	if err != nil {
		return fmt.Errorf("поиск застрявших заказов: %w", err)
	}
	if len(orders) == 0 {
		return nil
	}

	uc.log.Warn("обнаружены застрявшие заказы, инициируем восстановление", "count", len(orders))
	for _, order := range orders {
		if err := uc.recoverStuckOrder(ctx, order); err != nil {
			uc.log.Error("не удалось восстановить застрявший заказ",
				"order_id", order.ID(), "err", err)
			// Продолжаем с остальными — частичное восстановление лучше полного простоя.
		}
	}
	return nil
}

// RecoverOrphanedQueuedOrders повторно ставит задачу генерации для оплаченных
// заказов, надолго застрявших в статусе queued: их Asynq-задача была потеряна
// (краш Redis/воркера) или исчерпала ретраи и ушла в archived. Без этого заказ
// навсегда висит в «В очереди». Повторная постановка идемпотентна (TaskID), так
// что для заказа с ещё живой задачей это no-op.
func (uc *OrderUseCase) RecoverOrphanedQueuedOrders(ctx context.Context) error {
	threshold := time.Now().UTC().Add(-stuckQueuedThreshold)
	orders, err := uc.orderRepo.ListStuckQueued(ctx, threshold)
	if err != nil {
		return fmt.Errorf("поиск осиротевших queued-заказов: %w", err)
	}
	if len(orders) == 0 {
		return nil
	}

	uc.log.Warn("обнаружены осиротевшие заказы в очереди, повторно ставим задачу генерации", "count", len(orders))
	for _, order := range orders {
		if err := uc.queue.EnqueueGenerationTask(ctx, order.ID()); err != nil {
			uc.log.Error("не удалось повторно поставить задачу для осиротевшего заказа",
				"order_id", order.ID(), "err", err)
			continue
		}
		uc.log.Info("осиротевший queued-заказ повторно поставлен в очередь", "order_id", order.ID())
	}
	return nil
}

func (uc *OrderUseCase) recoverStuckOrder(ctx context.Context, order *domain.Order) error {
	// Сохраняем ID до вызова RequeueForRetry, который очищает assignedAccountID.
	accountID := order.AssignedAccountID()

	if err := order.RequeueForRetry(); err != nil {
		return fmt.Errorf("перевод застрявшего заказа в queued: %w", err)
	}
	metrics.ActiveWorkerSlots.Dec()

	// 1. Сохраняем заказ (queued) и освобождаем слот захваченного аккаунта.
	if accountID != nil {
		account, err := uc.accRepo.GetByID(ctx, *accountID)
		if err != nil {
			uc.log.Error("аккаунт не найден при recovery — сохраняем только заказ",
				"order_id", order.ID(), "account_id", *accountID, "err", err)
			if err := uc.orderRepo.Update(ctx, order); err != nil {
				return fmt.Errorf("сохранение восстановленного заказа: %w", err)
			}
		} else {
			account.ReleaseSlot()
			if err := uc.saveOrderAndAccount(ctx, order, account); err != nil {
				return fmt.Errorf("атомарное сохранение при recovery: %w", err)
			}
		}
	} else if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("сохранение восстановленного заказа: %w", err)
	}

	// 2. КЛЮЧЕВОЕ: заново ставим задачу генерации. RecoverStuckOrders только меняет
	// статус в БД на queued; исходная Asynq-задача к этому моменту уже завершилась
	// (повторный прогон пропускался по статусу) или потеряна при краше. Без новой
	// задачи заказ навсегда остался бы в queued. Постановка идемпотентна (TaskID),
	// а ListStuckProcessing после смены статуса этот заказ уже не вернёт — поэтому
	// если enqueue упадёт, возвращаем ошибку, чтобы это попало в логи/метрики.
	if err := uc.queue.EnqueueGenerationTask(ctx, order.ID()); err != nil {
		return fmt.Errorf("повторная постановка задачи генерации при recovery: %w", err)
	}

	uc.log.Info("застрявший заказ восстановлен и повторно поставлен в очередь", "order_id", order.ID())
	return nil
}

func (uc *OrderUseCase) buildGenerationBriefs(ctx context.Context, order *domain.Order) ([]string, error) {
	if order.SunoPrompt() != "" {
		prompt := order.SunoPrompt()
		uc.log.Info("используем готовый промпт из квиза", "order_id", order.ID(), "category_id", order.CategoryID())
		return []string{prompt, prompt + quizLyricVariantSuffix}, nil
	}

	uc.log.Info("генерация двух вариантов текста через LLM", "order_id", order.ID())
	briefs, err := uc.llmClient.GenerateLyricsVariants(ctx, order.Brief(), domain.DefaultLyricVariantCount)
	if err != nil {
		return nil, err
	}
	if len(briefs) < domain.DefaultLyricVariantCount {
		return nil, fmt.Errorf("LLM вернул %d вариантов, ожидали %d", len(briefs), domain.DefaultLyricVariantCount)
	}
	return briefs[:domain.DefaultLyricVariantCount], nil
}
