package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/metrics"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/openai"
	"github.com/numaestra/numaestra/pkg/suno"
)

// quizLyricVariantSuffix — инструкция для Suno Inspiration Mode: второй запрос
// с тем же стилем, но другими русскими словами и припевом.
const quizLyricVariantSuffix = "\n\n[VERSION 2: Write completely different Russian lyrics and a different chorus. Keep the same story, mood, genre, vocals, and tempo as version 1. Do not reuse lines or phrases from version 1.]"

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

// PaymentVerifier — порт проверки фактического статуса платежа в Robokassa
// (OpStateExt). Реализуется *robokassa.Client. nil → фоновая сверка платежей
// отключена (например, в окружениях без боевой Robokassa).
type PaymentVerifier interface {
	GetPaidAmountKopecks(ctx context.Context, invID int64) (kopecks int64, paid bool, err error)
}

// DemoLimiter — порт защиты расхода кредитов на бесплатные демо: дневной лимит и
// лимит «одно демо на email». Reserve вызывается, когда демо уже прошло проверку
// токенового резерва и вот-вот стартует. Идемпотентен по orderID — повторная
// демо-задача (retry Asynq) не тратит лимит повторно. nil → лимиты отключены.
type DemoLimiter interface {
	Reserve(ctx context.Context, orderID uuid.UUID, email string) (allowed bool, err error)
	// AllowIP ограничивает число демо с одного IP в сутки. Проверяется при триггере
	// демо (на HTTP-слое, где известен IP), чтобы один источник не выжег дневной бюджет.
	AllowIP(ctx context.Context, ip string) (allowed bool, err error)
}

// DemoClipProcessor — порт обработки демо-фрагмента (Фаза 2): выбирает «сочный»
// участок, обрезает и накладывает водяной знак, возвращая готовый mp3.
// nil → обработка выключена (демо = полный клип, Фаза 1). Ошибка процессора
// трактуется как сигнал деградировать до полного клипа.
type DemoClipProcessor interface {
	Process(ctx context.Context, sourceURL string) ([]byte, error)
}

type OrderUseCase struct {
	orderRepo orderRepository
	accRepo   accountRepository
	promoRepo domain.PromoCodeRepository // nil если не подключён
	queue     queuePublisher
	provider  musicProvider
	storage   domain.TrackStorage
	notifier  notify.Notifier
	llmClient openai.APIClient
	promptUC  PromptBuilder
	// priceKopecks — фиксированная серверная цена заказа (4 версии песни).
	// Клиент не может повлиять на сумму — иначе её можно занизить и пройти
	// сверку в вебхуке оплаты (см. CreateOrder).
	priceKopecks     int64
	tx               TransactionManager
	paymentVerifier  PaymentVerifier   // nil → сверка платежей отключена
	demoLimiter      DemoLimiter       // nil → лимиты демо отключены
	demoTokenReserve int               // 0 → резерв токенов под платные отключён
	demoClip         DemoClipProcessor // nil → демо = полный клип (Фаза 1)
	log              *slog.Logger
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

// WithPromoRepo подключает репозиторий промокодов для применения скидок при создании заказа.
func (uc *OrderUseCase) WithPromoRepo(repo domain.PromoCodeRepository) *OrderUseCase {
	uc.promoRepo = repo
	return uc
}

// WithPaymentVerifier подключает проверку платежей Robokassa для фоновой сверки
// pending-заказов (ReconcilePendingPayments). Без него сверка отключена.
func (uc *OrderUseCase) WithPaymentVerifier(v PaymentVerifier) *OrderUseCase {
	uc.paymentVerifier = v
	return uc
}

// WithDemoGuards подключает защиту расхода кредитов на демо: лимитер (дневной/email)
// и резерв токенов под платные заказы. limiter может быть nil (тогда лимиты
// отключены), tokenReserve=0 отключает резерв.
func (uc *OrderUseCase) WithDemoGuards(limiter DemoLimiter, tokenReserve int) *OrderUseCase {
	uc.demoLimiter = limiter
	uc.demoTokenReserve = tokenReserve
	return uc
}

// WithDemoClip подключает обработку демо-фрагмента (Фаза 2): обрезка «сочного»
// участка + водяной знак. nil → демо отдаётся полным клипом (Фаза 1).
func (uc *OrderUseCase) WithDemoClip(p DemoClipProcessor) *OrderUseCase {
	uc.demoClip = p
	return uc
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

func (uc *OrderUseCase) CreateOrder(ctx context.Context, email, phone, brief, categoryID, consentDocVersion, promoCode, referralCode string, answers map[string]string) (*domain.Order, error) {
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

	// Если передана категория — проверяем квиз и строим Suno-промпт на сервере.
	var sunoPrompt string
	if categoryID != "" {
		cat, err := uc.promptUC.GetCategoryWizard(ctx, categoryID)
		if err != nil {
			if errors.Is(err, domain.ErrCategoryNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("загрузка категории: %w", err)
		}
		if err := domain.ValidateQuizAnswers(cat.Questions(), answers); err != nil {
			return nil, err
		}
		sunoPrompt, err = uc.promptUC.BuildFinalPrompt(ctx, categoryID, answers)
		if err != nil {
			return nil, fmt.Errorf("построение промпта: %w", err)
		}
		if utf8.RuneCountInString(sunoPrompt) > domain.MaxSunoPromptLength {
			return nil, domain.ErrPromptTooLong
		}
	}

	// Промокод ищем заранее, но применяем к агрегату после его создания —
	// ApplyPromo сохраняет originalAmountKopecks = полная цена до скидки.
	var appliedPromo *domain.PromoCode
	if promoCode != "" && uc.promoRepo != nil {
		promo, err := uc.promoRepo.GetByCode(ctx, strings.ToUpper(promoCode))
		if err != nil {
			if errors.Is(err, domain.ErrPromoCodeNotFound) {
				return nil, domain.ErrPromoCodeNotFound
			}
			return nil, fmt.Errorf("проверка промокода: %w", err)
		}
		if !promo.IsValid() {
			return nil, domain.ErrPromoCodeInvalid
		}
		appliedPromo = promo
	}

	order, err := domain.NewOrder(invoiceID, email, phone, brief, categoryID, sunoPrompt, consentDocVersion, amountKopecks)
	if err != nil {
		return nil, fmt.Errorf("ошибка валидации заказа: %w", err)
	}

	// ApplyPromo устанавливает originalAmountKopecks = текущий amountKopecks (полная цена)
	// и уменьшает amountKopecks на размер скидки — итоговая сумма к оплате.
	if appliedPromo != nil {
		discountKopecks := appliedPromo.Apply(amountKopecks)
		order.ApplyPromo(appliedPromo.ID(), discountKopecks)
	}
	if referralCode != "" {
		order.SetReferralCode(strings.TrimSpace(referralCode))
	}

	// Атомарно: сначала IncrementUses (SQL WHERE current_uses < max_uses), затем Create.
	// Порядок критичен: если бы мы сначала Create, а потом IncrementUses, то при гонке
	// два конкурентных запроса оба успели бы создать заказ со скидкой, а IncrementUses
	// одного из них вернул бы false уже после сохранения заказа. При текущем порядке
	// только выигравший гонку создаёт заказ; проигравший получает ErrPromoCodeInvalid.
	if err := uc.tx.Do(ctx, func(ctx context.Context) error {
		if appliedPromo != nil {
			ok, err := uc.promoRepo.IncrementUses(ctx, appliedPromo.ID())
			if err != nil {
				return fmt.Errorf("увеличение счётчика промокода: %w", err)
			}
			if !ok {
				return domain.ErrPromoCodeInvalid
			}
		}
		return uc.orderRepo.Create(ctx, order)
	}); err != nil {
		if errors.Is(err, domain.ErrPromoCodeInvalid) {
			return nil, domain.ErrPromoCodeInvalid
		}
		return nil, fmt.Errorf("ошибка сохранения заказа: %w", err)
	}

	metrics.OrdersCreated.Inc()
	uc.log.Info("создан новый заказ", "order_id", order.ID(), "invoice_id", invoiceID,
		"category_id", categoryID, "promo_code", promoCode, "referral_code", referralCode)
	return order, nil
}

func (uc *OrderUseCase) HandlePaymentSuccess(ctx context.Context, invoiceID int64, paidKopecks int64) error {
	order, err := uc.orderRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("поиск заказа по invoice_id: %w", err)
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

	return uc.applyPaymentSuccess(ctx, order, "webhook")
}

// AdminConfirmPayment помечает заказ оплаченным без вебхука Robokassa.
// Используется, когда деньги списаны, но ResultURL не дошёл до приложения.
// Перед вызовом админ должен убедиться, что платёж есть в кабинете Robokassa.
func (uc *OrderUseCase) AdminConfirmPayment(ctx context.Context, orderID uuid.UUID) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("получение заказа: %w", err)
	}
	if order.PaymentStatus() != domain.PaymentStatusPending {
		if order.PaymentStatus() == domain.PaymentStatusPaid {
			return nil
		}
		return fmt.Errorf("подтверждение оплаты недоступно: статус %q", order.PaymentStatus())
	}
	uc.log.Warn("admin: ручное подтверждение оплаты",
		"order_id", orderID, "invoice_id", order.InvoiceID())
	return uc.applyPaymentSuccess(ctx, order, "admin")
}

func (uc *OrderUseCase) applyPaymentSuccess(ctx context.Context, order *domain.Order, source string) error {
	invoiceID := order.InvoiceID()

	// Идемпотентность: Robokassa повторяет доставку ResultURL до получения OK{InvId}.
	// Повторный вебхук для уже оплаченного заказа — это НЕ ошибка: трактуем как успех,
	// иначе бесконечные ретраи Robokassa будут получать 500.
	if order.PaymentStatus() == domain.PaymentStatusPaid {
		// Страховочная постановка задачи: если первый вебхук успел сохранить
		// paid+queued в БД, но упал до EnqueueGenerationTask, повторный вебхук
		// доходит сюда и должен поставить задачу вместо него.
		// EnqueueGenerationTask идемпотентна через TaskID, поэтому двойная
		// постановка для уже живой задачи безопасна (ErrTaskIDConflict → success).
		if order.GenerationStatus() == domain.GenerationStatusQueued {
			if enqErr := uc.queue.EnqueueGenerationTask(ctx, order.ID()); enqErr != nil {
				uc.log.Warn("страховочная постановка задачи не удалась — заказ подхватит cron",
					"order_id", order.ID(), "invoice_id", invoiceID, "err", enqErr)
			}
		}
		uc.log.Info("оплата уже подтверждена — идемпотентно ОК",
			"order_id", order.ID(), "invoice_id", invoiceID, "source", source)
		return nil
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
			"order_id", order.ID(), "invoice_id", invoiceID, "source", source)
		return nil
	}

	if err := uc.queue.EnqueueGenerationTask(ctx, order.ID()); err != nil {
		uc.log.Error("ошибка постановки задачи в очередь", "order_id", order.ID(), "err", err)
		return err
	}

	metrics.PaymentsReceived.Inc()
	uc.notifyAdminAsync(notify.AdminEventNotification{
		Kind:          notify.AdminEventPaidOrder,
		OrderID:       order.ID().String(),
		InvoiceID:     order.InvoiceID(),
		CustomerEmail: order.CustomerEmail(),
		CustomerPhone: order.CustomerPhone(),
		AmountKopecks: order.AmountKopecks(),
		Brief:         order.Brief(),
	})
	uc.log.Info("заказ успешно оплачен и поставлен в очередь",
		"order_id", order.ID(), "invoice_id", invoiceID, "source", source)
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
		// Слот уже захвачен в БД FetchAndLockAvailable — без освобождения он
		// остался бы занятым навсегда (аккаунт пропал бы из ротации без задачи).
		account.ReleaseSlot()
		if releaseErr := uc.accRepo.Update(ctx, account); releaseErr != nil {
			uc.log.Error("не удалось освободить слот аккаунта после ошибки старта обработки",
				"order_id", order.ID(), "account_id", account.ID(), "err", releaseErr)
		}
		return fmt.Errorf("недопустимый статус для старта: %w", err)
	}
	metrics.ActiveWorkerSlots.Inc()

	if err := uc.saveOrderAndAccount(ctx, order, account); err != nil {
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		return fmt.Errorf("сохранение старта обработки: %w", err)
	}

	// 3. Два варианта промпта для Suno Inspiration Mode (Suno сам пишет текст).
	briefs, err := uc.buildGenerationBriefs(ctx, order)
	if err != nil {
		// Аккаунт уже помечен busy в БД (saveOrderAndAccount выше). Без явного
		// отката он застрянет в Busy навсегда — освобождаем слот и возвращаем
		// заказ в queued, чтобы Asynq повторил задачу с другим аккаунтом.
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		_ = order.RequeueForRetry()
		if saveErr := uc.saveOrderAndAccount(ctx, order, account); saveErr != nil {
			uc.log.Error("не удалось откатить состояние после ошибки промпта",
				"order_id", order.ID(), "account_id", account.ID(), "err", saveErr)
		}
		return fmt.Errorf("подготовка промптов для Suno: %w", err)
	}
	uc.log.Info("подготовлены варианты промпта для Suno", "order_id", order.ID(), "variants", len(briefs))

	order.UpdateGenerationProgress(domain.GenerationPhaseSubmitting, 25, 0)
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		uc.log.Warn("не удалось сохранить прогресс (submitting)", "order_id", order.ID(), "err", err)
	}

	// 4. Отправка структурированного запроса в Suno API.
	// Продукт фиксированный — 4 версии песни на заказ, без тарифов.
	styleTags := resolveSunoStyleTags(order.SunoPrompt(), order.Brief())

	// Переиспользование демо: если демо готово и его полные клипы сохранены, они уже
	// станут версиями 1–2 (трек №1 = ровно то демо, что слушал клиент). Догенерируем
	// только недостающие версии текстом варианта 2 (отличным от демо) — это экономит
	// одну Suno-задачу на конверсию. Без сохранённого демо — полная генерация, как раньше.
	genBriefs := briefs
	trackCount := domain.DefaultTrackCount
	if demoCount := len(order.DemoClips()); demoCount > 0 && demoCount < domain.DefaultTrackCount {
		trackCount = domain.DefaultTrackCount - demoCount
		genBriefs = []string{briefs[len(briefs)-1]}
		uc.log.Info("переиспользуем демо-клипы как версии заказа, догенерируем недостающие",
			"order_id", order.ID(), "demo_clips", demoCount, "to_generate", trackCount)
	}

	req := domain.MusicGenerationRequest{
		Briefs:       genBriefs,
		Style:        styleTags,
		Instrumental: suno.IsInstrumentalFromTags(styleTags),
		TrackCount:   trackCount,
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
		// Задача поллинга не встала в очередь, но Suno-задание уже создано и слот
		// аккаунта занят. Вернуть ошибку без отката — значит оставить заказ в
		// processing навсегда (retry ProcessGenerationTask проходит мимо не-queued).
		// Откатываем в queued и освобождаем аккаунт: cron или Asynq retry создаст
		// новый generate-запрос. Дублирующий Suno-вызов менее критичен, чем зависший
		// заказ с заблокированным аккаунтом.
		uc.log.Error("не удалось поставить задачу поллинга — откатываем в queued",
			"order_id", order.ID(), "suno_job", sunoJobID, "err", err)
		_ = order.RequeueForRetry()
		account.ReleaseSlot()
		metrics.ActiveWorkerSlots.Dec()
		if saveErr := uc.saveOrderAndAccount(ctx, order, account); saveErr != nil {
			uc.log.Error("не удалось откатить состояние после ошибки постановки поллинга",
				"order_id", order.ID(), "account_id", account.ID(), "err", saveErr)
		}
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
		// Переиспользование демо: если сохранённые полные демо-клипы вместе с только
		// что сгенерированными укладываются в продуктовые 4 версии, демо становится
		// версиями 1–2, а новые — последующими. Решение принимается по числу клипов,
		// поэтому остаётся корректным даже в гонке «демо доготовилось после старта
		// платной генерации»: если платный поток выдал полные 4 версии, демо просто
		// игнорируется (его S3-объекты остаются осиротевшими — это безвредно).
		demoClips := order.DemoClips()
		reuseDemo := len(demoClips) > 0 && len(result.Tracks) > 0 &&
			len(result.Tracks) <= domain.DefaultTrackCount-len(demoClips)
		indexOffset := 0
		if reuseDemo {
			indexOffset = len(demoClips)
		}

		var domainTracks []domain.Track
		totalTracks := len(result.Tracks)
		for i, pt := range result.Tracks {
			// Перезаливаем трек в собственное S3-хранилище. Временные ссылки Suno
			// протухают через несколько часов, поэтому загрузка ОБЯЗАТЕЛЬНА: при
			// ошибке S3 мы НЕ завершаем заказ временной ссылкой (через пару часов
			// она отдаст 403), а возвращаем ошибку — Asynq повторит задачу с
			// backoff. Заказ остаётся в processing, слот аккаунта не освобождается.
			// Повторная загрузка идемпотентна: тот же S3-ключ перезаписывается.
			trackIndex := indexOffset + i + 1
			s3Key := fmt.Sprintf("tracks/%s/%d.mp3", order.ID(), trackIndex)
			permanentURL, uploadErr := uc.storage.UploadFromURL(ctx, pt.SourceURL, s3Key, "audio/mpeg")
			if uploadErr != nil {
				uc.log.Error("ошибка загрузки трека в S3, задача будет повторена",
					"order_id", order.ID(), "track_index", trackIndex, "err", uploadErr)
				return fmt.Errorf("загрузка трека %d в постоянное хранилище: %w", trackIndex, uploadErr)
			}
			domainTracks = append(domainTracks, domain.Track{
				ID:          uuid.New(),
				Index:       trackIndex,
				AudioURL:    permanentURL,
				DurationSec: pt.DurationSec,
				SunoTrackID: pt.ExternalID,
			})
			uploadPct := 85 + ((i+1)*14)/max(totalTracks, 1)
			order.UpdateGenerationProgress(domain.GenerationPhaseUploading, uploadPct, indexOffset+i+1)
			if err := uc.orderRepo.Update(ctx, order); err != nil {
				uc.log.Warn("не удалось сохранить прогресс загрузки", "order_id", order.ID(), "err", err)
			}
		}

		// Демо-клипы (уже в постоянном S3) идут версиями 1–2 — клиент получает ровно ту
		// песню, что слушал, плюс догенерированные версии.
		if reuseDemo {
			domainTracks = append(append([]domain.Track{}, demoClips...), domainTracks...)
		}

		// Недопоставка: провайдер отдал меньше продуктовых версий (одна из задач Suno
		// окончательно упала). Доставляем что есть (лучше, чем провал), но громко
		// логируем — это сигнал к ручному разбору и возможному частичному возврату.
		if len(domainTracks) < domain.DefaultTrackCount {
			uc.log.Warn("заказ завершается с неполным числом версий — требуется разбор/частичный возврат",
				"order_id", order.ID(), "got_tracks", len(domainTracks), "expected", domain.DefaultTrackCount,
				"reuse_demo", reuseDemo, "demo_clips", len(demoClips))
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

	// Уведомляем клиента. Ошибка уведомления не роняет задачу —
	// финальный статус уже сохранён в БД и доступен через API.
	switch result.Status {
	case domain.MusicGenerationStatusCompleted:
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
			uc.log.Error("ошибка отправки уведомления об успехе", "order_id", order.ID(), "err", notifyErr)
		}
	case domain.MusicGenerationStatusFailed:
		uc.notifyOrderFailed(ctx, order)
	}

	uc.log.Info("цикл генерации завершен", "order_id", order.ID(), "status", result.Status)
	return nil
}

// pollMaxLifetime — предельный возраст оплаченного заказа в processing, в течение
// которого имеет смысл продолжать опрашивать Suno. Окно ретраев одной poll-задачи
// (~20 мин) меньше, чем иногда длится генерация, поэтому исчерпание ретраев — НЕ
// повод проваливать заказ. Перезапускаем опрос, пока заказ не старше этого порога;
// дальше считаем генерацию реально зависшей. Должен быть меньше stuckOrderThreshold,
// чтобы провалить заказ раньше, чем recovery-крон попытается его перегенерировать.
const pollMaxLifetime = 28 * time.Minute

// RetryStalePoll вызывается терминальным обработчиком, когда задача опроса статуса
// исчерпала ретраи, но Suno всё ещё «не готов» (ErrGenerationNotReady). Возвращает
// requeued=true, если опрос перезапущен (генерация ещё в окне ожидания) или заказ
// уже терминален; requeued=false — заказ пора провалить (вызывающий сделает
// FailGeneration). Так долгая, но живая генерация не падает в ложный failed.
func (uc *OrderUseCase) RetryStalePoll(ctx context.Context, orderID uuid.UUID, sunoJobID string) (bool, error) {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return false, fmt.Errorf("заказ не найден: %w", err)
	}
	// Уже завершён/провален параллельно — проваливать нечего.
	if order.GenerationStatus() != domain.GenerationStatusProcessing {
		return true, nil
	}
	if sunoJobID == "" {
		return false, nil // нечем опрашивать — пусть вызывающий провалит
	}
	// Якорь возраста — момент оплаты (старт оплаченного цикла); запасной — создание.
	anchor := order.CreatedAt()
	if paidAt := order.PaidAt(); paidAt != nil {
		anchor = *paidAt
	}
	if time.Since(anchor) > pollMaxLifetime {
		return false, nil // ждём слишком долго — генерация зависла, проваливаем
	}
	if err := uc.queue.EnqueueStatusCheckTask(ctx, orderID, sunoJobID); err != nil {
		return false, fmt.Errorf("перезапуск опроса статуса: %w", err)
	}
	uc.log.Warn("опрос статуса исчерпал ретраи, но генерация ещё в окне ожидания — перезапускаем опрос",
		"order_id", orderID, "suno_job", sunoJobID, "age", time.Since(anchor).Round(time.Minute))
	return true, nil
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
		uc.notifyOrderFailed(ctx, order)
		return nil
	}

	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("сохранение упавшего заказа: %w", err)
	}
	uc.log.Warn("заказ переведён в failed (аккаунт не был захвачен)", "order_id", orderID, "reason", reason)
	uc.notifyOrderFailed(ctx, order)
	return nil
}

func (uc *OrderUseCase) notifyOrderFailed(ctx context.Context, order *domain.Order) {
	if notifyErr := uc.notifier.NotifyOrderFailed(ctx, notify.OrderFailedNotification{
		OrderID:     order.ID().String(),
		AccessToken: order.AccessToken(),
		Email:       order.CustomerEmail(),
		Phone:       order.CustomerPhone(),
	}); notifyErr != nil {
		uc.log.Error("ошибка отправки уведомления о провале", "order_id", order.ID(), "err", notifyErr)
	}
	uc.notifyAdminAsync(notify.AdminEventNotification{
		Kind:          notify.AdminEventGenerationFailed,
		OrderID:       order.ID().String(),
		InvoiceID:     order.InvoiceID(),
		CustomerEmail: order.CustomerEmail(),
		CustomerPhone: order.CustomerPhone(),
		Brief:         order.Brief(),
		FailureReason: order.FailureReason(),
	})
}

// adminNotifyTimeout — предельное время на отправку одного админского письма.
const adminNotifyTimeout = 30 * time.Second

// notifyAdminAsync шлёт администратору письмо о событии заказа в фоне. Админские
// уведомления некритичны и НЕ должны замедлять платёжный вебхук или воркер-задачи,
// поэтому отправляем в отдельной горутине с собственным контекстом (контекст
// вызывающего к моменту отправки уже может быть отменён вместе с запросом/задачей).
func (uc *OrderUseCase) notifyAdminAsync(n notify.AdminEventNotification) {
	if uc.notifier == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), adminNotifyTimeout)
		defer cancel()
		if err := uc.notifier.NotifyAdmin(ctx, n); err != nil {
			uc.log.Error("ошибка админского уведомления о заказе",
				"kind", n.Kind, "order_id", n.OrderID, "err", err)
		}
	}()
}

func (uc *OrderUseCase) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения заказа: %w", err)
	}
	return order, nil
}

// GetOrderByInvoiceID находит заказ по InvId (Robokassa) — для PaymentReturnPage.
func (uc *OrderUseCase) GetOrderByInvoiceID(ctx context.Context, invoiceID int64) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("заказ по InvoiceID не найден: %w", err)
	}
	return order, nil
}

// EnqueueGeneration ставит задачу генерации в очередь — используется worker'ом
// при повторном запуске после pool-exhaustion dead-letter.
func (uc *OrderUseCase) EnqueueGeneration(ctx context.Context, orderID uuid.UUID) error {
	return uc.queue.EnqueueGenerationTask(ctx, orderID)
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

// GetOrderForCustomer возвращает заказ по ID, если он принадлежит тому же
// клиенту, что и owner (определён по токену в middleware).
func (uc *OrderUseCase) GetOrderForCustomer(ctx context.Context, owner *domain.Order, orderID uuid.UUID) (*domain.Order, error) {
	if owner == nil {
		return nil, domain.ErrOrderUnauthorized
	}
	if owner.ID() == orderID {
		return owner, nil
	}
	target, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !domain.SameCustomer(owner, target) {
		return nil, domain.ErrOrderAccessDenied
	}
	return target, nil
}

// SetShareRevoked включает или отключает публичную share-ссылку заказа владельцем.
func (uc *OrderUseCase) SetShareRevoked(ctx context.Context, owner *domain.Order, orderID uuid.UUID, revoked bool) error {
	order, err := uc.GetOrderForCustomer(ctx, owner, orderID)
	if err != nil {
		return err
	}
	if revoked {
		if err := order.RevokeShare(); err != nil {
			return err
		}
	} else if err := order.RestoreShare(); err != nil {
		return err
	}
	return uc.orderRepo.Update(ctx, order)
}

// SendAccessLinkEmail отправляет на email ссылку с access_token, если она совпадает
// с email заказа. При несовпадении или отсутствии заказа молча ничего не делает —
// защита от перебора чужих заказов.
func (uc *OrderUseCase) SendAccessLinkEmail(ctx context.Context, orderID uuid.UUID, email string) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			return nil
		}
		return err
	}
	if !domain.CustomerEmailMatches(order, email) {
		return nil
	}
	return uc.notifier.NotifyAccessLink(ctx, notify.AccessLinkNotification{
		OrderID:     order.ID().String(),
		AccessToken: order.AccessToken(),
		Email:       order.CustomerEmail(),
	})
}

// ==========================================
// 3. ФОНОВОЕ ОБСЛУЖИВАНИЕ (Вызывается по расписанию)
// ==========================================

// stuckOrderThreshold — время, после которого заказ в статусе processing считается
// застрявшим. Должен превышать максимальное окно polling'а (MaxRetry(80) с
// экспоненциальным backoff даёт ~15–20 мин и более), иначе recovery-крон счёл бы
// активно генерирующийся заказ застрявшим и запустил вторую генерацию. Опрос
// статуса обновляет updated_at на каждом тике (heartbeat в UpdateGenerationProgress),
// поэтому живой заказ сюда не попадёт; 30 минут — запас для случая, когда сам
// polling-воркер умер и обновления прекратились.
const stuckOrderThreshold = 30 * time.Minute

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

// reconcilePaymentWindow — насколько в прошлое сверка платежей просматривает
// pending-заказы. 72 часа — максимальный срок обработки платежа, который
// покрывает задержки вебхука, перезапуски сервиса и плановые простои.
// Большинство корзин не конвертируется, но лишние запросы к Robokassa по
// истёкшим заказам дешевле, чем потеря оплаченного заказа.
const reconcilePaymentWindow = 72 * time.Hour

// reconcileMinAge — минимальный возраст pending-заказа для сверки. Слишком свежие
// заказы клиент может оплачивать прямо сейчас (вебхук в пути), поэтому им даём
// фору перед опросом Robokassa.
const reconcileMinAge = 2 * time.Minute

// ReconcilePendingPayments — последний рубеж против «клиент заплатил, а песни нет».
// Сверяет неоплаченные (pending) заказы с фактическим статусом платежа в Robokassa
// (OpStateExt). Если деньги списаны, но вебхук ResultURL не дошёл (неверный
// IP-allowlist, сетевой сбой) и клиент не вернулся на страницу (не сработал
// sync-payment), заказ навсегда завис бы в pending при списанных деньгах. Здесь мы
// находим такие заказы и активируем их так же, как это сделал бы вебхук.
//
// Требует подключённого PaymentVerifier (боевая Robokassa). В тестовом режиме
// OpStateExt возвращает «не оплачено», поэтому сверка безопасно ничего не делает.
// HandlePaymentSuccess идемпотентен: если вебхук придёт во время сверки, повторная
// активация не запустит вторую генерацию.
func (uc *OrderUseCase) ReconcilePendingPayments(ctx context.Context) error {
	if uc.paymentVerifier == nil {
		return nil
	}

	now := time.Now().UTC()
	createdAfter := now.Add(-reconcilePaymentWindow)
	createdBefore := now.Add(-reconcileMinAge)

	orders, err := uc.orderRepo.ListPendingPayment(ctx, createdAfter, createdBefore)
	if err != nil {
		return fmt.Errorf("поиск pending-заказов для сверки платежей: %w", err)
	}
	if len(orders) == 0 {
		return nil
	}

	for _, order := range orders {
		kopecks, paid, err := uc.paymentVerifier.GetPaidAmountKopecks(ctx, order.InvoiceID())
		if err != nil {
			uc.log.Warn("сверка платежа: ошибка опроса Robokassa",
				"order_id", order.ID(), "invoice_id", order.InvoiceID(), "err", err)
			continue
		}
		if !paid {
			continue
		}

		if err := uc.HandlePaymentSuccess(ctx, order.InvoiceID(), kopecks); err != nil {
			if errors.Is(err, ErrPaymentAmountMismatch) {
				// Деньги списаны, но сумма не совпала с заказом — автоматически
				// активировать нельзя. Громкий лог для ручного разбора админом.
				uc.log.Error("сверка платежа: оплачена ИНАЯ сумма — требуется ручной разбор",
					"order_id", order.ID(), "invoice_id", order.InvoiceID(),
					"paid_kopecks", kopecks, "order_kopecks", order.AmountKopecks())
				continue
			}
			if errors.Is(err, ErrPaymentWindowExpired) {
				continue
			}
			uc.log.Error("сверка платежа: не удалось активировать оплаченный заказ",
				"order_id", order.ID(), "invoice_id", order.InvoiceID(), "err", err)
			continue
		}

		uc.log.Warn("сверка платежа: найден оплаченный заказ без вебхука — активирован",
			"order_id", order.ID(), "invoice_id", order.InvoiceID())
	}
	return nil
}

// RecoverFreePendingOrders находит бесплатные заказы (amount=0), застрявшие в
// pending из-за сетевой ошибки при активации в CreateOrder или sync-payment.
// Вызывается по расписанию — идемпотентен: applyPaymentSuccess проверяет статус.
func (uc *OrderUseCase) RecoverFreePendingOrders(ctx context.Context) error {
	now := time.Now().UTC()
	// Ищем заказы с нулевой суммой, pending старше 2 минут (старый хвост, не новые).
	createdBefore := now.Add(-2 * time.Minute)
	orders, err := uc.orderRepo.ListPendingPayment(ctx, now.Add(-72*time.Hour), createdBefore)
	if err != nil {
		return fmt.Errorf("поиск pending бесплатных заказов: %w", err)
	}
	for _, order := range orders {
		if order.AmountKopecks() != 0 {
			continue
		}
		uc.log.Warn("восстановление бесплатного pending-заказа",
			"order_id", order.ID(), "invoice_id", order.InvoiceID(),
			"created_at", order.CreatedAt())
		if err := uc.HandlePaymentSuccess(ctx, order.InvoiceID(), 0); err != nil {
			uc.log.Error("не удалось активировать бесплатный pending-заказ",
				"order_id", order.ID(), "err", err)
		}
	}
	return nil
}

// ==========================================
// 4. ДЕМО-ФРАГМЕНТ (бесплатное демо до оплаты — отдельная полоса)
// ==========================================
//
// Демо полностью изолировано от платёжного пути: пишет только demo-колонки
// (UpdateDemo), крутится на низкоприоритетной очереди "demo" и при нехватке
// аккаунтов уступает платным генерациям. Захваченный слот аккаунта хранится в
// demo_account_id, чтобы RecoverStuckDemos освободил его при крахе воркера.

// stuckDemoThreshold — после этого времени демо в processing считается застрявшим
// (демо-задача потеряна). Больше максимального окна опроса демо (40×15с = 10 мин).
const stuckDemoThreshold = 12 * time.Minute

// clipsPerDemoTask — сколько клипов запрашиваем у демо-задачи. Одна Suno-задача
// отдаёт пару клипов (см. clipsPerTask адаптера), поэтому 2 версии = 1 задача:
// стоимость демо не меняется, но оба клипа сохраняются для переиспользования.
const clipsPerDemoTask = 2

// TriggerDemo ставит фоновую задачу генерации демо. Best-effort: вызывается при
// создании заказа, ошибка постановки не влияет на сам заказ и оплату. clientIP
// проверяется по суточному лимиту демо на IP — чтобы один источник не выжег общий
// дневной бюджет демо фейковыми email. При превышении демо просто не запускается.
func (uc *OrderUseCase) TriggerDemo(ctx context.Context, orderID uuid.UUID, clientIP string) error {
	if uc.demoLimiter != nil && clientIP != "" {
		allowed, err := uc.demoLimiter.AllowIP(ctx, clientIP)
		if err != nil {
			// fail-open: расход всё равно ограничен дневным потолком и резервом токенов.
			uc.log.Warn("демо: ошибка проверки IP-лимита — пропускаем проверку", "order_id", orderID, "err", err)
		} else if !allowed {
			uc.log.Info("демо не запущено: превышен суточный лимит демо на IP", "order_id", orderID)
			return nil
		}
	}
	return uc.queue.EnqueueDemoTask(ctx, orderID)
}

// GenerateDemo запускает генерацию демо: захватывает слот аккаунта, отправляет ОДНУ
// задачу Suno (1 клип) и ставит задачу опроса. Идемпотентно и безопасно к гонкам:
// для оплаченного заказа и для уже идущего/готового демо — no-op.
func (uc *OrderUseCase) GenerateDemo(ctx context.Context, orderID uuid.UUID) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("демо: заказ не найден: %w", err)
	}

	// Оплаченному заказу демо не нужно — полная генерация уже идёт/сделает своё.
	if order.PaymentStatus() != domain.PaymentStatusPending {
		return nil
	}
	if order.DemoStatus() == domain.DemoStatusProcessing || order.DemoStatus() == domain.DemoStatusReady {
		return nil
	}

	account, err := uc.accRepo.FetchAndLockAvailable(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNoAvailableAccount) {
			// Пул занят платными — не отбираем, тихо ретраим демо позже.
			uc.log.Info("демо отложено: нет свободных аккаунтов", "order_id", orderID)
			return err
		}
		return fmt.Errorf("демо: захват аккаунта: %w", err)
	}

	// Резерв токенов: не запускаем демо, если у аккаунта баланс на грани — кредиты
	// бронируются под платные заказы. Освобождаем слот и тихо выходим (без ретрая).
	if uc.demoTokenReserve > 0 && account.TokenBalance() <= uc.demoTokenReserve {
		uc.releaseDemoSlot(ctx, account)
		uc.log.Info("демо пропущено: баланс аккаунта в зоне резерва под платные",
			"order_id", orderID, "balance", account.TokenBalance(), "reserve", uc.demoTokenReserve)
		return nil
	}

	// Лимиты демо (дневной + на email). Идемпотентно по orderID. При запрете —
	// освобождаем слот и выходим без ретрая. При ошибке лимитера — fail-closed
	// (не тратим кредиты, если не можем проверить лимит).
	if uc.demoLimiter != nil {
		allowed, limErr := uc.demoLimiter.Reserve(ctx, order.ID(), order.CustomerEmail())
		if limErr != nil {
			uc.releaseDemoSlot(ctx, account)
			uc.log.Warn("демо пропущено: ошибка проверки лимита", "order_id", orderID, "err", limErr)
			return nil
		}
		if !allowed {
			uc.releaseDemoSlot(ctx, account)
			uc.log.Info("демо пропущено: достигнут лимит (дневной или на email)", "order_id", orderID)
			return nil
		}
	}

	if err := order.StartDemo(account.ID()); err != nil {
		// Демо уже запущено параллельно — освобождаем только что захваченный слот.
		uc.releaseDemoSlot(ctx, account)
		return nil
	}
	if err := uc.orderRepo.UpdateDemo(ctx, order); err != nil {
		uc.releaseDemoSlot(ctx, account)
		return fmt.Errorf("демо: сохранение статуса processing: %w", err)
	}

	styleTags := resolveSunoStyleTags(order.SunoPrompt(), order.Brief())
	// Одна Suno-задача всё равно отдаёт пару клипов — запрашиваем оба (TrackCount=2,
	// по-прежнему 1 задача). Раньше второй клип выбрасывался; теперь сохраняем оба,
	// чтобы после оплаты переиспользовать их как версии 1–2 (см. CheckDemoStatus).
	req := domain.MusicGenerationRequest{
		Briefs:       []string{demoBrief(order)},
		Style:        styleTags,
		Instrumental: suno.IsInstrumentalFromTags(styleTags),
		TrackCount:   clipsPerDemoTask,
	}
	sunoJobID, err := uc.provider.SubmitGeneration(ctx, req)
	if err != nil {
		uc.releaseDemoSlot(ctx, account)
		uc.markDemoFailed(ctx, order)
		return fmt.Errorf("демо: submit в Suno: %w", err)
	}

	if err := uc.queue.EnqueueDemoCheckTask(ctx, order.ID(), sunoJobID, account.ID()); err != nil {
		uc.releaseDemoSlot(ctx, account)
		uc.markDemoFailed(ctx, order)
		return fmt.Errorf("демо: постановка опроса: %w", err)
	}

	uc.log.Info("демо: генерация запущена", "order_id", order.ID(), "suno_job", sunoJobID)
	return nil
}

// CheckDemoStatus опрашивает демо-задачу и при готовности заливает 1 клип в S3.
// Возвращает ErrGenerationNotReady, пока Suno не закончил (Asynq ретраит).
func (uc *OrderUseCase) CheckDemoStatus(ctx context.Context, orderID uuid.UUID, sunoJobID string, accountID uuid.UUID) error {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("демо: заказ не найден: %w", err)
	}
	// Демо уже не в работе (recovery пометил failed / гонка) — выходим.
	if order.DemoStatus() != domain.DemoStatusProcessing {
		return nil
	}

	result, err := uc.provider.FetchResult(ctx, sunoJobID)
	if err != nil {
		return fmt.Errorf("демо: опрос Suno: %w", err)
	}
	if result.Status == domain.MusicGenerationStatusPending || result.Status == domain.MusicGenerationStatusRunning {
		return ErrGenerationNotReady
	}

	// Успех с клипами: оформляем демо-фрагмент и заливаем в S3. При ошибке S3 —
	// оставляем processing и ретраим (слот держим), как в платном потоке.
	if result.Status == domain.MusicGenerationStatusCompleted && len(result.Tracks) > 0 {
		// 1. Сохраняем ПОЛНЫЕ клипы на будущее: после оплаты они станут версиями 1–2
		// заказа (демо = ровно та песня, что слушал клиент). Ключи содержат случайный
		// сегмент — до завершения заказа эти URL нигде не отдаются, а угадать их по
		// одному order_id нельзя (иначе полную песню можно было бы забрать бесплатно).
		demoClips, clipsErr := uc.uploadDemoFullClips(ctx, order.ID(), result.Tracks)
		if clipsErr != nil {
			uc.log.Warn("демо: ошибка загрузки полных клипов, задача будет повторена", "order_id", order.ID(), "err", clipsErr)
			return fmt.Errorf("демо: загрузка полных клипов в S3: %w", clipsErr)
		}

		// 2. Витринный фрагмент с водяным знаком (то, что слышит клиент до оплаты).
		s3Key := fmt.Sprintf("demos/%s.mp3", order.ID())
		demoURL, uploadErr := uc.uploadDemoClip(ctx, order.ID(), result.Tracks[0].SourceURL, s3Key)
		if uploadErr != nil {
			uc.log.Warn("демо: ошибка загрузки в S3, задача будет повторена", "order_id", order.ID(), "err", uploadErr)
			return fmt.Errorf("демо: загрузка клипа в S3: %w", uploadErr)
		}
		uc.releaseDemoAccount(ctx, accountID, true)
		if err := order.CompleteDemo(demoURL, demoClips); err != nil {
			return fmt.Errorf("демо: завершение в домене: %w", err)
		}
		if err := uc.orderRepo.UpdateDemo(ctx, order); err != nil {
			return fmt.Errorf("демо: сохранение результата: %w", err)
		}
		uc.log.Info("демо готово", "order_id", order.ID(), "clips", len(demoClips))
		uc.notifyAdminAsync(notify.AdminEventNotification{
			Kind:          notify.AdminEventDemoReady,
			OrderID:       order.ID().String(),
			InvoiceID:     order.InvoiceID(),
			CustomerEmail: order.CustomerEmail(),
			CustomerPhone: order.CustomerPhone(),
			Brief:         order.Brief(),
		})
		return nil
	}

	// Терминально без клипов (failed/timeout) — освобождаем слот, помечаем failed.
	uc.releaseDemoAccount(ctx, accountID, false)
	uc.markDemoFailed(ctx, order)
	uc.log.Warn("демо не удалось", "order_id", order.ID(), "status", result.Status)
	return nil
}

// RecoverStuckDemos освобождает слоты аккаунтов у демо, застрявших в processing
// (краш воркера), и помечает их failed — чтобы демо не отъедало ёмкость пула.
func (uc *OrderUseCase) RecoverStuckDemos(ctx context.Context) error {
	threshold := time.Now().UTC().Add(-stuckDemoThreshold)
	orders, err := uc.orderRepo.ListStuckDemo(ctx, threshold)
	if err != nil {
		return fmt.Errorf("поиск застрявших демо: %w", err)
	}
	if len(orders) == 0 {
		return nil
	}

	uc.log.Warn("обнаружены застрявшие демо, освобождаем слоты", "count", len(orders))
	for _, order := range orders {
		if accountID := order.DemoAccountID(); accountID != nil {
			uc.releaseDemoAccount(ctx, *accountID, false)
		}
		uc.markDemoFailed(ctx, order)
	}
	return nil
}

// uploadDemoClip оформляет и сохраняет демо-фрагмент. Если подключён процессор
// (Фаза 2), вырезает «сочный» участок с водяным знаком и заливает байты; при любой
// его ошибке безопасно деградирует до полного клипа (Фаза 1, UploadFromURL).
func (uc *OrderUseCase) uploadDemoClip(ctx context.Context, orderID uuid.UUID, sourceURL, s3Key string) (string, error) {
	if uc.demoClip != nil {
		data, perr := uc.demoClip.Process(ctx, sourceURL)
		if perr != nil {
			uc.log.Warn("демо: обработка фрагмента не удалась — отдаём полный клип",
				"order_id", orderID, "err", perr)
		} else if url, uerr := uc.storage.Upload(ctx, s3Key, "audio/mpeg", data); uerr != nil {
			// Ошибка S3 — пробрасываем (ретрай задачи), не молчим.
			return "", uerr
		} else {
			return url, nil
		}
	}
	// Фаза 1 / фоллбэк: полный клип напрямую из источника.
	return uc.storage.UploadFromURL(ctx, sourceURL, s3Key, "audio/mpeg")
}

// uploadDemoFullClips заливает ПОЛНЫЕ (без водяного знака) клипы демо-задачи в
// постоянное хранилище и возвращает доменные Track для переиспользования после
// оплаты. Ключи содержат случайный UUID-сегмент: до завершения заказа эти URL
// нигде не публикуются, а угадать их по order_id невозможно — иначе полную песню
// можно было бы скачать в обход оплаты. При ошибке S3 возвращаем ошибку (ретрай).
func (uc *OrderUseCase) uploadDemoFullClips(ctx context.Context, orderID uuid.UUID, tracks []domain.ProviderTrack) ([]domain.Track, error) {
	clips := make([]domain.Track, 0, len(tracks))
	for i, pt := range tracks {
		s3Key := fmt.Sprintf("tracks/%s/demo-%s.mp3", orderID, uuid.NewString())
		permanentURL, err := uc.storage.UploadFromURL(ctx, pt.SourceURL, s3Key, "audio/mpeg")
		if err != nil {
			return nil, fmt.Errorf("загрузка демо-клипа %d: %w", i+1, err)
		}
		clips = append(clips, domain.Track{
			ID:          uuid.New(),
			Index:       i + 1,
			AudioURL:    permanentURL,
			DurationSec: pt.DurationSec,
			SunoTrackID: pt.ExternalID,
		})
	}
	return clips, nil
}

// releaseDemoSlot освобождает слот аккаунта (без расхода токена) — для откатов,
// когда демо-генерация не стартовала.
func (uc *OrderUseCase) releaseDemoSlot(ctx context.Context, account *domain.SunoAccount) {
	account.ReleaseSlot()
	if err := uc.accRepo.Update(ctx, account); err != nil {
		uc.log.Error("демо: не удалось освободить слот аккаунта", "account_id", account.ID(), "err", err)
	}
}

// releaseDemoAccount загружает аккаунт по ID и освобождает слот. consumed=true
// дополнительно списывает токен (демо реально потратило кредит Suno).
func (uc *OrderUseCase) releaseDemoAccount(ctx context.Context, accountID uuid.UUID, consumed bool) {
	account, err := uc.accRepo.GetByID(ctx, accountID)
	if err != nil {
		uc.log.Error("демо: аккаунт для освобождения слота не найден", "account_id", accountID, "err", err)
		return
	}
	if consumed {
		_ = account.ConsumeTokens(1)
		account.ResetFailures()
	}
	account.ReleaseSlot()
	if err := uc.accRepo.Update(ctx, account); err != nil {
		uc.log.Error("демо: не удалось сохранить освобождение слота", "account_id", accountID, "err", err)
	}
}

// markDemoFailed помечает демо неуспешным (снимает привязку к аккаунту).
func (uc *OrderUseCase) markDemoFailed(ctx context.Context, order *domain.Order) {
	order.FailDemo()
	if err := uc.orderRepo.UpdateDemo(ctx, order); err != nil {
		uc.log.Error("демо: не удалось пометить failed", "order_id", order.ID(), "err", err)
	}
}

// demoBrief возвращает единственный бриф для демо: готовый промпт квиза либо brief.
func demoBrief(order *domain.Order) string {
	if raw := order.SunoPrompt(); raw != "" {
		return raw
	}
	return order.Brief()
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

func (uc *OrderUseCase) buildGenerationBriefs(_ context.Context, order *domain.Order) ([]string, error) {
	raw := order.SunoPrompt()
	if raw != "" {
		uc.log.Info("используем готовый промпт из квиза", "order_id", order.ID(), "category_id", order.CategoryID())
	} else {
		raw = order.Brief()
		uc.log.Info("используем brief напрямую в Suno (Inspiration Mode)", "order_id", order.ID())
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("пустой промпт для генерации")
	}
	return sunoInspirationBriefs(raw), nil

	/* --- LLM (ChatGPT / OpenRouter) отключён: Suno сам пишет текст ---
	uc.log.Info("генерация двух вариантов текста через LLM", "order_id", order.ID())
	briefs, err := uc.llmClient.GenerateLyricsVariants(ctx, suno.BriefStoryForLLM(order.Brief()), domain.DefaultLyricVariantCount)
	if err != nil {
		return nil, err
	}
	if len(briefs) < domain.DefaultLyricVariantCount {
		return nil, fmt.Errorf("LLM вернул %d вариантов, ожидали %d", len(briefs), domain.DefaultLyricVariantCount)
	}
	return briefs[:domain.DefaultLyricVariantCount], nil
	*/
}

func sunoInspirationBriefs(raw string) []string {
	desc := raw
	tags := ""
	if enc, ok := suno.DecodePrompt(raw); ok {
		tags = enc.Tags
		desc = enc.Description
	}
	variant2Desc := desc + quizLyricVariantSuffix
	if tags != "" {
		return []string{
			suno.EncodePrompt(tags, desc),
			suno.EncodePrompt(tags, variant2Desc),
		}
	}
	return []string{desc, variant2Desc}
}

func resolveSunoStyleTags(sunoPrompt, brief string) string {
	if enc, ok := suno.DecodePrompt(sunoPrompt); ok && enc.Tags != "" {
		return enc.Tags
	}
	if tags := suno.ExtractTagsFromBrief(brief); tags != "" {
		return tags
	}
	if enc, ok := suno.DecodePrompt(brief); ok {
		return enc.Tags
	}
	return ""
}
