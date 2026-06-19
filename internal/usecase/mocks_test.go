package usecase

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/openai"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- in-memory OrderRepository ---

type inMemOrderRepo struct {
	mu        sync.Mutex
	orders    map[uuid.UUID]domain.OrderSnapshot
	byInvoice map[int64]uuid.UUID
	seq       int64

	// Хуки инъекции ошибок для проверки путей сбоя.
	createErr          error
	saveWithAccountErr error
	applyPaymentErr    error
}

func newInMemOrderRepo() *inMemOrderRepo {
	return &inMemOrderRepo{
		orders:    make(map[uuid.UUID]domain.OrderSnapshot),
		byInvoice: make(map[int64]uuid.UUID),
	}
}

func (r *inMemOrderRepo) save(o *domain.Order) {
	snap := o.Snapshot()
	r.orders[snap.ID] = snap
	r.byInvoice[snap.InvoiceID] = snap.ID
}

func (r *inMemOrderRepo) Create(_ context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	r.save(order)
	return nil
}

func (r *inMemOrderRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(snap), nil
}

func (r *inMemOrderRepo) GetByInvoiceID(_ context.Context, invoiceID int64) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byInvoice[invoiceID]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(r.orders[id]), nil
}

func (r *inMemOrderRepo) Update(_ context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[order.ID()]; !ok {
		return domain.ErrOrderNotFound
	}
	r.save(order)
	return nil
}

func (r *inMemOrderRepo) ApplyPaymentSuccess(_ context.Context, order *domain.Order) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.applyPaymentErr != nil {
		return false, r.applyPaymentErr
	}
	snap, ok := r.orders[order.ID()]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	// Условный переход: только из pending (имитация WHERE payment_status='pending').
	if snap.PaymentStatus != domain.PaymentStatusPending {
		return false, nil
	}
	r.save(order)
	return true, nil
}

func (r *inMemOrderRepo) ListByCustomerEmail(_ context.Context, email string) ([]*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Order
	for _, snap := range r.orders {
		if snap.CustomerEmail == email {
			out = append(out, domain.RestoreOrder(snap))
		}
	}
	return out, nil
}

func (r *inMemOrderRepo) ListByCustomerPhone(_ context.Context, phone string) ([]*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Order
	for _, snap := range r.orders {
		if snap.CustomerPhone == phone {
			out = append(out, domain.RestoreOrder(snap))
		}
	}
	return out, nil
}

func (r *inMemOrderRepo) SaveWithAccount(_ context.Context, order *domain.Order, account *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveWithAccountErr != nil {
		return r.saveWithAccountErr
	}
	if _, ok := r.orders[order.ID()]; !ok {
		return domain.ErrOrderNotFound
	}
	r.save(order)
	accSaveSnapshot(account)
	return nil
}

func (r *inMemOrderRepo) NextInvoiceID(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return r.seq, nil
}

func (r *inMemOrderRepo) GetByAccessToken(_ context.Context, token string) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, snap := range r.orders {
		if snap.AccessToken == token {
			return domain.RestoreOrder(snap), nil
		}
	}
	return nil, domain.ErrOrderNotFound
}

var _ domain.OrderRepository = (*inMemOrderRepo)(nil)

// accSaveSnapshot — placeholder, чтобы зафиксировать, что SaveWithAccount
// сохраняет и аккаунт; реальное состояние аккаунта проверяется через accRepo.
func accSaveSnapshot(_ *domain.SunoAccount) {}

// --- in-memory AccountRepository ---

type inMemAccountRepo struct {
	mu       sync.Mutex
	accounts map[uuid.UUID]domain.SunoAccountSnapshot

	fetchErr error
	noFree   bool
}

func newInMemAccountRepo() *inMemAccountRepo {
	return &inMemAccountRepo{accounts: make(map[uuid.UUID]domain.SunoAccountSnapshot)}
}

func (r *inMemAccountRepo) add(a *domain.SunoAccount) {
	r.accounts[a.ID()] = a.Snapshot()
}

func (r *inMemAccountRepo) FetchAndLockAvailable(_ context.Context) (*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fetchErr != nil {
		return nil, r.fetchErr
	}
	if r.noFree {
		return nil, domain.ErrNoAvailableAccount
	}
	for _, snap := range r.accounts {
		acc := domain.RestoreSunoAccount(snap)
		if acc.IsAvailable(time.Now().UTC()) {
			if err := acc.MarkBusy(); err != nil {
				continue
			}
			r.accounts[acc.ID()] = acc.Snapshot()
			return acc, nil
		}
	}
	return nil, domain.ErrNoAvailableAccount
}

func (r *inMemAccountRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.accounts[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return domain.RestoreSunoAccount(snap), nil
}

func (r *inMemAccountRepo) Create(_ context.Context, account *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.add(account)
	return nil
}

func (r *inMemAccountRepo) Update(_ context.Context, account *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accounts[account.ID()] = account.Snapshot()
	return nil
}

func (r *inMemAccountRepo) ListByStatus(_ context.Context, status domain.AccountStatus) ([]*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.SunoAccount
	for _, snap := range r.accounts {
		if snap.Status == status {
			out = append(out, domain.RestoreSunoAccount(snap))
		}
	}
	return out, nil
}

func (r *inMemAccountRepo) statusOf(id uuid.UUID) domain.AccountStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accounts[id].Status
}

var _ domain.AccountRepository = (*inMemAccountRepo)(nil)

// --- mock QueuePublisher ---

type mockQueue struct {
	mu              sync.Mutex
	genCalls        []uuid.UUID
	statusCalls     []statusCheckCall
	enqueueGenErr   error
	enqueueCheckErr error
}

type statusCheckCall struct {
	OrderID   uuid.UUID
	SunoJobID string
}

func (q *mockQueue) EnqueueGenerationTask(_ context.Context, orderID uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueGenErr != nil {
		return q.enqueueGenErr
	}
	q.genCalls = append(q.genCalls, orderID)
	return nil
}

func (q *mockQueue) EnqueueStatusCheckTask(_ context.Context, orderID uuid.UUID, sunoJobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueCheckErr != nil {
		return q.enqueueCheckErr
	}
	q.statusCalls = append(q.statusCalls, statusCheckCall{OrderID: orderID, SunoJobID: sunoJobID})
	return nil
}

var _ domain.QueuePublisher = (*mockQueue)(nil)

// --- mock MusicProvider ---

type mockProvider struct {
	submitFn func(ctx context.Context, req domain.MusicGenerationRequest) (string, error)
	fetchFn  func(ctx context.Context, jobID string) (domain.MusicGenerationResult, error)
}

func (m *mockProvider) SubmitGeneration(ctx context.Context, req domain.MusicGenerationRequest) (string, error) {
	return m.submitFn(ctx, req)
}

func (m *mockProvider) FetchResult(ctx context.Context, jobID string) (domain.MusicGenerationResult, error) {
	return m.fetchFn(ctx, jobID)
}

var _ domain.MusicProvider = (*mockProvider)(nil)

// --- mock TrackStorage ---

type mockStorage struct {
	uploadFn func(ctx context.Context, sourceURL, key, contentType string) (string, error)
}

func (m *mockStorage) UploadFromURL(ctx context.Context, sourceURL, key, contentType string) (string, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, sourceURL, key, contentType)
	}
	return "https://s3.local/" + key, nil
}

var _ domain.TrackStorage = (*mockStorage)(nil)

// --- mock Notifier ---

type mockNotifier struct {
	mu    sync.Mutex
	calls []notify.OrderCompleteNotification
	err   error
}

func (m *mockNotifier) NotifyOrderComplete(_ context.Context, n notify.OrderCompleteNotification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, n)
	return m.err
}

var _ notify.Notifier = (*mockNotifier)(nil)

// --- mock LLM client ---

type mockLLM struct {
	fn func(ctx context.Context, facts string) (string, error)
}

func (m *mockLLM) GenerateLyrics(ctx context.Context, facts string) (string, error) {
	if m.fn != nil {
		return m.fn(ctx, facts)
	}
	return "сгенерированный текст для: " + facts, nil
}

var _ openai.APIClient = (*mockLLM)(nil)
