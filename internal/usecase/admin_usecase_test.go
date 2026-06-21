package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
)

// --- mock Refunder ---

type mockRefunder struct {
	err error
}

func (m *mockRefunder) Refund(_ context.Context, _ string, _ int64) error {
	return m.err
}

var _ Refunder = (*mockRefunder)(nil)

func newAdminUC(t *testing.T) (*AdminUseCase, *inMemOrderRepo, *inMemAccountRepo) {
	t.Helper()
	orders := newInMemOrderRepo()
	accounts := newInMemAccountRepo()
	uc := NewAdminUseCase(orders, accounts, nil, &mockRefunder{}, nil, testLogger())
	return uc, orders, accounts
}

// --- AddAccount ---

func TestAdminUseCase_AddAccount_CreatesAccount(t *testing.T) {
	uc, _, accounts := newAdminUC(t)

	acc, err := uc.AddAccount(context.Background(), "test@suno.com", "enc-session", 3)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if acc.Email() != "test@suno.com" {
		t.Errorf("ожидали email=test@suno.com, получили %q", acc.Email())
	}
	if acc.MaxConcurrentTasks() != 3 {
		t.Errorf("ожидали MaxConcurrentTasks=3, получили %d", acc.MaxConcurrentTasks())
	}
	// Должен быть сохранён в репозитории.
	saved, err := accounts.GetByID(context.Background(), acc.ID())
	if err != nil {
		t.Fatalf("аккаунт не сохранён в репозитории: %v", err)
	}
	if saved.Email() != acc.Email() {
		t.Error("сохранённый аккаунт не совпадает с созданным")
	}
}

func TestAdminUseCase_AddAccount_DefaultMaxConcurrent(t *testing.T) {
	uc, _, _ := newAdminUC(t)

	// maxConcurrent <= 1 → не вызывает SetMaxConcurrentTasks → остаётся дефолт.
	acc, err := uc.AddAccount(context.Background(), "a@b.com", "sess", 0)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if acc.MaxConcurrentTasks() != domain.DefaultMaxConcurrentTasks {
		t.Errorf("ожидали дефолт %d, получили %d",
			domain.DefaultMaxConcurrentTasks, acc.MaxConcurrentTasks())
	}
}

// --- ListAccounts ---

func TestAdminUseCase_ListAccounts_ReturnsAll(t *testing.T) {
	uc, _, accounts := newAdminUC(t)

	for i := 0; i < 3; i++ {
		acc, _ := domain.NewSunoAccount("u@b.com", "s", 10)
		_ = accounts.Create(context.Background(), acc)
	}

	list, err := uc.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ожидали 3 аккаунта, получили %d", len(list))
	}
}

// --- SetAccountStatus ---

func TestAdminUseCase_SetAccountStatus_Valid(t *testing.T) {
	uc, _, accounts := newAdminUC(t)

	acc, _ := domain.NewSunoAccount("x@y.com", "s", 5)
	_ = accounts.Create(context.Background(), acc)

	if err := uc.SetAccountStatus(context.Background(), acc.ID(), domain.AccountStatusBanned); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}

func TestAdminUseCase_SetAccountStatus_InvalidStatus(t *testing.T) {
	uc, _, _ := newAdminUC(t)

	err := uc.SetAccountStatus(context.Background(), uuid.New(), domain.AccountStatus("invalid"))
	if err == nil {
		t.Fatal("ожидали ошибку для недопустимого статуса")
	}
}

// --- ListOrders ---

func TestAdminUseCase_ListOrders_Pagination(t *testing.T) {
	uc, orders, _ := newAdminUC(t)

	for i := int64(1); i <= 5; i++ {
		o, _ := domain.NewOrder(i, "u@a.com", "", "бриф", "", "", 100)
		_ = orders.Create(context.Background(), o)
	}

	// page=1, perPage=3 → 3 заказа, total=5
	list, total, err := uc.ListOrders(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if total != 5 {
		t.Errorf("ожидали total=5, получили %d", total)
	}
	if len(list) != 3 {
		t.Errorf("ожидали 3 заказа на странице, получили %d", len(list))
	}
}

func TestAdminUseCase_ListOrders_DefaultsOnInvalidParams(t *testing.T) {
	uc, _, _ := newAdminUC(t)

	// page=0, perPage=200 → нормализуются до 1 и 20
	_, _, err := uc.ListOrders(context.Background(), 0, 200)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}

// --- GetOrder ---

func TestAdminUseCase_GetOrder_Found(t *testing.T) {
	uc, orders, _ := newAdminUC(t)

	o, _ := domain.NewOrder(1, "a@b.com", "", "бриф", "", "", 500)
	_ = orders.Create(context.Background(), o)

	got, err := uc.GetOrder(context.Background(), o.ID())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got.ID() != o.ID() {
		t.Error("получен не тот заказ")
	}
}

func TestAdminUseCase_GetOrder_NotFound(t *testing.T) {
	uc, _, _ := newAdminUC(t)

	_, err := uc.GetOrder(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("ожидали ErrOrderNotFound, получили %v", err)
	}
}

// --- RefundOrder ---

func TestAdminUseCase_RefundOrder_Success(t *testing.T) {
	orders := newInMemOrderRepo()
	accounts := newInMemAccountRepo()
	rk := &mockRefunder{}
	uc := NewAdminUseCase(orders, accounts, nil, rk, nil, testLogger())

	o, _ := domain.NewOrder(1, "c@d.com", "", "бриф", "", "", 10000)
	_ = o.MarkPaid()
	_ = orders.Create(context.Background(), o)

	if err := uc.RefundOrder(context.Background(), o.ID()); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	updated, _ := orders.GetByID(context.Background(), o.ID())
	if updated.PaymentStatus() != domain.PaymentStatusRefunded {
		t.Errorf("ожидали статус refunded, получили %q", updated.PaymentStatus())
	}
}

func TestAdminUseCase_RefundOrder_NotPaid(t *testing.T) {
	uc, orders, _ := newAdminUC(t)

	o, _ := domain.NewOrder(1, "c@d.com", "", "бриф", "", "", 100)
	// Заказ не оплачен (статус pending).
	_ = orders.Create(context.Background(), o)

	err := uc.RefundOrder(context.Background(), o.ID())
	if err == nil {
		t.Fatal("ожидали ошибку для неоплаченного заказа")
	}
}

// --- AddAccount repo error ---

func TestAdminUseCase_AddAccount_RepoError(t *testing.T) {
	uc, _, accounts := newAdminUC(t)
	accounts.createErr = errors.New("db down")

	_, err := uc.AddAccount(context.Background(), "x@y.com", "sess", 1)
	if err == nil {
		t.Fatal("ожидали ошибку при сбое Create")
	}
}

// --- ListAccounts repo error ---

func TestAdminUseCase_ListAccounts_RepoError(t *testing.T) {
	uc, _, accounts := newAdminUC(t)
	accounts.listErr = errors.New("db down")

	_, err := uc.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("ожидали ошибку при сбое List")
	}
}

// --- SetAccountStatus not found ---

func TestAdminUseCase_SetAccountStatus_NotFound(t *testing.T) {
	uc, _, _ := newAdminUC(t)

	err := uc.SetAccountStatus(context.Background(), uuid.New(), domain.AccountStatusActive)
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("ожидали ErrAccountNotFound, получили %v", err)
	}
}

// --- ListOrders repo errors ---

func TestAdminUseCase_ListOrders_CountAllError(t *testing.T) {
	uc, orders, _ := newAdminUC(t)
	orders.countAllErr = errors.New("db down")

	_, _, err := uc.ListOrders(context.Background(), 1, 10)
	if err == nil {
		t.Fatal("ожидали ошибку при сбое CountAll")
	}
}

func TestAdminUseCase_ListOrders_ListAllError(t *testing.T) {
	uc, orders, _ := newAdminUC(t)
	orders.listAllErr = errors.New("db down")

	_, _, err := uc.ListOrders(context.Background(), 1, 10)
	if err == nil {
		t.Fatal("ожидали ошибку при сбое ListAll")
	}
}

// --- RefundOrder extra paths ---

func TestAdminUseCase_RefundOrder_NotFound(t *testing.T) {
	uc, _, _ := newAdminUC(t)

	err := uc.RefundOrder(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("ожидали ErrOrderNotFound, получили %v", err)
	}
}

func TestAdminUseCase_RefundOrder_UpdateError(t *testing.T) {
	uc, orders, _ := newAdminUC(t)

	o, _ := domain.NewOrder(1, "c@d.com", "", "бриф", "", "", 10000)
	_ = o.MarkPaid()
	_ = orders.Create(context.Background(), o)

	orders.updateErr = errors.New("db down")

	err := uc.RefundOrder(context.Background(), o.ID())
	if err == nil {
		t.Fatal("ожидали ошибку при сбое Update после возврата")
	}
}

func TestAdminUseCase_RefundOrder_RobokassaFailure(t *testing.T) {
	orders := newInMemOrderRepo()
	accounts := newInMemAccountRepo()
	rk := &mockRefunder{err: errors.New("Robokassa 500")}
	uc := NewAdminUseCase(orders, accounts, nil, rk, nil, testLogger())

	o, _ := domain.NewOrder(1, "c@d.com", "", "бриф", "", "", 10000)
	_ = o.MarkPaid()
	_ = orders.Create(context.Background(), o)

	err := uc.RefundOrder(context.Background(), o.ID())
	if err == nil {
		t.Fatal("ожидали ошибку при сбое Robokassa")
	}

	// Статус заказа не должен был поменяться.
	unchanged, _ := orders.GetByID(context.Background(), o.ID())
	if unchanged.PaymentStatus() != domain.PaymentStatusPaid {
		t.Errorf("статус заказа не должен меняться при сбое Robokassa, получили %q",
			unchanged.PaymentStatus())
	}
}
