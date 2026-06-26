package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/repository/queue"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/pkg/notify"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// passthroughTx — заглушка Unit of Work для тестов воркера: выполняет fn без
// реальной транзакции (in-memory репозитории атомарны сами по себе).
type passthroughTx struct{}

func (passthroughTx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// --- Ошибки разбора payload ---

func TestHandleGenerateTask_BadPayload_SkipRetry(t *testing.T) {
	p := NewOrderProcessor(usecase.NewOrderUseCase(nil, nil, nil, nil, nil, nil, nil, usecase.NewNoopPromptUseCase(), 0, nil, discardLogger()), discardLogger())
	task := asynq.NewTask(queue.TaskTypeGenerateTrack, []byte("{не json"))
	err := p.HandleGenerateTask(context.Background(), task)
	if err == nil {
		t.Fatal("ожидали ошибку при некорректном payload")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("ошибка разбора payload должна оборачивать asynq.SkipRetry, получили %v", err)
	}
}

func TestHandleStatusCheckTask_BadPayload_SkipRetry(t *testing.T) {
	p := NewOrderProcessor(usecase.NewOrderUseCase(nil, nil, nil, nil, nil, nil, nil, usecase.NewNoopPromptUseCase(), 0, nil, discardLogger()), discardLogger())
	task := asynq.NewTask(queue.TaskTypeCheckStatus, []byte("garbage"))
	err := p.HandleStatusCheckTask(context.Background(), task)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("ожидали обёртку asynq.SkipRetry, получили %v", err)
	}
}

// --- Проброс ErrGenerationNotReady ---

func TestHandleStatusCheckTask_NotReady_PassesThrough(t *testing.T) {
	repo := newWOrderRepo()
	acc := newWAccountRepo()

	// Готовим аккаунт и заказ в статусе processing.
	account, _ := domain.NewSunoAccount("a@b.c", "sess", 10)
	_ = acc.Create(context.Background(), account)

	order, _ := domain.NewOrder(1, "user@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(account.ID())
	repo.put(order)

	provider := &wProvider{fetchFn: func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusRunning}, nil
	}}

	uc := usecase.NewOrderUseCase(repo, acc, &wQueue{}, provider, &wStorage{}, &wNotifier{}, &wLLM{}, usecase.NewNoopPromptUseCase(), 0, passthroughTx{}, discardLogger())
	p := NewOrderProcessor(uc, discardLogger())

	payload, _ := json.Marshal(queue.StatusCheckTaskPayload{OrderID: order.ID(), SunoJobID: "job-1"})
	task := asynq.NewTask(queue.TaskTypeCheckStatus, payload)

	err := p.HandleStatusCheckTask(context.Background(), task)
	if !errors.Is(err, usecase.ErrGenerationNotReady) {
		t.Errorf("воркер должен пробрасывать ErrGenerationNotReady для повторного опроса, получили %v", err)
	}
}

func TestHandleGenerateTask_Success(t *testing.T) {
	repo := newWOrderRepo()
	acc := newWAccountRepo()
	account, _ := domain.NewSunoAccount("a@b.c", "sess", 10)
	_ = acc.Create(context.Background(), account)

	order, _ := domain.NewOrder(1, "user@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = order.MarkPaid()
	_ = order.Enqueue()
	repo.put(order)

	provider := &wProvider{submitFn: func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		return "job-xyz", nil
	}}
	q := &wQueue{}
	uc := usecase.NewOrderUseCase(repo, acc, q, provider, &wStorage{}, &wNotifier{}, &wLLM{}, usecase.NewNoopPromptUseCase(), 0, passthroughTx{}, discardLogger())
	p := NewOrderProcessor(uc, discardLogger())

	payload, _ := json.Marshal(queue.GenerationTaskPayload{OrderID: order.ID()})
	task := asynq.NewTask(queue.TaskTypeGenerateTrack, payload)

	if err := p.HandleGenerateTask(context.Background(), task); err != nil {
		t.Fatalf("HandleGenerateTask упал: %v", err)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.statusJobs) != 1 || q.statusJobs[0] != "job-xyz" {
		t.Errorf("ожидали постановку опроса с job-xyz, получили %+v", q.statusJobs)
	}
}

func TestHandleDeadTask_FailsOrderAndReleasesAccount(t *testing.T) {
	repo := newWOrderRepo()
	acc := newWAccountRepo()
	account, _ := domain.NewSunoAccount("a@b.c", "sess", 10)
	_ = acc.Create(context.Background(), account)

	order, _ := domain.NewOrder(1, "user@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(account.ID())
	repo.put(order)

	uc := usecase.NewOrderUseCase(repo, acc, &wQueue{}, &wProvider{}, &wStorage{}, &wNotifier{}, &wLLM{}, usecase.NewNoopPromptUseCase(), 0, passthroughTx{}, discardLogger())
	p := NewOrderProcessor(uc, discardLogger())

	payload, _ := json.Marshal(queue.StatusCheckTaskPayload{OrderID: order.ID(), SunoJobID: "job-1"})
	task := asynq.NewTask(queue.TaskTypeCheckStatus, payload)

	p.HandleDeadTask(context.Background(), task, errors.New("истекли ретраи"))

	got, _ := repo.GetByID(context.Background(), order.ID())
	if got.GenerationStatus() != domain.GenerationStatusFailed {
		t.Errorf("ожидали failed после исчерпания ретраев, получили %q", got.GenerationStatus())
	}
	gotAcc, _ := acc.GetByID(context.Background(), account.ID())
	if gotAcc.Status() == domain.AccountStatusBusy { //nolint:staticcheck // намеренно проверяем legacy-статус: освобождение не должно оставить Busy
		t.Error("аккаунт должен быть освобождён, а не остаться Busy")
	}
}

func TestOrderIDFromTask_InvalidJSONStatusCheck(t *testing.T) {
	task := asynq.NewTask(queue.TaskTypeCheckStatus, []byte("{invalid json"))
	id, ok := orderIDFromTask(task)
	if ok || id != uuid.Nil {
		t.Errorf("ожидали (uuid.Nil, false) для некорректного JSON StatusCheck, получили (%s, %v)", id, ok)
	}
}

func TestOrderIDFromTask_UnknownType(t *testing.T) {
	task := asynq.NewTask("unknown:task:type", []byte(`{}`))
	id, ok := orderIDFromTask(task)
	if ok || id != uuid.Nil {
		t.Errorf("ожидали (uuid.Nil, false) для неизвестного типа задачи, получили (%s, %v)", id, ok)
	}
}

func TestHandleDeadTask_BadPayload_NoPanic(t *testing.T) {
	uc := usecase.NewOrderUseCase(nil, nil, nil, nil, nil, nil, nil, usecase.NewNoopPromptUseCase(), 0, nil, discardLogger())
	p := NewOrderProcessor(uc, discardLogger())
	task := asynq.NewTask(queue.TaskTypeGenerateTrack, []byte("{broken"))
	// Не должно паниковать и не должно вызывать use-case (orderID не извлечён).
	p.HandleDeadTask(context.Background(), task, errors.New("x"))
}

func TestHandleGenerateTask_UseCaseError_ReturnsError(t *testing.T) {
	repo := newWOrderRepo()
	// Нет аккаунтов — FetchAndLockAvailable вернёт ErrNoAvailableAccount →
	// ProcessGenerationTask вернёт ошибку → HandleGenerateTask пробросит её.
	order, _ := domain.NewOrder(1, "user@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = order.MarkPaid()
	_ = order.Enqueue()
	repo.put(order)

	uc := usecase.NewOrderUseCase(repo, newWAccountRepo(), &wQueue{}, &wProvider{
		submitFn: func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
			return "", errors.New("provider down")
		},
	}, &wStorage{}, &wNotifier{}, &wLLM{}, usecase.NewNoopPromptUseCase(), 0, passthroughTx{}, discardLogger())
	p := NewOrderProcessor(uc, discardLogger())

	payload, _ := json.Marshal(queue.GenerationTaskPayload{OrderID: order.ID()})
	task := asynq.NewTask(queue.TaskTypeGenerateTrack, payload)

	if err := p.HandleGenerateTask(context.Background(), task); err == nil {
		t.Fatal("ожидали ошибку при отсутствии доступного аккаунта")
	}
}

func TestHandleStatusCheckTask_CriticalError_Propagates(t *testing.T) {
	repo := newWOrderRepo()
	acc := newWAccountRepo()
	account, _ := domain.NewSunoAccount("a@b.c", "sess", 10)
	_ = acc.Create(context.Background(), account)

	order, _ := domain.NewOrder(1, "user@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(account.ID())
	repo.put(order)

	// Провайдер возвращает произвольную ошибку (не ErrGenerationNotReady)
	provider := &wProvider{fetchFn: func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{}, errors.New("suno API unavailable")
	}}

	uc := usecase.NewOrderUseCase(repo, acc, &wQueue{}, provider, &wStorage{}, &wNotifier{}, &wLLM{}, usecase.NewNoopPromptUseCase(), 0, passthroughTx{}, discardLogger())
	p := NewOrderProcessor(uc, discardLogger())

	payload, _ := json.Marshal(queue.StatusCheckTaskPayload{OrderID: order.ID(), SunoJobID: "job-1"})
	task := asynq.NewTask(queue.TaskTypeCheckStatus, payload)

	err := p.HandleStatusCheckTask(context.Background(), task)
	if err == nil {
		t.Fatal("ожидали ошибку при критическом сбое провайдера")
	}
	if errors.Is(err, usecase.ErrGenerationNotReady) {
		t.Error("ошибка должна быть НЕ ErrGenerationNotReady")
	}
}

// --- компактные in-memory моки ---

type wOrderRepo struct {
	mu     sync.Mutex
	orders map[uuid.UUID]domain.OrderSnapshot
	byInv  map[int64]uuid.UUID
}

func newWOrderRepo() *wOrderRepo {
	return &wOrderRepo{orders: map[uuid.UUID]domain.OrderSnapshot{}, byInv: map[int64]uuid.UUID{}}
}
func (r *wOrderRepo) put(o *domain.Order) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := o.Snapshot()
	r.orders[s.ID] = s
	r.byInv[s.InvoiceID] = s.ID
}
func (r *wOrderRepo) Create(_ context.Context, o *domain.Order) error { r.put(o); return nil }
func (r *wOrderRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(s), nil
}
func (r *wOrderRepo) GetByInvoiceID(_ context.Context, inv int64) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byInv[inv]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(r.orders[id]), nil
}
func (r *wOrderRepo) SetAdminFeedback(_ context.Context, id uuid.UUID, feedback string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	s.AdminFeedback = feedback
	s.AdminFeedbackAt = &at
	r.orders[id] = s
	return nil
}
func (r *wOrderRepo) Update(_ context.Context, o *domain.Order) error { r.put(o); return nil }
func (r *wOrderRepo) ApplyPaymentSuccess(_ context.Context, o *domain.Order) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.orders[o.ID()]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	if cur.PaymentStatus != domain.PaymentStatusPending {
		return false, nil
	}
	s := o.Snapshot()
	r.orders[s.ID] = s
	return true, nil
}
func (r *wOrderRepo) ListByCustomerEmail(_ context.Context, _ string, _, _ int) ([]*domain.Order, error) {
	return nil, nil
}
func (r *wOrderRepo) ListByCustomerPhone(_ context.Context, _ string, _, _ int) ([]*domain.Order, error) {
	return nil, nil
}
func (r *wOrderRepo) NextInvoiceID(_ context.Context) (int64, error) { return 1, nil }
func (r *wOrderRepo) GetByAccessToken(_ context.Context, _ string) (*domain.Order, error) {
	return nil, domain.ErrOrderNotFound
}
func (r *wOrderRepo) ListAll(_ context.Context, _, _ int) ([]*domain.Order, error) {
	return nil, nil
}
func (r *wOrderRepo) CountAll(_ context.Context) (int, error) { return 0, nil }
func (r *wOrderRepo) ListStuckProcessing(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}
func (r *wOrderRepo) ListStuckQueued(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}
func (r *wOrderRepo) ListPendingPayment(_ context.Context, _, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}
func (r *wOrderRepo) UpdateDemo(_ context.Context, _ *domain.Order) error { return nil }
func (r *wOrderRepo) ListStuckDemo(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}
func (r *wOrderRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	delete(r.orders, id)
	delete(r.byInv, snap.InvoiceID)
	return nil
}

var _ domain.OrderRepository = (*wOrderRepo)(nil)

type wAccountRepo struct {
	mu   sync.Mutex
	accs map[uuid.UUID]domain.SunoAccountSnapshot
}

func newWAccountRepo() *wAccountRepo {
	return &wAccountRepo{accs: map[uuid.UUID]domain.SunoAccountSnapshot{}}
}
func (r *wAccountRepo) FetchAndLockAvailable(_ context.Context) (*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.accs {
		a := domain.RestoreSunoAccount(s)
		if err := a.AcquireSlot(time.Now().UTC()); err != nil {
			continue
		}
		r.accs[a.ID()] = a.Snapshot()
		return a, nil
	}
	return nil, domain.ErrNoAvailableAccount
}
func (r *wAccountRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.accs[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return domain.RestoreSunoAccount(s), nil
}
func (r *wAccountRepo) Create(_ context.Context, a *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accs[a.ID()] = a.Snapshot()
	return nil
}
func (r *wAccountRepo) Update(_ context.Context, a *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accs[a.ID()] = a.Snapshot()
	return nil
}
func (r *wAccountRepo) ListByStatus(_ context.Context, _ domain.AccountStatus) ([]*domain.SunoAccount, error) {
	return nil, nil
}
func (r *wAccountRepo) List(_ context.Context) ([]*domain.SunoAccount, error) { return nil, nil }
func (r *wAccountRepo) SetStatus(_ context.Context, _ uuid.UUID, _ domain.AccountStatus) error {
	return nil
}
func (r *wAccountRepo) ResetAccount(_ context.Context, _ uuid.UUID, _ bool) error {
	return nil
}

var _ domain.AccountRepository = (*wAccountRepo)(nil)

type wQueue struct {
	mu         sync.Mutex
	statusJobs []string
}

func (q *wQueue) EnqueueGenerationTask(_ context.Context, _ uuid.UUID) error { return nil }
func (q *wQueue) EnqueueDemoTask(_ context.Context, _ uuid.UUID) error       { return nil }
func (q *wQueue) EnqueueDemoCheckTask(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) error {
	return nil
}
func (q *wQueue) EnqueueStatusCheckTask(_ context.Context, _ uuid.UUID, jobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.statusJobs = append(q.statusJobs, jobID)
	return nil
}

var _ domain.QueuePublisher = (*wQueue)(nil)

type wProvider struct {
	submitFn func(ctx context.Context, req domain.MusicGenerationRequest) (string, error)
	fetchFn  func(ctx context.Context, jobID string) (domain.MusicGenerationResult, error)
}

func (p *wProvider) SubmitGeneration(ctx context.Context, req domain.MusicGenerationRequest) (string, error) {
	return p.submitFn(ctx, req)
}
func (p *wProvider) FetchResult(ctx context.Context, jobID string) (domain.MusicGenerationResult, error) {
	return p.fetchFn(ctx, jobID)
}

var _ domain.MusicProvider = (*wProvider)(nil)

type wStorage struct{}

func (s *wStorage) UploadFromURL(_ context.Context, _, key, _ string) (string, error) {
	return "https://s3/" + key, nil
}
func (s *wStorage) Upload(_ context.Context, key, _ string, _ []byte) (string, error) {
	return "https://s3/" + key, nil
}
func (s *wStorage) DeleteOrderTracks(_ context.Context, _ uuid.UUID) error { return nil }

var _ domain.TrackStorage = (*wStorage)(nil)

type wNotifier struct{}

func (n *wNotifier) NotifyOrderComplete(_ context.Context, _ notify.OrderCompleteNotification) error {
	return nil
}

func (n *wNotifier) NotifyAdminFeedback(_ context.Context, _ notify.AdminFeedbackNotification) error {
	return nil
}

func (n *wNotifier) NotifyOrderFailed(_ context.Context, _ notify.OrderFailedNotification) error {
	return nil
}

func (n *wNotifier) NotifyAccessLink(_ context.Context, _ notify.AccessLinkNotification) error {
	return nil
}

var _ notify.Notifier = (*wNotifier)(nil)

type wLLM struct{}

func (l *wLLM) GenerateLyrics(_ context.Context, facts string) (string, error) { return facts, nil }

func (l *wLLM) GenerateLyricsVariants(_ context.Context, facts string, count int) ([]string, error) {
	out := make([]string, count)
	for i := range out {
		if i == 0 {
			out[i] = facts
		} else {
			out[i] = facts + " alt"
		}
	}
	return out, nil
}
