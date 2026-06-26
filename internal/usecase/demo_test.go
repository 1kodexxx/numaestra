package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

func demoOrder(t *testing.T, f *fixture) *domain.Order {
	t.Helper()
	order, err := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф про маму", "", domain.CurrentConsentDocVersion, "", "", nil)
	if err != nil {
		t.Fatalf("подготовка заказа: %v", err)
	}
	return order
}

// GenerateDemo для pending-заказа: захватывает слот, сабмитит 1 задачу, ставит опрос.
func TestGenerateDemo_Success(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 5)
	order := demoOrder(t, f)

	var gotReq domain.MusicGenerationRequest
	f.provider.submitFn = func(_ context.Context, req domain.MusicGenerationRequest) (string, error) {
		gotReq = req
		return "demo-job", nil
	}

	if err := f.uc.GenerateDemo(context.Background(), order.ID()); err != nil {
		t.Fatalf("GenerateDemo: %v", err)
	}

	// Демо ровно одна задача Suno (1 клип), не 4.
	if gotReq.TrackCount != 1 {
		t.Errorf("ожидали TrackCount=1 для демо, получили %d", gotReq.TrackCount)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.DemoStatus() != domain.DemoStatusProcessing {
		t.Errorf("ожидали demo processing, получили %q", got.DemoStatus())
	}
	if got.DemoAccountID() == nil || *got.DemoAccountID() != acc.ID() {
		t.Errorf("demo_account_id должен указывать на захваченный аккаунт")
	}
	// Слот аккаунта удержан на время демо.
	accNow, _ := f.accRepo.GetByID(context.Background(), acc.ID())
	if accNow.ConcurrentTasks() != 1 {
		t.Errorf("ожидали удержанный слот (concurrent=1), получили %d", accNow.ConcurrentTasks())
	}
	// Платёжный статус НЕ затронут демо.
	if got.PaymentStatus() != domain.PaymentStatusPending {
		t.Errorf("демо не должно менять payment_status, получили %q", got.PaymentStatus())
	}
	f.queue.mu.Lock()
	defer f.queue.mu.Unlock()
	if len(f.queue.demoCheckCalls) != 1 {
		t.Errorf("ожидали постановку опроса демо, получили %v", f.queue.demoCheckCalls)
	}
}

// Оплаченному заказу демо не нужно — GenerateDemo выходит без захвата аккаунта.
func TestGenerateDemo_PaidOrder_Skips(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 5)
	order := f.queuedOrder(t) // paid + queued

	called := false
	f.provider.submitFn = func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		called = true
		return "x", nil
	}

	if err := f.uc.GenerateDemo(context.Background(), order.ID()); err != nil {
		t.Fatalf("GenerateDemo: %v", err)
	}
	if called {
		t.Error("для оплаченного заказа демо не должно сабмититься")
	}
	accNow, _ := f.accRepo.GetByID(context.Background(), acc.ID())
	if accNow.ConcurrentTasks() != 0 {
		t.Error("слот аккаунта не должен захватываться для оплаченного заказа")
	}
}

// Нет свободных аккаунтов → демо не отбирает у платных, возвращает ErrNoAvailableAccount.
func TestGenerateDemo_NoAccount_LeavesNone(t *testing.T) {
	f := newFixture(t)
	f.accRepo.noFree = true
	order := demoOrder(t, f)

	err := f.uc.GenerateDemo(context.Background(), order.ID())
	if !errors.Is(err, domain.ErrNoAvailableAccount) {
		t.Errorf("ожидали ErrNoAvailableAccount, получили %v", err)
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.DemoStatus() != domain.DemoStatusNone {
		t.Errorf("без аккаунта демо должно остаться none, получили %q", got.DemoStatus())
	}
}

// CheckDemoStatus при готовности: демо ready + URL, слот освобождён, токен списан.
func TestCheckDemoStatus_Completed_ReadyAndReleasesSlot(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 5)
	order := demoOrder(t, f)
	f.provider.submitFn = func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		return "demo-job", nil
	}
	if err := f.uc.GenerateDemo(context.Background(), order.ID()); err != nil {
		t.Fatalf("GenerateDemo: %v", err)
	}

	f.provider.fetchFn = func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{
			Status: domain.MusicGenerationStatusCompleted,
			Tracks: []domain.ProviderTrack{{SourceURL: "http://suno/clip", DurationSec: 30, ExternalID: "c1"}},
		}, nil
	}

	if err := f.uc.CheckDemoStatus(context.Background(), order.ID(), "demo-job", acc.ID()); err != nil {
		t.Fatalf("CheckDemoStatus: %v", err)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.DemoStatus() != domain.DemoStatusReady {
		t.Errorf("ожидали demo ready, получили %q", got.DemoStatus())
	}
	if got.DemoURL() == "" {
		t.Error("demo_url должен быть заполнен")
	}
	if got.DemoAccountID() != nil {
		t.Error("demo_account_id должен быть очищен после завершения")
	}
	accNow, _ := f.accRepo.GetByID(context.Background(), acc.ID())
	if accNow.ConcurrentTasks() != 0 {
		t.Errorf("слот должен быть освобождён, concurrent=%d", accNow.ConcurrentTasks())
	}
	if accNow.TokenBalance() != 4 {
		t.Errorf("ожидали списание 1 токена (5→4), получили %d", accNow.TokenBalance())
	}
}

// Пока Suno генерит — ErrGenerationNotReady (Asynq ретраит), слот удержан.
func TestCheckDemoStatus_Running_NotReady(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 5)
	order := demoOrder(t, f)
	f.provider.submitFn = func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		return "demo-job", nil
	}
	_ = f.uc.GenerateDemo(context.Background(), order.ID())

	f.provider.fetchFn = func(_ context.Context, _ string) (domain.MusicGenerationResult, error) {
		return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusRunning}, nil
	}
	err := f.uc.CheckDemoStatus(context.Background(), order.ID(), "demo-job", acc.ID())
	if !errors.Is(err, ErrGenerationNotReady) {
		t.Errorf("ожидали ErrGenerationNotReady, получили %v", err)
	}
	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.DemoStatus() != domain.DemoStatusProcessing {
		t.Errorf("во время генерации демо остаётся processing, получили %q", got.DemoStatus())
	}
}

// RecoverStuckDemos освобождает слот застрявшего демо и помечает его failed.
func TestRecoverStuckDemos_ReleasesSlotAndFails(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(t, 5)
	order := demoOrder(t, f)
	f.provider.submitFn = func(_ context.Context, _ domain.MusicGenerationRequest) (string, error) {
		return "demo-job", nil
	}
	_ = f.uc.GenerateDemo(context.Background(), order.ID())

	// Эмулируем «застрявшее» демо: оно в processing со слотом, но воркер «умер».
	stuck, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	f.orderRepo.stuckDemoOrders = []*domain.Order{stuck}

	if err := f.uc.RecoverStuckDemos(context.Background()); err != nil {
		t.Fatalf("RecoverStuckDemos: %v", err)
	}

	got, _ := f.orderRepo.GetByID(context.Background(), order.ID())
	if got.DemoStatus() != domain.DemoStatusFailed {
		t.Errorf("ожидали demo failed после recovery, получили %q", got.DemoStatus())
	}
	accNow, _ := f.accRepo.GetByID(context.Background(), acc.ID())
	if accNow.ConcurrentTasks() != 0 {
		t.Errorf("recovery должен освободить слот, concurrent=%d", accNow.ConcurrentTasks())
	}
}

// TriggerDemo ставит фоновую задачу демо.
func TestTriggerDemo_Enqueues(t *testing.T) {
	f := newFixture(t)
	order := demoOrder(t, f)
	if err := f.uc.TriggerDemo(context.Background(), order.ID()); err != nil {
		t.Fatalf("TriggerDemo: %v", err)
	}
	f.queue.mu.Lock()
	defer f.queue.mu.Unlock()
	if len(f.queue.demoCalls) != 1 || f.queue.demoCalls[0] != order.ID() {
		t.Errorf("ожидали постановку демо-задачи для %s, получили %v", order.ID(), f.queue.demoCalls)
	}
}
