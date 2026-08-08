package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

// Цены сценария платного демо: заказ 990 ₽ = демо 50 ₽ + доплата 940 ₽.
const (
	demoTestPrice      int64 = 5000
	demoTestOrderPrice int64 = 99000
	demoTestRemaining  int64 = demoTestOrderPrice - demoTestPrice
)

// newPaidDemoFixture — фикстура с ценой заказа 990 ₽ и платным демо 50 ₽.
func newPaidDemoFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.uc.priceKopecks = demoTestOrderPrice
	f.uc.WithDemoPrice(demoTestPrice)
	return f
}

// createDemoOrder создаёт заказ через use case — вместе со вторым счётом на демо.
func createDemoOrder(t *testing.T, f *fixture) *domain.Order {
	t.Helper()
	order, err := f.uc.CreateOrder(context.Background(), "user@example.com", "", "Песня на ДР", "", domain.CurrentConsentDocVersion, "", "", nil)
	if err != nil {
		t.Fatalf("CreateOrder упал: %v", err)
	}
	return order
}

func TestCreateOrder_ВыставляетОтдельныйСчётНаДемо(t *testing.T) {
	f := newPaidDemoFixture(t)
	order := createDemoOrder(t, f)

	if !order.HasDemoPayment() {
		t.Fatal("у заказа должен быть счёт на демо")
	}
	if order.DemoInvoiceID() == order.InvoiceID() {
		t.Errorf("InvId демо и заказа совпали (%d) — вебхук не сможет их различить", order.InvoiceID())
	}
	if got := order.DemoAmountKopecks(); got != demoTestPrice {
		t.Errorf("цена демо = %d, ожидалось %d", got, demoTestPrice)
	}
	if got := order.DemoPaymentStatus(); got != domain.PaymentStatusPending {
		t.Errorf("статус оплаты демо = %q, ожидался pending", got)
	}
	// Пока демо не оплачено, к оплате стоит полная сумма: заказ остаётся
	// оплачиваемым одним платежом, если клиент пропустил шаг демо.
	if got := order.RemainingKopecks(); got != demoTestOrderPrice {
		t.Errorf("остаток до оплаты демо = %d, ожидалось %d", got, demoTestOrderPrice)
	}
}

func TestCreateOrder_БезЦеныДемоВторойСчётНеВыставляется(t *testing.T) {
	f := newFixture(t) // demoPriceKopecks не задан
	order := createDemoOrder(t, f)

	if order.HasDemoPayment() {
		t.Error("при нулевой цене демо второй счёт выставляться не должен")
	}
	if got := order.RemainingKopecks(); got != order.AmountKopecks() {
		t.Errorf("остаток = %d, ожидалась полная сумма %d", got, order.AmountKopecks())
	}
}

// Заказ дешевле демо (щедрый промокод) должен идти одним платежом: иначе после
// зачёта к доплате осталось бы 0 и генерацию песни было бы нечем запустить.
func TestCreateOrder_ЗаказДешевлеДемоИдётОднимПлатежом(t *testing.T) {
	f := newFixture(t)
	f.uc.priceKopecks = demoTestPrice // цена заказа равна цене демо
	f.uc.WithDemoPrice(demoTestPrice)

	order := createDemoOrder(t, f)
	if order.HasDemoPayment() {
		t.Error("счёт на демо не должен выставляться, когда заказ не дороже демо")
	}
}

func TestHandleDemoPaymentSuccess_ПомечаетОплаченнымИСтавитЗадачу(t *testing.T) {
	f := newPaidDemoFixture(t)
	order := createDemoOrder(t, f)

	if err := f.uc.HandleDemoPaymentSuccess(context.Background(), order.DemoInvoiceID(), demoTestPrice); err != nil {
		t.Fatalf("HandleDemoPaymentSuccess упал: %v", err)
	}

	stored, err := f.uc.GetOrder(context.Background(), order.ID())
	if err != nil {
		t.Fatalf("получение заказа: %v", err)
	}
	if !stored.DemoPaid() {
		t.Error("демо должно быть помечено оплаченным")
	}
	// Основная полоса не тронута: заказ ждёт доплаты за песню.
	if stored.PaymentStatus() != domain.PaymentStatusPending {
		t.Errorf("оплата заказа = %q, ожидался pending", stored.PaymentStatus())
	}
	if got := stored.RemainingKopecks(); got != demoTestRemaining {
		t.Errorf("остаток после оплаты демо = %d, ожидалось %d", got, demoTestRemaining)
	}
	if len(f.queue.demoCalls) != 1 {
		t.Fatalf("задач демо поставлено %d, ожидалась 1", len(f.queue.demoCalls))
	}
	if len(f.queue.genCalls) != 0 {
		t.Errorf("оплата демо не должна запускать генерацию песни (поставлено %d задач)", len(f.queue.genCalls))
	}
}

// Robokassa повторяет доставку до получения OK — повторный вебхук обязан быть
// успехом и не ставить вторую задачу (иначе двойной расход кредитов Suno).
func TestHandleDemoPaymentSuccess_Идемпотентен(t *testing.T) {
	f := newPaidDemoFixture(t)
	order := createDemoOrder(t, f)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := f.uc.HandleDemoPaymentSuccess(ctx, order.DemoInvoiceID(), demoTestPrice); err != nil {
			t.Fatalf("доставка №%d вернула ошибку: %v", i+1, err)
		}
	}
	if len(f.queue.demoCalls) != 1 {
		t.Errorf("задач демо поставлено %d, ожидалась ровно 1", len(f.queue.demoCalls))
	}
}

func TestHandleDemoPaymentSuccess_ОтклоняетЧужуюСумму(t *testing.T) {
	f := newPaidDemoFixture(t)
	order := createDemoOrder(t, f)

	err := f.uc.HandleDemoPaymentSuccess(context.Background(), order.DemoInvoiceID(), 100)
	if !errors.Is(err, ErrPaymentAmountMismatch) {
		t.Fatalf("ожидалась ErrPaymentAmountMismatch, получено: %v", err)
	}
	if len(f.queue.demoCalls) != 0 {
		t.Error("при несовпадении суммы задача демо ставиться не должна")
	}
}

// Ключевой барьер расхода: неоплаченное демо не тратит кредиты Suno ни на одном
// из путей запуска задачи (вебхук, retry Asynq, recovery-крон).
func TestGenerateDemo_НеЗапускаетсяБезОплаты(t *testing.T) {
	f := newPaidDemoFixture(t)
	f.addAccount(t, 100)
	order := createDemoOrder(t, f)

	submits := 0
	f.provider.submitFn = func(context.Context, domain.MusicGenerationRequest) (string, error) {
		submits++
		return "job-1", nil
	}

	if err := f.uc.GenerateDemo(context.Background(), order.ID()); err != nil {
		t.Fatalf("GenerateDemo должен тихо выйти, а не падать: %v", err)
	}

	stored, _ := f.uc.GetOrder(context.Background(), order.ID())
	if stored.DemoStatus() != domain.DemoStatusNone {
		t.Errorf("статус демо = %q, ожидался none — генерация не должна была начаться", stored.DemoStatus())
	}
	if submits != 0 {
		t.Errorf("провайдер вызван %d раз — кредиты Suno потрачены на неоплаченное демо", submits)
	}
}

// Зеркало предыдущего теста: после оплаты демо тот же вызов доходит до провайдера.
func TestGenerateDemo_ЗапускаетсяПослеОплаты(t *testing.T) {
	f := newPaidDemoFixture(t)
	f.addAccount(t, 100)
	order := createDemoOrder(t, f)
	ctx := context.Background()

	submits := 0
	f.provider.submitFn = func(context.Context, domain.MusicGenerationRequest) (string, error) {
		submits++
		return "job-1", nil
	}

	if err := f.uc.HandleDemoPaymentSuccess(ctx, order.DemoInvoiceID(), demoTestPrice); err != nil {
		t.Fatalf("оплата демо: %v", err)
	}
	if err := f.uc.GenerateDemo(ctx, order.ID()); err != nil {
		t.Fatalf("GenerateDemo упал: %v", err)
	}

	if submits != 1 {
		t.Errorf("провайдер вызван %d раз, ожидался 1", submits)
	}
	stored, _ := f.uc.GetOrder(ctx, order.ID())
	if stored.DemoStatus() != domain.DemoStatusProcessing {
		t.Errorf("статус демо = %q, ожидался processing", stored.DemoStatus())
	}
}

// Счёт за песню выставляется на остаток, поэтому вебхук должен принимать именно
// его; полная сумма после зачёта демо означала бы двойную оплату 50 ₽.
func TestHandlePaymentSuccess_ПослеОплатыДемоЖдётОстаток(t *testing.T) {
	f := newPaidDemoFixture(t)
	order := createDemoOrder(t, f)
	ctx := context.Background()

	if err := f.uc.HandleDemoPaymentSuccess(ctx, order.DemoInvoiceID(), demoTestPrice); err != nil {
		t.Fatalf("оплата демо: %v", err)
	}

	// Полная сумма больше не ожидается.
	err := f.uc.HandlePaymentSuccess(ctx, order.InvoiceID(), demoTestOrderPrice)
	if !errors.Is(err, ErrPaymentAmountMismatch) {
		t.Fatalf("полная сумма должна отклоняться, получено: %v", err)
	}

	// Остаток принимается и запускает генерацию.
	if err := f.uc.HandlePaymentSuccess(ctx, order.InvoiceID(), demoTestRemaining); err != nil {
		t.Fatalf("доплата остатка упала: %v", err)
	}
	stored, _ := f.uc.GetOrder(ctx, order.ID())
	if stored.PaymentStatus() != domain.PaymentStatusPaid {
		t.Errorf("статус оплаты = %q, ожидался paid", stored.PaymentStatus())
	}
	if len(f.queue.genCalls) != 1 {
		t.Errorf("задач генерации поставлено %d, ожидалась 1", len(f.queue.genCalls))
	}
}

// Клиент мог пропустить шаг демо и оплатить песню целиком — тогда ожидается
// полная сумма, а зачитывать нечего.
func TestHandlePaymentSuccess_БезОплатыДемоЖдётПолнуюСумму(t *testing.T) {
	f := newPaidDemoFixture(t)
	order := createDemoOrder(t, f)
	ctx := context.Background()

	if err := f.uc.HandlePaymentSuccess(ctx, order.InvoiceID(), demoTestOrderPrice); err != nil {
		t.Fatalf("оплата полной суммы упала: %v", err)
	}
	stored, _ := f.uc.GetOrder(ctx, order.ID())
	if stored.PaymentStatus() != domain.PaymentStatusPaid {
		t.Errorf("статус оплаты = %q, ожидался paid", stored.PaymentStatus())
	}
}

func TestResolveInvoice_РазличаетПлатёжныеПолосы(t *testing.T) {
	f := newPaidDemoFixture(t)
	order := createDemoOrder(t, f)
	ctx := context.Background()

	got, kind, err := f.uc.ResolveInvoice(ctx, order.InvoiceID())
	if err != nil {
		t.Fatalf("резолв основного счёта: %v", err)
	}
	if kind != InvoiceKindMain || got.ID() != order.ID() {
		t.Errorf("основной счёт: kind=%q id=%v, ожидалось main / %v", kind, got.ID(), order.ID())
	}

	got, kind, err = f.uc.ResolveInvoice(ctx, order.DemoInvoiceID())
	if err != nil {
		t.Fatalf("резолв счёта демо: %v", err)
	}
	if kind != InvoiceKindDemo || got.ID() != order.ID() {
		t.Errorf("счёт демо: kind=%q id=%v, ожидалось demo / %v", kind, got.ID(), order.ID())
	}

	if _, _, err := f.uc.ResolveInvoice(ctx, 999999); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("несуществующий InvId: ожидалась ErrOrderNotFound, получено: %v", err)
	}
}
