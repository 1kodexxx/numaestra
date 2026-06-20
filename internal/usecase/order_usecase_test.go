package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
)

type fixture struct {
	orderRepo *inMemOrderRepo
	accRepo   *inMemAccountRepo
	queue     *mockQueue
	provider  *mockProvider
	storage   *mockStorage
	notifier  *mockNotifier
	llm       *mockLLM
	uc        *OrderUseCase
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		orderRepo: newInMemOrderRepo(),
		accRepo:   newInMemAccountRepo(),
		queue:     &mockQueue{},
		provider:  &mockProvider{},
		storage:   &mockStorage{},
		notifier:  &mockNotifier{},
		llm:       &mockLLM{},
	}
	pricing := NewStaticPricing(map[string]int64{"standard": 150000}, "standard")
	f.uc = NewOrderUseCase(f.orderRepo, f.accRepo, f.queue, f.provider, f.storage, f.notifier, f.llm, &PromptUseCase{}, pricing, fakeTxManager{}, testLogger())
	return f
}

func (f *fixture) addAccount(t *testing.T, tokens int) *domain.SunoAccount {
	t.Helper()
	acc, err := domain.NewSunoAccount("acc@example.com", "encrypted-session", tokens)
	if err != nil {
		t.Fatalf("создание аккаунта: %v", err)
	}
	_ = f.accRepo.Create(context.Background(), acc)
	return acc
}

// --- CreateOrder ---

func TestCreateOrder_Success(t *testing.T) {
	f := newFixture(t)
	order, err := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Песня на ДР", "standard", "", nil)
	if err != nil {
		t.Fatalf("CreateOrder упал: %v", err)
	}
	if order.AmountKopecks() != 150000 {
		t.Errorf("цену определяет сервер по тарифу standard=150000, получили %d", order.AmountKopecks())
	}
	if order.InvoiceID() == 0 {
		t.Error("InvoiceID должен быть присвоен из sequence")
	}
	if order.AccessToken() == "" {
		t.Error("должен быть сгенерирован access token")
	}
	// Заказ должен быть найден по токену (проверяет, что Create реально сохранил данные).
	got, err := f.uc.GetOrderByToken(context.Background(), order.AccessToken())
	if err != nil {
		t.Fatalf("заказ не найден по токену после создания: %v", err)
	}
	if got.ID() != order.ID() {
		t.Error("найденный по токену заказ не совпадает с созданным")
	}
}

func TestCreateOrder_InvalidEmail(t *testing.T) {
	f := newFixture(t)

	// Некорректный email должен возвращать ErrInvalidEmail.
	_, err := f.uc.CreateOrder(context.Background(), "не_email", "", "Бриф", "standard", "", nil)
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("ожидали ErrInvalidEmail для строки 'не_email', получили %v", err)
	}

	// Пустой email допустим — поле необязательное.
	_, err = f.uc.CreateOrder(context.Background(), "", "+79991234567", "Бриф", "standard", "", nil)
	if err != nil {
		t.Fatalf("пустой email при наличии телефона должен быть допустим: %v", err)
	}

	// Корректный email проходит.
	_, err = f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)
	if err != nil {
		t.Fatalf("корректный email должен проходить валидацию: %v", err)
	}
}

func TestCreateOrder_ValidationError(t *testing.T) {
	f := newFixture(t)
	_, err := f.uc.CreateOrder(context.Background(), "", "", "", "standard", "", nil)
	if err == nil {
		t.Fatal("ожидали ошибку валидации при пустых данных")
	}
}

func TestCreateOrder_RepoError(t *testing.T) {
	f := newFixture(t)
	f.orderRepo.createErr = errors.New("db down")
	_, err := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)
	if err == nil {
		t.Fatal("ожидали ошибку при сбое сохранения")
	}
}

func TestCreateOrder_UnknownPlan(t *testing.T) {
	f := newFixture(t)
	_, err := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "vip-неизвестный", "", nil)
	if !errors.Is(err, ErrUnknownPlan) {
		t.Fatalf("ожидали ErrUnknownPlan, получили %v", err)
	}
}

func TestCreateOrder_PriceFromServer_NotClient(t *testing.T) {
	f := newFixture(t)
	// Пустой тариф → дефолтный standard. Клиент не может задать произвольную цену.
	order, err := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "", "", nil)
	if err != nil {
		t.Fatalf("CreateOrder упал: %v", err)
	}
	if order.AmountKopecks() != 150000 {
		t.Errorf("ожидали серверную цену 150000 для дефолтного тарифа, получили %d", order.AmountKopecks())
	}
}

// --- HandlePaymentSuccess ---

func TestHandlePaymentSuccess_Success(t *testing.T) {
	f := newFixture(t)
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	if err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 150000); err != nil {
		t.Fatalf("HandlePaymentSuccess упал: %v", err)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.PaymentStatus() != domain.PaymentStatusPaid {
		t.Errorf("ожидали статус paid, получили %q", got.PaymentStatus())
	}
	if got.GenerationStatus() != domain.GenerationStatusQueued {
		t.Errorf("ожидали статус queued, получили %q", got.GenerationStatus())
	}
	if len(f.queue.genCalls) != 1 {
		t.Errorf("ожидали 1 постановку задачи генерации, получили %d", len(f.queue.genCalls))
	}
}

func TestHandlePaymentSuccess_AmountMismatch(t *testing.T) {
	f := newFixture(t)
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 100000)
	if !errors.Is(err, ErrPaymentAmountMismatch) {
		t.Fatalf("ожидали ErrPaymentAmountMismatch, получили %v", err)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.PaymentStatus() != domain.PaymentStatusPending {
		t.Error("заказ не должен переходить в paid при несовпадении суммы")
	}
	if len(f.queue.genCalls) != 0 {
		t.Error("задача генерации не должна ставиться при несовпадении суммы")
	}
}

func TestHandlePaymentSuccess_UnknownInvoice(t *testing.T) {
	f := newFixture(t)
	err := f.uc.HandlePaymentSuccess(context.Background(), 99999, 100)
	if err == nil {
		t.Fatal("ожидали ошибку для несуществующего инвойса")
	}
}

func TestHandlePaymentSuccess_DuplicateWebhook_Idempotent(t *testing.T) {
	f := newFixture(t)
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	// Первая доставка вебхука.
	if err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 150000); err != nil {
		t.Fatalf("первая доставка упала: %v", err)
	}
	// Повторная доставка (Robokassa ретраит) — должна быть идемпотентной (nil), без второй задачи.
	if err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 150000); err != nil {
		t.Fatalf("повторная доставка должна возвращать успех, получили %v", err)
	}
	if len(f.queue.genCalls) != 1 {
		t.Errorf("при дубликате вебхука должна быть ровно 1 задача генерации, получили %d", len(f.queue.genCalls))
	}
}

// TestApplyPaymentSuccess_OnlyFromPending проверяет условный переход на уровне
// репозитория: повторное применение к уже оплаченному заказу даёт applied=false.
func TestApplyPaymentSuccess_OnlyFromPending(t *testing.T) {
	f := newFixture(t)
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	loaded, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	_ = loaded.MarkPaid()
	_ = loaded.Enqueue()

	applied, err := f.orderRepo.ApplyPaymentSuccess(context.Background(), loaded)
	if err != nil || !applied {
		t.Fatalf("первый переход должен примениться: applied=%v err=%v", applied, err)
	}
	// Повторное применение к тому же заказу (уже paid) — applied=false.
	applied2, err := f.orderRepo.ApplyPaymentSuccess(context.Background(), loaded)
	if err != nil {
		t.Fatalf("ошибка при повторном применении: %v", err)
	}
	if applied2 {
		t.Error("повторный переход из не-pending не должен применяться (applied=false)")
	}
}

// --- FailGeneration ---

func TestFailGeneration_FromProcessing_ReleasesAccount(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	if err := f.uc.FailGeneration(context.Background(), order.ID(), "исчерпаны ретраи опроса"); err != nil {
		t.Fatalf("FailGeneration упал: %v", err)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusFailed {
		t.Errorf("ожидали failed, получили %q", got.GenerationStatus())
	}
	if n := f.accRepo.concurrentOf(acc.ID()); n != 0 {
		t.Errorf("слот аккаунта должен быть освобождён при провале, занято=%d", n)
	}
}

func TestFailGeneration_AlreadyTerminal_NoOp(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	tracks := []domain.Track{{ID: uuid.New(), Index: 1, AudioURL: "u"}}
	_ = order.Complete(tracks)
	_ = f.orderRepo.Update(context.Background(), order)

	// Для уже завершённого заказа FailGeneration не должен ничего менять.
	if err := f.uc.FailGeneration(context.Background(), order.ID(), "поздний провал"); err != nil {
		t.Fatalf("FailGeneration на завершённом заказе не должен падать: %v", err)
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusCompleted {
		t.Errorf("статус завершённого заказа не должен меняться, получили %q", got.GenerationStatus())
	}
}

// --- ProcessGenerationTask ---

func TestProcessGenerationTask_Success(t *testing.T) {
	f := newFixture(t)
	f.addAccount(t, 10)
	order := f.queuedOrder(t)

	f.provider.submitFn = func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		return "job-123", nil
	}

	if err := f.uc.ProcessGenerationTask(context.Background(), order.ID()); err != nil {
		t.Fatalf("ProcessGenerationTask упал: %v", err)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusProcessing {
		t.Errorf("ожидали processing, получили %q", got.GenerationStatus())
	}
	if len(f.queue.statusCalls) != 1 || f.queue.statusCalls[0].SunoJobID != "job-123" {
		t.Errorf("ожидали постановку задачи опроса с job-123, получили %+v", f.queue.statusCalls)
	}
}

func TestProcessGenerationTask_NoAccount(t *testing.T) {
	f := newFixture(t)
	f.accRepo.noFree = true
	order := f.queuedOrder(t)

	err := f.uc.ProcessGenerationTask(context.Background(), order.ID())
	if !errors.Is(err, domain.ErrNoAvailableAccount) {
		t.Fatalf("ожидали ErrNoAvailableAccount (для retry), получили %v", err)
	}
}

func TestProcessGenerationTask_ProviderError_Requeues(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.queuedOrder(t)

	f.provider.submitFn = func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		return "", errors.New("suno 500")
	}

	err := f.uc.ProcessGenerationTask(context.Background(), order.ID())
	if err == nil {
		t.Fatal("ожидали ошибку провайдера")
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusQueued {
		t.Errorf("заказ должен вернуться в queued для повторной попытки, получили %q", got.GenerationStatus())
	}
	// Слот аккаунта не должен застрять занятым после ошибки провайдера.
	if n := f.accRepo.concurrentOf(acc.ID()); n != 0 {
		t.Errorf("слот аккаунта должен быть освобождён после ошибки провайдера, занято=%d", n)
	}
}

// При недоступности LLM заказ НЕ должен уходить в Suno с сырым брифом.
// Ожидаем ошибку (для retry Asynq), заказ возвращается в queued, аккаунт свободен,
// и задача опроса не ставится.
func TestProcessGenerationTask_LLMFailure_Requeues(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.queuedOrder(t)

	f.llm.fn = func(_ context.Context, _ string) (string, error) {
		return "", errors.New("openrouter 503")
	}
	submitCalled := false
	f.provider.submitFn = func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		submitCalled = true
		return "job-x", nil
	}

	err := f.uc.ProcessGenerationTask(context.Background(), order.ID())
	if err == nil {
		t.Fatal("ожидали ошибку при недоступности LLM, чтобы Asynq повторил задачу")
	}
	if submitCalled {
		t.Error("при ошибке LLM генерация в Suno не должна запускаться")
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusQueued {
		t.Errorf("заказ должен вернуться в queued, получили %q", got.GenerationStatus())
	}
	// Аккаунт не должен застрять занятым: счётчик неудач не растёт (вина не аккаунта).
	gotAcc, _ := f.accRepo.GetByID(context.Background(), acc.ID())
	if gotAcc.FailureCount() != 0 {
		t.Errorf("сбой LLM не должен увеличивать счётчик ошибок аккаунта, получили %d", gotAcc.FailureCount())
	}
	if len(f.queue.statusCalls) != 0 {
		t.Error("задача опроса не должна ставиться при ошибке LLM")
	}
}

func TestProcessGenerationTask_WrongStatus_Skipped(t *testing.T) {
	f := newFixture(t)
	f.addAccount(t, 10)
	// Заказ в статусе New (не Queued) — задача должна быть пропущена без ошибки.
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	if err := f.uc.ProcessGenerationTask(context.Background(), order.ID()); err != nil {
		t.Fatalf("пропуск задачи с неверным статусом не должен возвращать ошибку: %v", err)
	}
	if len(f.queue.statusCalls) != 0 {
		t.Error("не должно быть постановки задачи опроса для пропущенного заказа")
	}
}

// --- CheckGenerationStatus ---

func TestCheckGenerationStatus_Completed(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	f.provider.fetchFn = func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{
			Status: domain.MusicGenerationStatusCompleted,
			Tracks: []domain.ProviderTrack{
				{SourceURL: "https://suno/1.mp3", DurationSec: 180, ExternalID: "ext-1"},
				{SourceURL: "https://suno/2.mp3", DurationSec: 175, ExternalID: "ext-2"},
			},
		}, nil
	}

	if err := f.uc.CheckGenerationStatus(context.Background(), order.ID(), "job-1"); err != nil {
		t.Fatalf("CheckGenerationStatus упал: %v", err)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusCompleted {
		t.Errorf("ожидали completed, получили %q", got.GenerationStatus())
	}
	if len(got.Tracks()) != 2 {
		t.Errorf("ожидали 2 трека, получили %d", len(got.Tracks()))
	}
	if got.Tracks()[0].AudioURL == "https://suno/1.mp3" {
		t.Error("URL трека должен быть заменён на постоянную ссылку S3")
	}
	if len(f.notifier.calls) != 1 {
		t.Errorf("ожидали 1 уведомление клиента, получили %d", len(f.notifier.calls))
	}
}

func TestCheckGenerationStatus_StillRunning(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	f.provider.fetchFn = func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusRunning}, nil
	}

	err := f.uc.CheckGenerationStatus(context.Background(), order.ID(), "job-1")
	if !errors.Is(err, ErrGenerationNotReady) {
		t.Fatalf("ожидали ErrGenerationNotReady, получили %v", err)
	}
}

func TestCheckGenerationStatus_Failed(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	f.provider.fetchFn = func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusFailed, Error: "moderation"}, nil
	}

	if err := f.uc.CheckGenerationStatus(context.Background(), order.ID(), "job-1"); err != nil {
		t.Fatalf("CheckGenerationStatus упал: %v", err)
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusFailed {
		t.Errorf("ожидали failed, получили %q", got.GenerationStatus())
	}
	if len(f.notifier.calls) != 0 {
		t.Error("при провале уведомление о готовности не должно отправляться")
	}
}

// При ошибке S3 заказ НЕ должен завершаться временной ссылкой Suno (она протухнет
// через несколько часов). Вместо этого CheckGenerationStatus возвращает ошибку,
// заказ остаётся в processing, и Asynq повторит задачу.
func TestCheckGenerationStatus_S3Failure_RetriesAndKeepsProcessing(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	f.storage.uploadFn = func(_ context.Context, sourceURL, _, _ string) (string, error) {
		return "", errors.New("s3 unavailable")
	}
	f.provider.fetchFn = func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{
			Status: domain.MusicGenerationStatusCompleted,
			Tracks: []domain.ProviderTrack{{SourceURL: "https://suno/temp.mp3", DurationSec: 100, ExternalID: "x"}},
		}, nil
	}

	err := f.uc.CheckGenerationStatus(context.Background(), order.ID(), "job-1")
	if err == nil {
		t.Fatal("ожидали ошибку при недоступности S3, чтобы Asynq повторил задачу")
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusProcessing {
		t.Errorf("при ошибке S3 заказ должен остаться в processing, получили %q", got.GenerationStatus())
	}
	if len(f.notifier.calls) != 0 {
		t.Error("уведомление не должно отправляться, пока треки не в постоянном хранилище")
	}
}

// --- List / Get ---

func TestListOrdersByEmail_EmptyEmail(t *testing.T) {
	f := newFixture(t)
	if _, err := f.uc.ListOrdersByEmail(context.Background(), "", 20, 0); err == nil {
		t.Error("ожидали ошибку при пустом email")
	}
}

func TestListOrdersByEmail_ReturnsMatching(t *testing.T) {
	f := newFixture(t)
	order, err := f.uc.CreateOrder(context.Background(), "find@example.com", "", "Бриф", "standard", "", nil)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	_, _ = f.uc.CreateOrder(context.Background(), "other@example.com", "", "Другой", "standard", "", nil)

	list, err := f.uc.ListOrdersByEmail(context.Background(), "find@example.com", 20, 0)
	if err != nil {
		t.Fatalf("ListOrdersByEmail: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ожидали 1 заказ, получили %d", len(list))
	}
	if list[0].ID() != order.ID() {
		t.Errorf("вернулся не тот заказ: got %v, want %v", list[0].ID(), order.ID())
	}
}

func TestGetOrderByToken_Empty(t *testing.T) {
	f := newFixture(t)
	if _, err := f.uc.GetOrderByToken(context.Background(), ""); !errors.Is(err, domain.ErrOrderUnauthorized) {
		t.Errorf("ожидали ErrOrderUnauthorized, получили %v", err)
	}
}

func TestGetOrder_Found(t *testing.T) {
	f := newFixture(t)
	order, err := f.uc.CreateOrder(context.Background(), "get@test.com", "", "бриф", "standard", "", nil)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	got, err := f.uc.GetOrder(context.Background(), order.ID())
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.ID() != order.ID() {
		t.Errorf("ожидали id=%s, получили %s", order.ID(), got.ID())
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.uc.GetOrder(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("ожидали ErrOrderNotFound, получили %v", err)
	}
}

func TestListOrdersByPhone_EmptyPhone(t *testing.T) {
	f := newFixture(t)
	if _, err := f.uc.ListOrdersByPhone(context.Background(), "", 20, 0); err == nil {
		t.Error("ожидали ошибку при пустом phone")
	}
}

func TestListOrdersByPhone_ReturnsMatching(t *testing.T) {
	f := newFixture(t)

	for i := int64(1); i <= 3; i++ {
		o, _ := domain.NewOrder(i, "p@q.com", "+79991234567", "бриф", "", "", 100)
		_ = f.orderRepo.Create(context.Background(), o)
	}
	o, _ := domain.NewOrder(4, "x@y.com", "+70000000000", "бриф", "", "", 100)
	_ = f.orderRepo.Create(context.Background(), o)

	list, err := f.uc.ListOrdersByPhone(context.Background(), "+79991234567", 20, 0)
	if err != nil {
		t.Fatalf("ListOrdersByPhone: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ожидали 3 заказа, получили %d", len(list))
	}
}

// --- FailGeneration extra paths ---

func TestFailGeneration_OrderNotFound(t *testing.T) {
	f := newFixture(t)
	err := f.uc.FailGeneration(context.Background(), uuid.New(), "timeout")
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("ожидали ErrOrderNotFound, получили %v", err)
	}
}

func TestFailGeneration_NoAssignedAccount(t *testing.T) {
	f := newFixture(t)
	order := f.queuedOrder(t) // queued, AssignedAccountID == nil

	if err := f.uc.FailGeneration(context.Background(), order.ID(), "исчерпаны ретраи"); err != nil {
		t.Fatalf("FailGeneration без аккаунта упал: %v", err)
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusFailed {
		t.Errorf("ожидали failed, получили %q", got.GenerationStatus())
	}
}

func TestFailGeneration_AccountNotFound_StillUpdatesOrder(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	// Удаляем аккаунт из in-memory хранилища, чтобы GetByID вернул ErrAccountNotFound.
	f.accRepo.mu.Lock()
	delete(f.accRepo.accounts, acc.ID())
	f.accRepo.mu.Unlock()

	if err := f.uc.FailGeneration(context.Background(), order.ID(), "timeout"); err != nil {
		t.Fatalf("FailGeneration должен обновить заказ даже если аккаунт не найден: %v", err)
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusFailed {
		t.Errorf("заказ должен стать failed, получили %q", got.GenerationStatus())
	}
}

// --- HandlePaymentSuccess extra paths ---

func TestHandlePaymentSuccess_NotApplied_SkipsEnqueue(t *testing.T) {
	f := newFixture(t)
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	f.orderRepo.applyPaymentNotApplied = true

	if err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 150000); err != nil {
		t.Fatalf("HandlePaymentSuccess должен вернуть nil при applied=false: %v", err)
	}
	if len(f.queue.genCalls) != 0 {
		t.Error("задача генерации не должна ставиться при applied=false")
	}
}

func TestHandlePaymentSuccess_EnqueueError(t *testing.T) {
	f := newFixture(t)
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	f.queue.enqueueGenErr = errors.New("очередь недоступна")

	err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 150000)
	if err == nil {
		t.Fatal("ожидали ошибку при сбое постановки в очередь")
	}
}

func TestHandlePaymentSuccess_ApplyPaymentError(t *testing.T) {
	f := newFixture(t)
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)

	f.orderRepo.applyPaymentErr = errors.New("db error")

	err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 150000)
	if err == nil {
		t.Fatal("ожидали ошибку при сбое ApplyPaymentSuccess")
	}
}

// --- saveOrderAndAccount error path ---

func TestSaveOrderAndAccount_AccountUpdateFails(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 10)
	order := f.processingOrder(t, acc)

	// FailGeneration → saveOrderAndAccount → accRepo.Update → возвращает ошибку.
	f.accRepo.updateErr = errors.New("db down")

	err := f.uc.FailGeneration(context.Background(), order.ID(), "timeout")
	if err == nil {
		t.Fatal("ожидали ошибку при сбое сохранения аккаунта")
	}
}

// --- helpers ---

func (f *fixture) queuedOrder(t *testing.T) *domain.Order {
	t.Helper()
	order, _ := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "standard", "", nil)
	if err := f.uc.HandlePaymentSuccess(context.Background(), order.InvoiceID(), 150000); err != nil {
		t.Fatalf("подготовка queued-заказа: %v", err)
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	return got
}

func (f *fixture) processingOrder(t *testing.T, acc *domain.SunoAccount) *domain.Order {
	t.Helper()
	order := f.queuedOrder(t)
	if err := order.StartProcessing(acc.ID()); err != nil {
		t.Fatalf("перевод в processing: %v", err)
	}
	if err := f.orderRepo.Update(context.Background(), order); err != nil {
		t.Fatalf("сохранение processing-заказа: %v", err)
	}
	return order
}
