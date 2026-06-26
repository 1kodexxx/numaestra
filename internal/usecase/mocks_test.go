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

func intPtr(n int) *int { return &n }

// --- fake PaymentVerifier (Robokassa OpStateExt) ---

type fakeVerifier struct {
	kopecks int64
	paid    bool
	err     error
	calls   []int64
}

func (v *fakeVerifier) GetPaidAmountKopecks(_ context.Context, invID int64) (int64, bool, error) {
	v.calls = append(v.calls, invID)
	return v.kopecks, v.paid, v.err
}

// --- in-memory OrderRepository ---

type inMemOrderRepo struct {
	mu        sync.Mutex
	orders    map[uuid.UUID]domain.OrderSnapshot
	byInvoice map[int64]uuid.UUID
	seq       int64

	// Хуки инъекции ошибок для проверки путей сбоя.
	createErr              error
	updateErr              error
	applyPaymentErr        error
	applyPaymentNotApplied bool // возвращает (false, nil) вместо реального CAS
	countAllErr            error
	listAllErr             error
	stuckOrders            []*domain.Order
	listStuckErr           error
	stuckQueuedOrders      []*domain.Order
	listStuckQueuedErr     error
	pendingOrders          []*domain.Order
	listPendingErr         error
	stuckDemoOrders        []*domain.Order
	listStuckDemoErr       error
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

func (r *inMemOrderRepo) SetAdminFeedback(_ context.Context, id uuid.UUID, feedback string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	snap.AdminFeedback = feedback
	snap.AdminFeedbackAt = &at
	r.orders[id] = snap
	return nil
}

func (r *inMemOrderRepo) Update(_ context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
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
	if r.applyPaymentNotApplied {
		return false, nil
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

func (r *inMemOrderRepo) ListByCustomerEmail(_ context.Context, email string, _, _ int) ([]*domain.Order, error) {
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

func (r *inMemOrderRepo) ListByCustomerPhone(_ context.Context, phone string, _, _ int) ([]*domain.Order, error) {
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

func (r *inMemOrderRepo) ListAll(_ context.Context, limit, offset int) ([]*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listAllErr != nil {
		return nil, r.listAllErr
	}
	var out []*domain.Order
	i := 0
	for _, snap := range r.orders {
		if i >= offset && len(out) < limit {
			out = append(out, domain.RestoreOrder(snap))
		}
		i++
	}
	return out, nil
}

func (r *inMemOrderRepo) ListStuckProcessing(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	if r.listStuckErr != nil {
		return nil, r.listStuckErr
	}
	return r.stuckOrders, nil
}

func (r *inMemOrderRepo) ListStuckQueued(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	if r.listStuckQueuedErr != nil {
		return nil, r.listStuckQueuedErr
	}
	return r.stuckQueuedOrders, nil
}

func (r *inMemOrderRepo) ListPendingPayment(_ context.Context, _, _ time.Time) ([]*domain.Order, error) {
	if r.listPendingErr != nil {
		return nil, r.listPendingErr
	}
	return r.pendingOrders, nil
}

func (r *inMemOrderRepo) UpdateDemo(_ context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[order.ID()]; !ok {
		return domain.ErrOrderNotFound
	}
	r.save(order)
	return nil
}

func (r *inMemOrderRepo) ListStuckDemo(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	if r.listStuckDemoErr != nil {
		return nil, r.listStuckDemoErr
	}
	return r.stuckDemoOrders, nil
}

func (r *inMemOrderRepo) CountAll(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.countAllErr != nil {
		return 0, r.countAllErr
	}
	return len(r.orders), nil
}

func (r *inMemOrderRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	delete(r.orders, id)
	delete(r.byInvoice, snap.InvoiceID)
	return nil
}

var _ domain.OrderRepository = (*inMemOrderRepo)(nil)

// --- passthrough TransactionManager ---

// fakeTxManager выполняет fn без реальной транзакции: in-memory репозитории сами
// по себе атомарны, нам достаточно сохранить семантику оркестрации Unit of Work.
type fakeTxManager struct{}

func (fakeTxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

var _ TransactionManager = fakeTxManager{}

// --- in-memory AccountRepository ---

type inMemAccountRepo struct {
	mu       sync.Mutex
	accounts map[uuid.UUID]domain.SunoAccountSnapshot

	fetchErr  error
	noFree    bool
	createErr error
	updateErr error
	listErr   error
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
		if err := acc.AcquireSlot(time.Now().UTC()); err != nil {
			continue
		}
		r.accounts[acc.ID()] = acc.Snapshot()
		return acc, nil
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
	if r.createErr != nil {
		return r.createErr
	}
	r.add(account)
	return nil
}

func (r *inMemAccountRepo) Update(_ context.Context, account *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
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

func (r *inMemAccountRepo) List(_ context.Context) ([]*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]*domain.SunoAccount, 0, len(r.accounts))
	for _, snap := range r.accounts {
		out = append(out, domain.RestoreSunoAccount(snap))
	}
	return out, nil
}

func (r *inMemAccountRepo) SetStatus(_ context.Context, id uuid.UUID, status domain.AccountStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.accounts[id]
	if !ok {
		return domain.ErrAccountNotFound
	}
	snap.Status = status
	r.accounts[id] = snap
	return nil
}

func (r *inMemAccountRepo) ResetAccount(_ context.Context, id uuid.UUID, releaseSlots bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.accounts[id]
	if !ok {
		return domain.ErrAccountNotFound
	}
	snap.Status = domain.AccountStatusActive
	snap.FailureCount = 0
	snap.CooldownUntil = nil
	if releaseSlots {
		snap.ConcurrentTasks = 0
	}
	r.accounts[id] = snap
	return nil
}

// concurrentOf возвращает число занятых слотов аккаунта — используется тестами,
// чтобы проверить, что слот корректно освобождён после завершения/провала задачи.
func (r *inMemAccountRepo) concurrentOf(id uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accounts[id].ConcurrentTasks
}

var _ domain.AccountRepository = (*inMemAccountRepo)(nil)

// --- mock QueuePublisher ---

type mockQueue struct {
	mu              sync.Mutex
	genCalls        []uuid.UUID
	statusCalls     []statusCheckCall
	demoCalls       []uuid.UUID
	demoCheckCalls  []uuid.UUID
	enqueueGenErr   error
	enqueueCheckErr error
	enqueueDemoErr  error
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

func (q *mockQueue) EnqueueDemoTask(_ context.Context, orderID uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueDemoErr != nil {
		return q.enqueueDemoErr
	}
	q.demoCalls = append(q.demoCalls, orderID)
	return nil
}

func (q *mockQueue) EnqueueDemoCheckTask(_ context.Context, orderID uuid.UUID, _ string, _ uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.demoCheckCalls = append(q.demoCheckCalls, orderID)
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
	deleteFn func(ctx context.Context, orderID uuid.UUID) error
}

func (m *mockStorage) UploadFromURL(ctx context.Context, sourceURL, key, contentType string) (string, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, sourceURL, key, contentType)
	}
	return "https://s3.local/" + key, nil
}

func (m *mockStorage) DeleteOrderTracks(ctx context.Context, orderID uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, orderID)
	}
	return nil
}

var _ domain.TrackStorage = (*mockStorage)(nil)

// --- mock Notifier ---

type mockNotifier struct {
	mu            sync.Mutex
	calls         []notify.OrderCompleteNotification
	err           error
	feedbackCalls []notify.AdminFeedbackNotification
	feedbackErr   error
}

func (m *mockNotifier) NotifyOrderComplete(_ context.Context, n notify.OrderCompleteNotification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, n)
	return m.err
}

func (m *mockNotifier) NotifyAdminFeedback(_ context.Context, n notify.AdminFeedbackNotification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.feedbackCalls = append(m.feedbackCalls, n)
	return m.feedbackErr
}

func (m *mockNotifier) NotifyOrderFailed(_ context.Context, _ notify.OrderFailedNotification) error {
	return nil
}

func (m *mockNotifier) NotifyAccessLink(_ context.Context, _ notify.AccessLinkNotification) error {
	return m.err
}

var _ notify.Notifier = (*mockNotifier)(nil)

// --- mock LLM client ---

type mockLLM struct {
	fn         func(ctx context.Context, facts string) (string, error)
	variantsFn func(ctx context.Context, facts string, count int) ([]string, error)
}

func (m *mockLLM) GenerateLyrics(ctx context.Context, facts string) (string, error) {
	if m.fn != nil {
		return m.fn(ctx, facts)
	}
	return "сгенерированный текст для: " + facts, nil
}

func (m *mockLLM) GenerateLyricsVariants(ctx context.Context, facts string, count int) ([]string, error) {
	if m.variantsFn != nil {
		return m.variantsFn(ctx, facts, count)
	}
	if m.fn != nil {
		one, err := m.fn(ctx, facts)
		if err != nil {
			return nil, err
		}
		out := make([]string, count)
		for i := range out {
			out[i] = one
		}
		return out, nil
	}
	first := "[Verse 1]\nсгенерированный текст для: " + facts
	second := "[Verse 1]\nальтернативный текст для: " + facts
	if count <= 1 {
		return []string{first}, nil
	}
	return []string{first, second}, nil
}

var _ openai.APIClient = (*mockLLM)(nil)

// --- in-memory CategoryRepository ---

type inMemCategoryRepo struct {
	categories map[string]*domain.Category
	getErr     error
}

func newInMemCategoryRepo() *inMemCategoryRepo {
	return &inMemCategoryRepo{categories: make(map[string]*domain.Category)}
}

func (r *inMemCategoryRepo) GetByID(_ context.Context, id string) (*domain.Category, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	c, ok := r.categories[id]
	if !ok {
		return nil, domain.ErrCategoryNotFound
	}
	return c, nil
}

func (r *inMemCategoryRepo) GetAll(_ context.Context) ([]*domain.Category, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	out := make([]*domain.Category, 0, len(r.categories))
	for _, c := range r.categories {
		out = append(out, c)
	}
	return out, nil
}

func (r *inMemCategoryRepo) Create(_ context.Context, c *domain.Category) error {
	if _, ok := r.categories[c.ID()]; ok {
		return domain.ErrCategoryAlreadyExists
	}
	r.categories[c.ID()] = c
	return nil
}

func (r *inMemCategoryRepo) Update(_ context.Context, c *domain.Category) error {
	if _, ok := r.categories[c.ID()]; !ok {
		return domain.ErrCategoryNotFound
	}
	r.categories[c.ID()] = c
	return nil
}

func (r *inMemCategoryRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.categories[id]; !ok {
		return domain.ErrCategoryNotFound
	}
	delete(r.categories, id)
	return nil
}

func (r *inMemCategoryRepo) AddQuestion(_ context.Context, categoryID string, q domain.Question) (domain.Question, error) {
	c, ok := r.categories[categoryID]
	if !ok {
		return domain.Question{}, domain.ErrCategoryNotFound
	}
	q.ID = len(c.Questions()) + 1
	snap := c.Snapshot()
	snap.Questions = append(snap.Questions, q)
	r.categories[categoryID] = domain.RestoreCategory(snap)
	return q, nil
}

func (r *inMemCategoryRepo) UpdateQuestion(_ context.Context, categoryID string, q domain.Question) error {
	c, ok := r.categories[categoryID]
	if !ok {
		return domain.ErrCategoryNotFound
	}
	snap := c.Snapshot()
	for i, existing := range snap.Questions {
		if existing.ID == q.ID {
			snap.Questions[i] = q
			r.categories[categoryID] = domain.RestoreCategory(snap)
			return nil
		}
	}
	return domain.ErrQuestionNotFound
}

func (r *inMemCategoryRepo) DeleteQuestion(_ context.Context, categoryID string, questionID int) error {
	c, ok := r.categories[categoryID]
	if !ok {
		return domain.ErrCategoryNotFound
	}
	snap := c.Snapshot()
	for i, q := range snap.Questions {
		if q.ID == questionID {
			snap.Questions = append(snap.Questions[:i], snap.Questions[i+1:]...)
			r.categories[categoryID] = domain.RestoreCategory(snap)
			return nil
		}
	}
	return domain.ErrQuestionNotFound
}

var _ domain.CategoryRepository = (*inMemCategoryRepo)(nil)
