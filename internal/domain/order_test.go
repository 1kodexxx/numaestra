package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// --- NewOrder ---

func TestNewOrder_Valid(t *testing.T) {
	o, err := NewOrder(1, "user@example.com", "+79991234567", "Р’РµСЃС‘Р»Р°СЏ РїРµСЃРЅСЏ", "", "", 150000)
	if err != nil {
		t.Fatalf("РѕР¶РёРґР°Р»Рё СѓСЃРїРµС…, РїРѕР»СѓС‡РёР»Рё РѕС€РёР±РєСѓ: %v", err)
	}
	if o.ID() == (uuid.UUID{}) {
		t.Error("ID РЅРµ РґРѕР»Р¶РµРЅ Р±С‹С‚СЊ РЅСѓР»РµРІС‹Рј")
	}
	if o.PaymentStatus() != PaymentStatusPending {
		t.Errorf("РѕР¶РёРґР°Р»Рё СЃС‚Р°С‚СѓСЃ РѕРїР»Р°С‚С‹ %q, РїРѕР»СѓС‡РёР»Рё %q", PaymentStatusPending, o.PaymentStatus())
	}
	if o.GenerationStatus() != GenerationStatusNew {
		t.Errorf("РѕР¶РёРґР°Р»Рё СЃС‚Р°С‚СѓСЃ РіРµРЅРµСЂР°С†РёРё %q, РїРѕР»СѓС‡РёР»Рё %q", GenerationStatusNew, o.GenerationStatus())
	}
}

func TestNewOrder_GeneratesUniqueAccessTokens(t *testing.T) {
	o1, _ := NewOrder(1, "a@example.com", "", "Р‘СЂРёС„ 1", "", "", 100)
	o2, _ := NewOrder(2, "b@example.com", "", "Р‘СЂРёС„ 2", "", "", 100)

	if o1.AccessToken() == "" {
		t.Error("С‚РѕРєРµРЅ РЅРµ РґРѕР»Р¶РµРЅ Р±С‹С‚СЊ РїСѓСЃС‚С‹Рј")
	}
	if o1.AccessToken() == o2.AccessToken() {
		t.Error("С‚РѕРєРµРЅС‹ РґРІСѓС… СЂР°Р·РЅС‹С… Р·Р°РєР°Р·РѕРІ РЅРµ РґРѕР»Р¶РЅС‹ СЃРѕРІРїР°РґР°С‚СЊ")
	}
	if len(o1.AccessToken()) != 64 {
		t.Errorf("РѕР¶РёРґР°Р»Рё РґР»РёРЅСѓ С‚РѕРєРµРЅР° 64 СЃРёРјРІРѕР»Р°, РїРѕР»СѓС‡РёР»Рё %d", len(o1.AccessToken()))
	}
}

func TestNewOrder_RequiresContact(t *testing.T) {
	_, err := NewOrder(1, "", "", "Р‘СЂРёС„", "", "", 100)
	if err == nil {
		t.Error("РѕР¶РёРґР°Р»Рё РѕС€РёР±РєСѓ РїСЂРё РѕС‚СЃСѓС‚СЃС‚РІРёРё email Рё С‚РµР»РµС„РѕРЅР°")
	}
}

func TestNewOrder_RequiresBrief(t *testing.T) {
	_, err := NewOrder(1, "user@example.com", "", "", "", "", 100)
	if err == nil {
		t.Error("РѕР¶РёРґР°Р»Рё РѕС€РёР±РєСѓ РїСЂРё РїСѓСЃС‚РѕРј Р±СЂРёС„Рµ")
	}
}

func TestNewOrder_RejectsTooLongBrief(t *testing.T) {
	longBrief := strings.Repeat("a", MaxBriefLength+1)
	_, err := NewOrder(1, "user@example.com", "", longBrief, "", "", 100)
	if err == nil {
		t.Fatal("РѕР¶РёРґР°Р»Рё РѕС€РёР±РєСѓ РїСЂРё СЃР»РёС€РєРѕРј РґР»РёРЅРЅРѕРј Р±СЂРёС„Рµ")
	}
	if err != ErrBriefTooLong {
		t.Errorf("РѕР¶РёРґР°Р»Рё ErrBriefTooLong, РїРѕР»СѓС‡РёР»Рё %v", err)
	}

	// Р‘СЂРёС„ СЂРѕРІРЅРѕ РЅР° РіСЂР°РЅРёС†Рµ РґР»РёРЅС‹ РґРѕР»Р¶РµРЅ РїСЂРёРЅРёРјР°С‚СЊСЃСЏ.
	maxBrief := strings.Repeat("a", MaxBriefLength)
	if _, err := NewOrder(1, "user@example.com", "", maxBrief, "", "", 100); err != nil {
		t.Errorf("Р±СЂРёС„ РґР»РёРЅРѕР№ СЂРѕРІРЅРѕ MaxBriefLength РґРѕР»Р¶РµРЅ РїСЂРёРЅРёРјР°С‚СЊСЃСЏ, РїРѕР»СѓС‡РёР»Рё %v", err)
	}
}

func TestNewOrder_RequiresPositiveAmount(t *testing.T) {
	_, err := NewOrder(1, "user@example.com", "", "Р‘СЂРёС„", "", "", 0)
	if err == nil {
		t.Error("РѕР¶РёРґР°Р»Рё РѕС€РёР±РєСѓ РїСЂРё РЅСѓР»РµРІРѕР№ СЃСѓРјРјРµ")
	}
	_, err = NewOrder(1, "user@example.com", "", "Р‘СЂРёС„", "", "", -100)
	if err == nil {
		t.Error("РѕР¶РёРґР°Р»Рё РѕС€РёР±РєСѓ РїСЂРё РѕС‚СЂРёС†Р°С‚РµР»СЊРЅРѕР№ СЃСѓРјРјРµ")
	}
}

func TestNewOrder_OnlyPhone(t *testing.T) {
	_, err := NewOrder(1, "", "+79991234567", "Р‘СЂРёС„", "", "", 100)
	if err != nil {
		t.Errorf("С‚РµР»РµС„РѕРЅ Р±РµР· email РґРѕР»Р¶РµРЅ Р±С‹С‚СЊ РґРѕРїСѓСЃС‚РёРј: %v", err)
	}
}

// --- РЎС‚РµР№С‚-РјР°С€РёРЅР° РѕРїР»Р°С‚С‹ ---

func TestOrder_MarkPaid(t *testing.T) {
	o := newTestOrder(t)

	if err := o.MarkPaid(); err != nil {
		t.Fatalf("MarkPaid СѓРїР°Р»: %v", err)
	}
	if o.PaymentStatus() != PaymentStatusPaid {
		t.Errorf("РѕР¶РёРґР°Р»Рё %q, РїРѕР»СѓС‡РёР»Рё %q", PaymentStatusPaid, o.PaymentStatus())
	}
}

func TestOrder_MarkPaid_Idempotency(t *testing.T) {
	o := newTestOrder(t)
	_ = o.MarkPaid()

	err := o.MarkPaid()
	if err != ErrOrderAlreadyPaid {
		t.Errorf("РїРѕРІС‚РѕСЂРЅС‹Р№ MarkPaid РґРѕР»Р¶РµРЅ РІРѕР·РІСЂР°С‰Р°С‚СЊ ErrOrderAlreadyPaid, РїРѕР»СѓС‡РёР»Рё: %v", err)
	}
}

func TestOrder_MarkPaymentFailed(t *testing.T) {
	o := newTestOrder(t)
	if err := o.MarkPaymentFailed(); err != nil {
		t.Fatalf("MarkPaymentFailed СѓРїР°Р»: %v", err)
	}
	if o.PaymentStatus() != PaymentStatusFailed {
		t.Errorf("РѕР¶РёРґР°Р»Рё %q, РїРѕР»СѓС‡РёР»Рё %q", PaymentStatusFailed, o.PaymentStatus())
	}
}

// --- РЎС‚РµР№С‚-РјР°С€РёРЅР° РіРµРЅРµСЂР°С†РёРё ---

func TestOrder_GenerationLifecycle_HappyPath(t *testing.T) {
	o := newPaidOrder(t)
	accountID := uuid.New()

	// Pending в†’ Queued
	if err := o.Enqueue(); err != nil {
		t.Fatalf("Enqueue СѓРїР°Р»: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusQueued {
		t.Errorf("РѕР¶РёРґР°Р»Рё Queued, РїРѕР»СѓС‡РёР»Рё %q", o.GenerationStatus())
	}

	// Queued в†’ Processing
	if err := o.StartProcessing(accountID); err != nil {
		t.Fatalf("StartProcessing СѓРїР°Р»: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusProcessing {
		t.Errorf("РѕР¶РёРґР°Р»Рё Processing, РїРѕР»СѓС‡РёР»Рё %q", o.GenerationStatus())
	}

	// Processing в†’ Completed
	tracks := []Track{{ID: uuid.New(), Index: 1, AudioURL: "https://s3/track1.mp3"}}
	if err := o.Complete(tracks); err != nil {
		t.Fatalf("Complete СѓРїР°Р»: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusCompleted {
		t.Errorf("РѕР¶РёРґР°Р»Рё Completed, РїРѕР»СѓС‡РёР»Рё %q", o.GenerationStatus())
	}
	if len(o.Tracks()) != 1 {
		t.Errorf("РѕР¶РёРґР°Р»Рё 1 С‚СЂРµРє, РїРѕР»СѓС‡РёР»Рё %d", len(o.Tracks()))
	}
}

func TestOrder_Enqueue_RequiresPaid(t *testing.T) {
	o := newTestOrder(t) // СЃС‚Р°С‚СѓСЃ Pending
	err := o.Enqueue()
	if err != ErrOrderNotPaid {
		t.Errorf("Enqueue Р±РµР· РѕРїР»Р°С‚С‹ РґРѕР»Р¶РµРЅ РІРµСЂРЅСѓС‚СЊ ErrOrderNotPaid, РїРѕР»СѓС‡РёР»Рё: %v", err)
	}
}

func TestOrder_Complete_RequiresTracks(t *testing.T) {
	o := newProcessingOrder(t)
	err := o.Complete(nil)
	if err == nil {
		t.Error("Complete СЃ РїСѓСЃС‚С‹Рј СЃРїРёСЃРєРѕРј С‚СЂРµРєРѕРІ РґРѕР»Р¶РµРЅ РІРµСЂРЅСѓС‚СЊ РѕС€РёР±РєСѓ")
	}
}

func TestOrder_RequeueForRetry(t *testing.T) {
	o := newProcessingOrder(t)
	if err := o.RequeueForRetry(); err != nil {
		t.Fatalf("RequeueForRetry СѓРїР°Р»: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusQueued {
		t.Errorf("РѕР¶РёРґР°Р»Рё Queued РїРѕСЃР»Рµ retry, РїРѕР»СѓС‡РёР»Рё %q", o.GenerationStatus())
	}
	if o.AssignedAccountID() != nil {
		t.Error("AssignedAccountID РґРѕР»Р¶РµРЅ Р±С‹С‚СЊ nil РїРѕСЃР»Рµ retry")
	}
}

func TestOrder_Fail(t *testing.T) {
	o := newProcessingOrder(t)
	if err := o.Fail("РїСЂРµРІС‹С€РµРЅ Р»РёРјРёС‚ СЂРµС‚СЂР°РµРІ"); err != nil {
		t.Fatalf("Fail СѓРїР°Р»: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusFailed {
		t.Errorf("РѕР¶РёРґР°Р»Рё Failed, РїРѕР»СѓС‡РёР»Рё %q", o.GenerationStatus())
	}
	if !strings.Contains(o.FailureReason(), "Р»РёРјРёС‚") {
		t.Errorf("FailureReason РґРѕР»Р¶РµРЅ СЃРѕРґРµСЂР¶Р°С‚СЊ РїСЂРёС‡РёРЅСѓ, РїРѕР»СѓС‡РёР»Рё %q", o.FailureReason())
	}
}

// --- Snapshot / Restore ---

func TestOrder_SnapshotRestore_PreservesToken(t *testing.T) {
	original, _ := NewOrder(42, "user@example.com", "", "Р‘СЂРёС„", "", "", 100000)
	snap := original.Snapshot()

	if snap.AccessToken == "" {
		t.Error("Snapshot РґРѕР»Р¶РµРЅ СЃРѕРґРµСЂР¶Р°С‚СЊ AccessToken")
	}

	restored := RestoreOrder(snap)
	if restored.AccessToken() != original.AccessToken() {
		t.Errorf("С‚РѕРєРµРЅ РїРѕС‚РµСЂСЏР»СЃСЏ РїСЂРё Snapshot/Restore: Р±С‹Р»Рѕ %q, СЃС‚Р°Р»Рѕ %q",
			original.AccessToken(), restored.AccessToken())
	}
	if restored.ID() != original.ID() {
		t.Error("ID РЅРµ СЃРѕРІРїР°РґР°РµС‚ РїРѕСЃР»Рµ Restore")
	}
}

// --- MarkRefunded ---

func TestOrder_MarkRefunded_Success(t *testing.T) {
	o := newPaidOrder(t)
	if err := o.MarkRefunded(); err != nil {
		t.Fatalf("MarkRefunded СѓРїР°Р»: %v", err)
	}
	if o.PaymentStatus() != PaymentStatusRefunded {
		t.Errorf("РѕР¶РёРґР°Р»Рё Refunded, РїРѕР»СѓС‡РёР»Рё %q", o.PaymentStatus())
	}
}

func TestOrder_MarkRefunded_WhenNotPaid_ReturnsError(t *testing.T) {
	o := newTestOrder(t) // status = Pending
	err := o.MarkRefunded()
	if err != ErrInvalidPaymentTransition {
		t.Errorf("РѕР¶РёРґР°Р»Рё ErrInvalidPaymentTransition, РїРѕР»СѓС‡РёР»Рё %v", err)
	}
}

// --- Р“РµС‚С‚РµСЂС‹ ---

func TestOrder_Getters(t *testing.T) {
	o, err := NewOrder(42, "user@example.com", "+79991234567", "РўРµСЃС‚РѕРІС‹Р№ Р±СЂРёС„", "wedding", "Create a song", 150000)
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}

	if o.InvoiceID() != 42 {
		t.Errorf("InvoiceID: got %d, want 42", o.InvoiceID())
	}
	if o.CustomerEmail() != "user@example.com" {
		t.Errorf("CustomerEmail: got %q", o.CustomerEmail())
	}
	if o.CustomerPhone() != "+79991234567" {
		t.Errorf("CustomerPhone: got %q", o.CustomerPhone())
	}
	if o.Brief() != "РўРµСЃС‚РѕРІС‹Р№ Р±СЂРёС„" {
		t.Errorf("Brief: got %q", o.Brief())
	}
	if o.CategoryID() != "wedding" {
		t.Errorf("CategoryID: got %q", o.CategoryID())
	}
	if o.SunoPrompt() != "Create a song" {
		t.Errorf("SunoPrompt: got %q", o.SunoPrompt())
	}
	if o.AmountKopecks() != 150000 {
		t.Errorf("AmountKopecks: got %d", o.AmountKopecks())
	}
	if o.Currency() != "RUB" {
		t.Errorf("Currency: got %q", o.Currency())
	}
	if o.UpdatedAt().IsZero() {
		t.Error("UpdatedAt РЅРµ РґРѕР»Р¶РµРЅ Р±С‹С‚СЊ РЅСѓР»РµРІС‹Рј")
	}
}

func TestOrder_RestoreOrder_PreservesAllFields(t *testing.T) {
	o, _ := NewOrder(7, "a@example.com", "+70000000001", "Р‘СЂРёС„", "boss", "Prompt", 290000)
	_ = o.MarkPaid()
	_ = o.Enqueue()

	snap := o.Snapshot()
	r := RestoreOrder(snap)

	if r.InvoiceID() != 7 {
		t.Errorf("InvoiceID: got %d", r.InvoiceID())
	}
	if r.CustomerPhone() != "+70000000001" {
		t.Errorf("CustomerPhone: got %q", r.CustomerPhone())
	}
	if r.CategoryID() != "boss" {
		t.Errorf("CategoryID: got %q", r.CategoryID())
	}
	if r.SunoPrompt() != "Prompt" {
		t.Errorf("SunoPrompt: got %q", r.SunoPrompt())
	}
	if r.AmountKopecks() != 290000 {
		t.Errorf("AmountKopecks: got %d", r.AmountKopecks())
	}
	if r.GenerationStatus() != GenerationStatusQueued {
		t.Errorf("GenerationStatus: got %q", r.GenerationStatus())
	}
}

// --- helpers ---

func newTestOrder(t *testing.T) *Order {
	t.Helper()
	o, err := NewOrder(1, "user@example.com", "", "РўРµСЃС‚РѕРІС‹Р№ Р±СЂРёС„", "", "", 150000)
	if err != nil {
		t.Fatalf("РЅРµ СѓРґР°Р»РѕСЃСЊ СЃРѕР·РґР°С‚СЊ С‚РµСЃС‚РѕРІС‹Р№ Р·Р°РєР°Р·: %v", err)
	}
	return o
}

func newPaidOrder(t *testing.T) *Order {
	t.Helper()
	o := newTestOrder(t)
	if err := o.MarkPaid(); err != nil {
		t.Fatalf("MarkPaid СѓРїР°Р»: %v", err)
	}
	return o
}

func newProcessingOrder(t *testing.T) *Order {
	t.Helper()
	o := newPaidOrder(t)
	_ = o.Enqueue()
	_ = o.StartProcessing(uuid.New())
	return o
}

func TestOrder_Regenerate(t *testing.T) {
	o, _ := NewOrder(1, "u@e.c", "", "Бриф", "", "", 100)

	// Неоплаченный — нельзя.
	if err := o.Regenerate(); err == nil {
		t.Error("неоплаченный заказ нельзя перегенерировать")
	}

	_ = o.MarkPaid()
	_ = o.Enqueue()
	acc := uuid.New()
	_ = o.StartProcessing(acc)

	// processing (не failed) — нельзя.
	if err := o.Regenerate(); err == nil {
		t.Error("заказ не в статусе failed нельзя перегенерировать")
	}

	_ = o.Fail("в пуле нет аккаунтов")

	// paid + failed — можно.
	if err := o.Regenerate(); err != nil {
		t.Fatalf("paid+failed должен перегенерироваться: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusQueued {
		t.Errorf("ожидали queued, получили %q", o.GenerationStatus())
	}
	if o.FailureReason() != "" {
		t.Errorf("причина ошибки должна очиститься, получили %q", o.FailureReason())
	}
	if o.AssignedAccountID() != nil {
		t.Error("привязка аккаунта должна сброситься")
	}
}
