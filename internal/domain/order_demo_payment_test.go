package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

// Цены сценария платного демо: заказ 990 ₽ = демо 50 ₽ + доплата 940 ₽.
const (
	demoPrice     int64 = 5000
	orderPrice    int64 = 99000
	remainingSum  int64 = orderPrice - demoPrice
	demoInvoiceNo int64 = 4243
)

// makeDemoOrder — заказ на 990 ₽ с выставленным счётом на демо.
func makeDemoOrder(t *testing.T) *Order {
	t.Helper()
	o, err := NewOrder(4242, "test@example.com", "", "Бриф", "", "", CurrentConsentDocVersion, orderPrice)
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	o.SetDemoPayment(demoInvoiceNo, demoPrice)
	return o
}

func TestNewOrder_ДемоПлатежаПоУмолчаниюНет(t *testing.T) {
	o := makeOrder(t)

	if o.HasDemoPayment() {
		t.Error("у свежего заказа не должно быть счёта на демо, пока его не выставили")
	}
	if o.DemoPaid() {
		t.Error("демо не может быть оплачено до выставления счёта")
	}
	if got := o.DemoPaymentStatus(); got != PaymentStatusPending {
		t.Errorf("DemoPaymentStatus = %q, ожидался pending", got)
	}
	if got := o.RemainingKopecks(); got != o.AmountKopecks() {
		t.Errorf("RemainingKopecks = %d, без демо ожидалась полная сумма %d", got, o.AmountKopecks())
	}
}

func TestSetDemoPayment_ЗаполняетПолосуДемо(t *testing.T) {
	o := makeDemoOrder(t)

	if !o.HasDemoPayment() {
		t.Fatal("HasDemoPayment = false после SetDemoPayment")
	}
	if got := o.DemoInvoiceID(); got != demoInvoiceNo {
		t.Errorf("DemoInvoiceID = %d, ожидалось %d", got, demoInvoiceNo)
	}
	if got := o.DemoAmountKopecks(); got != demoPrice {
		t.Errorf("DemoAmountKopecks = %d, ожидалось %d", got, demoPrice)
	}
	if got := o.DemoPaymentStatus(); got != PaymentStatusPending {
		t.Errorf("DemoPaymentStatus = %q, ожидался pending", got)
	}
}

// Нулевой InvId или неположительная цена — «демо-платежа нет»: заказ должен
// остаться оплачиваемым одним платежом, а не получить битый счёт.
func TestSetDemoPayment_ИгнорируетНекорректныеЗначения(t *testing.T) {
	cases := []struct {
		name      string
		invoiceID int64
		amount    int64
	}{
		{"нулевой InvId", 0, demoPrice},
		{"нулевая цена", demoInvoiceNo, 0},
		{"отрицательная цена", demoInvoiceNo, -100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := makeOrder(t)
			o.SetDemoPayment(tc.invoiceID, tc.amount)
			if o.HasDemoPayment() {
				t.Error("счёт на демо не должен был выставиться")
			}
		})
	}
}

func TestMarkDemoPaid_ПереводитВPaidИНеТрогаетОсновнуюПолосу(t *testing.T) {
	o := makeDemoOrder(t)

	if err := o.MarkDemoPaid(); err != nil {
		t.Fatalf("MarkDemoPaid: %v", err)
	}
	if !o.DemoPaid() {
		t.Error("DemoPaid = false после MarkDemoPaid")
	}
	// Оплата демо не оплачивает заказ: песня ждёт доплаты.
	if got := o.PaymentStatus(); got != PaymentStatusPending {
		t.Errorf("PaymentStatus = %q, ожидался pending — 50 ₽ не оплачивают песню", got)
	}
	if o.PaidAt() != nil {
		t.Error("paid_at не должен проставляться оплатой демо")
	}
}

func TestMarkDemoPaid_ПовторныйВызовДаётErrOrderAlreadyPaid(t *testing.T) {
	o := makeDemoOrder(t)
	if err := o.MarkDemoPaid(); err != nil {
		t.Fatalf("первый MarkDemoPaid: %v", err)
	}

	// Вызывающий слой трактует эту ошибку как идемпотентный успех вебхука.
	if err := o.MarkDemoPaid(); !errors.Is(err, ErrOrderAlreadyPaid) {
		t.Fatalf("ожидалась ErrOrderAlreadyPaid, получено: %v", err)
	}
}

func TestRemainingKopecks_ЗачитываетДемоТолькоПослеОплаты(t *testing.T) {
	o := makeDemoOrder(t)

	// До оплаты демо зачитывать нечего — счёт на песню идёт на полную сумму.
	if got := o.RemainingKopecks(); got != orderPrice {
		t.Errorf("до оплаты демо RemainingKopecks = %d, ожидалось %d", got, orderPrice)
	}

	if err := o.MarkDemoPaid(); err != nil {
		t.Fatalf("MarkDemoPaid: %v", err)
	}
	if got := o.RemainingKopecks(); got != remainingSum {
		t.Errorf("после оплаты демо RemainingKopecks = %d, ожидалось %d", got, remainingSum)
	}
	// Полная цена не меняется: промокоды и возвраты считаются по ней.
	if got := o.AmountKopecks(); got != orderPrice {
		t.Errorf("AmountKopecks = %d, ожидалось %d", got, orderPrice)
	}
}

// Промокод может опустить цену заказа ниже уже уплаченного за демо — остаток
// не должен уходить в минус и превращаться в отрицательную сумму счёта.
func TestRemainingKopecks_НеУходитВМинус(t *testing.T) {
	o, err := NewOrder(4242, "test@example.com", "", "Бриф", "", "", CurrentConsentDocVersion, orderPrice)
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	o.SetDemoPayment(demoInvoiceNo, demoPrice)
	if err := o.MarkDemoPaid(); err != nil {
		t.Fatalf("MarkDemoPaid: %v", err)
	}

	// Скидка почти на всю сумму: цена заказа становится меньше цены демо.
	o.ApplyPromo(uuid.New(), orderPrice-1000)

	if got := o.RemainingKopecks(); got != 0 {
		t.Errorf("RemainingKopecks = %d, ожидался 0 — отрицательный остаток недопустим", got)
	}
}

// Заказы, созданные до миграции 0025, приходят из БД с пустым статусом оплаты
// демо — он должен читаться как «не оплачено», а не как пустая строка.
func TestRestoreOrder_НормализуетПустойСтатусОплатыДемо(t *testing.T) {
	o := RestoreOrder(OrderSnapshot{
		ID:                uuid.New(),
		InvoiceID:         4242,
		CustomerEmail:     "legacy@example.com",
		AmountKopecks:     orderPrice,
		PaymentStatus:     PaymentStatusPending,
		GenerationStatus:  GenerationStatusNew,
		DemoPaymentStatus: "", // колонки не было
	})

	if got := o.DemoPaymentStatus(); got != PaymentStatusPending {
		t.Errorf("DemoPaymentStatus = %q, ожидался pending", got)
	}
	if o.HasDemoPayment() {
		t.Error("легаси-заказ без demo_invoice_id не должен считаться имеющим счёт на демо")
	}
	if got := o.RemainingKopecks(); got != orderPrice {
		t.Errorf("RemainingKopecks = %d, легаси-заказ платится целиком (%d)", got, orderPrice)
	}
}

// Snapshot должен переносить обе платёжные полосы: иначе репозиторий сохранит
// заказ без счёта на демо и клиент потеряет оплаченный шаг.
func TestSnapshot_СохраняетПолосуДемо(t *testing.T) {
	o := makeDemoOrder(t)
	if err := o.MarkDemoPaid(); err != nil {
		t.Fatalf("MarkDemoPaid: %v", err)
	}

	snap := o.Snapshot()
	if snap.DemoInvoiceID != demoInvoiceNo {
		t.Errorf("snapshot.DemoInvoiceID = %d, ожидалось %d", snap.DemoInvoiceID, demoInvoiceNo)
	}
	if snap.DemoAmountKopecks != demoPrice {
		t.Errorf("snapshot.DemoAmountKopecks = %d, ожидалось %d", snap.DemoAmountKopecks, demoPrice)
	}
	if snap.DemoPaymentStatus != PaymentStatusPaid {
		t.Errorf("snapshot.DemoPaymentStatus = %q, ожидался paid", snap.DemoPaymentStatus)
	}

	restored := RestoreOrder(snap)
	if !restored.DemoPaid() || restored.RemainingKopecks() != remainingSum {
		t.Errorf("после round-trip: DemoPaid=%v, Remaining=%d; ожидалось true / %d",
			restored.DemoPaid(), restored.RemainingKopecks(), remainingSum)
	}
}

// --- Демо-полоса генерации (StartDemo → CompleteDemo / FailDemo / ClearDemo) ---

func TestStartDemo_ЗахватываетАккаунтИЗапрещаетПовтор(t *testing.T) {
	o := makeDemoOrder(t)
	accID := uuid.New()

	if err := o.StartDemo(accID); err != nil {
		t.Fatalf("StartDemo: %v", err)
	}
	if got := o.DemoStatus(); got != DemoStatusProcessing {
		t.Errorf("DemoStatus = %q, ожидался processing", got)
	}
	if o.DemoAccountID() == nil || *o.DemoAccountID() != accID {
		t.Error("DemoAccountID не зафиксирован — recovery не освободит слот при краше")
	}

	// Повторный старт уже идущего демо запрещён: иначе двойной расход кредитов.
	if err := o.StartDemo(uuid.New()); err == nil {
		t.Error("повторный StartDemo для processing должен возвращать ошибку")
	}
}

func TestCompleteDemo_ФиксируетКлипыИОсвобождаетАккаунт(t *testing.T) {
	o := makeDemoOrder(t)
	if err := o.StartDemo(uuid.New()); err != nil {
		t.Fatalf("StartDemo: %v", err)
	}

	clips := []Track{{Index: 1, AudioURL: "https://cdn/1.mp3"}, {Index: 2, AudioURL: "https://cdn/2.mp3"}}
	if err := o.CompleteDemo("https://cdn/demo.mp3", clips); err != nil {
		t.Fatalf("CompleteDemo: %v", err)
	}

	if got := o.DemoStatus(); got != DemoStatusReady {
		t.Errorf("DemoStatus = %q, ожидался ready", got)
	}
	if got := o.DemoURL(); got != "https://cdn/demo.mp3" {
		t.Errorf("DemoURL = %q", got)
	}
	if len(o.DemoClips()) != 2 {
		t.Errorf("DemoClips = %d, ожидалось 2 — они переиспользуются как версии 1–2", len(o.DemoClips()))
	}
	if o.DemoAccountID() != nil {
		t.Error("DemoAccountID должен обнуляться: слот аккаунта освобождён")
	}

	// Повторный StartDemo для готового демо запрещён.
	if err := o.StartDemo(uuid.New()); err == nil {
		t.Error("StartDemo для ready-демо должен возвращать ошибку")
	}
}

func TestCompleteDemo_ПустойURLОтклоняется(t *testing.T) {
	o := makeDemoOrder(t)
	if err := o.CompleteDemo("", nil); err == nil {
		t.Error("CompleteDemo с пустым URL должен возвращать ошибку")
	}
}

func TestFailDemo_ОсвобождаетАккаунтИРазрешаетПовтор(t *testing.T) {
	o := makeDemoOrder(t)
	if err := o.StartDemo(uuid.New()); err != nil {
		t.Fatalf("StartDemo: %v", err)
	}

	o.FailDemo()
	if got := o.DemoStatus(); got != DemoStatusFailed {
		t.Errorf("DemoStatus = %q, ожидался failed", got)
	}
	if o.DemoAccountID() != nil {
		t.Error("DemoAccountID должен обнуляться после провала")
	}
	// Из failed повтор разрешён — клиент уже заплатил за демо.
	if err := o.StartDemo(uuid.New()); err != nil {
		t.Errorf("повторный запуск после failed должен быть разрешён: %v", err)
	}
}

func TestMarkDemoLimited_НеПерезаписываетИдущееИлиГотовоеДемо(t *testing.T) {
	t.Run("из none переводит в limited", func(t *testing.T) {
		o := makeDemoOrder(t)
		o.MarkDemoLimited()
		if got := o.DemoStatus(); got != DemoStatusLimited {
			t.Errorf("DemoStatus = %q, ожидался limited", got)
		}
	})

	t.Run("processing не трогает", func(t *testing.T) {
		o := makeDemoOrder(t)
		if err := o.StartDemo(uuid.New()); err != nil {
			t.Fatalf("StartDemo: %v", err)
		}
		o.MarkDemoLimited()
		if got := o.DemoStatus(); got != DemoStatusProcessing {
			t.Errorf("DemoStatus = %q, идущее демо не должно сбрасываться в limited", got)
		}
	})

	t.Run("ready не трогает", func(t *testing.T) {
		o := makeDemoOrder(t)
		if err := o.StartDemo(uuid.New()); err != nil {
			t.Fatalf("StartDemo: %v", err)
		}
		if err := o.CompleteDemo("https://cdn/demo.mp3", nil); err != nil {
			t.Fatalf("CompleteDemo: %v", err)
		}
		o.MarkDemoLimited()
		if got := o.DemoStatus(); got != DemoStatusReady {
			t.Errorf("DemoStatus = %q, готовое демо не должно сбрасываться в limited", got)
		}
	})
}

func TestClearDemo_СбрасываетСсылкиНоНеОплату(t *testing.T) {
	o := makeDemoOrder(t)
	if err := o.StartDemo(uuid.New()); err != nil {
		t.Fatalf("StartDemo: %v", err)
	}
	if err := o.CompleteDemo("https://cdn/demo.mp3", []Track{{Index: 1}}); err != nil {
		t.Fatalf("CompleteDemo: %v", err)
	}
	if err := o.MarkDemoPaid(); err != nil {
		t.Fatalf("MarkDemoPaid: %v", err)
	}

	o.ClearDemo()

	if got := o.DemoStatus(); got != DemoStatusNone {
		t.Errorf("DemoStatus = %q, ожидался none", got)
	}
	if o.DemoURL() != "" || o.DemoClips() != nil || o.DemoAccountID() != nil {
		t.Error("ClearDemo должен обнулить url, клипы и аккаунт")
	}
	// Деньги клиента остаются зачтёнными: чистка хранилища не отменяет платёж.
	if !o.DemoPaid() {
		t.Error("ClearDemo не должен сбрасывать статус оплаты демо")
	}
	if got := o.RemainingKopecks(); got != remainingSum {
		t.Errorf("RemainingKopecks = %d, ожидалось %d — зачёт должен сохраниться", got, remainingSum)
	}
}
