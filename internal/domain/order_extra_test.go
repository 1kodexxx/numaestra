package domain

import (
	"testing"

	"github.com/google/uuid"
)

func makeOrder(t *testing.T) *Order {
	t.Helper()
	o, err := NewOrder(42, "test@example.com", "+79991234567", "Бриф", "", "", CurrentConsentDocVersion, 150000)
	if err != nil {
		t.Fatalf("makeOrder: %v", err)
	}
	return o
}

func makePaidOrder(t *testing.T) *Order {
	t.Helper()
	o := makeOrder(t)
	if err := o.MarkPaid(); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	return o
}

func makeCompletedOrder(t *testing.T) *Order {
	t.Helper()
	o := makePaidOrder(t)
	accID := uuid.New()
	_ = o.Enqueue()
	_ = o.StartProcessing(accID)
	_ = o.Complete([]Track{{SunoTrackID: "s1", AudioURL: "http://url1"}})
	return o
}

// --- accessors ---

func TestOrder_PromoAccessors(t *testing.T) {
	o := makeOrder(t)
	if o.PromoCodeID() != nil {
		t.Error("PromoCodeID должен быть nil")
	}
	if o.OriginalAmountKopecks() != 0 {
		t.Error("OriginalAmountKopecks должен быть 0")
	}
	if o.DiscountKopecks() != 0 {
		t.Error("DiscountKopecks должен быть 0")
	}
	if o.ReferralCode() != "" {
		t.Error("ReferralCode должен быть пустым")
	}
}

func TestOrder_TimeAccessors(t *testing.T) {
	o := makeOrder(t)
	if o.CreatedAt().IsZero() {
		t.Error("CreatedAt не должен быть нулевым")
	}
	if o.PaidAt() != nil {
		t.Error("PaidAt должен быть nil до оплаты")
	}

	_ = o.MarkPaid()
	if o.PaidAt() == nil {
		t.Error("PaidAt должен быть установлен после MarkPaid")
	}
}

func TestOrder_ConsentAccessors(t *testing.T) {
	o := makeOrder(t)
	if o.ConsentDocVersion() != CurrentConsentDocVersion {
		t.Errorf("ConsentDocVersion: %q", o.ConsentDocVersion())
	}
	if o.ConsentGivenAt() == nil {
		t.Error("ConsentGivenAt: должен быть установлен при создании заказа")
	}
}

func TestOrder_ShareRevokedAccessor(t *testing.T) {
	o := makeCompletedOrder(t)
	if o.ShareRevoked() {
		t.Error("ShareRevoked должен быть false у нового завершённого заказа")
	}
	if o.ShareRevokedAt() != nil {
		t.Error("ShareRevokedAt должен быть nil")
	}
}

func TestOrder_AdminFeedbackAccessors(t *testing.T) {
	o := makeOrder(t)
	if o.AdminFeedback() != "" {
		t.Error("AdminFeedback должен быть пустым")
	}
	if o.AdminFeedbackAt() != nil {
		t.Error("AdminFeedbackAt должен быть nil")
	}
}

// --- NormalizeCustomerEmail / CustomerEmailMatches ---

func TestNormalizeCustomerEmail(t *testing.T) {
	cases := []struct{ in, out string }{
		{"User@Example.COM", "user@example.com"},
		{"  spaced@test.ru  ", "spaced@test.ru"},
		{"", ""},
	}
	for _, c := range cases {
		got := NormalizeCustomerEmail(c.in)
		if got != c.out {
			t.Errorf("NormalizeCustomerEmail(%q) = %q, ожидали %q", c.in, got, c.out)
		}
	}
}

func TestCustomerEmailMatches(t *testing.T) {
	o := makeOrder(t) // email = test@example.com
	if !CustomerEmailMatches(o, "TEST@EXAMPLE.COM") {
		t.Error("должны совпадать (case-insensitive)")
	}
	if CustomerEmailMatches(o, "other@example.com") {
		t.Error("разные email не должны совпадать")
	}
	if CustomerEmailMatches(nil, "test@example.com") {
		t.Error("nil order не должен совпадать")
	}
}

// --- SetAdminFeedback ---

func TestOrder_SetAdminFeedback(t *testing.T) {
	o := makeOrder(t)
	if err := o.SetAdminFeedback("Ваш заказ готов!"); err != nil {
		t.Fatalf("SetAdminFeedback: %v", err)
	}
	if o.AdminFeedback() != "Ваш заказ готов!" {
		t.Errorf("AdminFeedback: %q", o.AdminFeedback())
	}
	if o.AdminFeedbackAt() == nil {
		t.Error("AdminFeedbackAt должен быть установлен")
	}
}

func TestOrder_SetAdminFeedback_Empty(t *testing.T) {
	o := makeOrder(t)
	if err := o.SetAdminFeedback(""); err == nil {
		t.Error("ожидали ошибку для пустого сообщения")
	}
}

// --- CancelGenerationForRefund ---

func TestOrder_CancelGenerationForRefund_Queued(t *testing.T) {
	o := makePaidOrder(t)
	_ = o.Enqueue()

	accID := o.CancelGenerationForRefund()
	if accID != nil {
		t.Error("ожидали nil accountID для queued без назначенного аккаунта")
	}
	if o.GenerationStatus() != GenerationStatusFailed {
		t.Errorf("ожидали failed, получили %q", o.GenerationStatus())
	}
}

func TestOrder_CancelGenerationForRefund_Processing(t *testing.T) {
	o := makePaidOrder(t)
	_ = o.Enqueue()
	acc := uuid.New()
	_ = o.StartProcessing(acc)

	returnedID := o.CancelGenerationForRefund()
	if returnedID == nil || *returnedID != acc {
		t.Error("должны вернуть accountID из processing")
	}
	if o.GenerationStatus() != GenerationStatusFailed {
		t.Errorf("ожидали failed, получили %q", o.GenerationStatus())
	}
}

func TestOrder_CancelGenerationForRefund_Completed_NoOp(t *testing.T) {
	o := makeCompletedOrder(t)
	id := o.CancelGenerationForRefund()
	if id != nil {
		t.Error("completed заказ не должен отменяться")
	}
	if o.GenerationStatus() != GenerationStatusCompleted {
		t.Error("статус не должен измениться")
	}
}

// --- RevokeShare / RestoreShare ---

func TestOrder_RevokeRestoreShare(t *testing.T) {
	o := makeCompletedOrder(t)

	if err := o.RevokeShare(); err != nil {
		t.Fatalf("RevokeShare: %v", err)
	}
	if !o.ShareRevoked() {
		t.Error("ShareRevoked должен быть true после RevokeShare")
	}
	if o.ShareRevokedAt() == nil {
		t.Error("ShareRevokedAt должен быть установлен")
	}

	if err := o.RestoreShare(); err != nil {
		t.Fatalf("RestoreShare: %v", err)
	}
	if o.ShareRevoked() {
		t.Error("ShareRevoked должен быть false после RestoreShare")
	}
	if o.ShareRevokedAt() != nil {
		t.Error("ShareRevokedAt должен быть nil после RestoreShare")
	}
}

func TestOrder_RevokeShare_NotCompleted(t *testing.T) {
	o := makeOrder(t)
	if err := o.RevokeShare(); err == nil {
		t.Error("RevokeShare на незавершённом заказе должна вернуть ошибку")
	}
}

func TestOrder_RestoreShare_NotCompleted(t *testing.T) {
	o := makeOrder(t)
	if err := o.RestoreShare(); err == nil {
		t.Error("RestoreShare на незавершённом заказе должна вернуть ошибку")
	}
}

// --- ApplyPromo / SetReferralCode ---

func TestOrder_ApplyPromo(t *testing.T) {
	o := makeOrder(t) // amount = 150000
	promoID := uuid.New()
	o.ApplyPromo(promoID, 15000)

	if o.OriginalAmountKopecks() != 150000 {
		t.Errorf("OriginalAmountKopecks: %d", o.OriginalAmountKopecks())
	}
	if o.DiscountKopecks() != 15000 {
		t.Errorf("DiscountKopecks: %d", o.DiscountKopecks())
	}
	if o.AmountKopecks() != 135000 {
		t.Errorf("AmountKopecks после скидки: %d", o.AmountKopecks())
	}
	if o.PromoCodeID() == nil || *o.PromoCodeID() != promoID {
		t.Error("PromoCodeID не установлен")
	}
}

func TestOrder_ApplyPromo_CapsAtZero(t *testing.T) {
	o := makeOrder(t) // amount = 150000
	o.ApplyPromo(uuid.New(), 999999)
	if o.AmountKopecks() != 0 {
		t.Errorf("сумма не должна уйти в минус: %d", o.AmountKopecks())
	}
}

func TestOrder_SetReferralCode(t *testing.T) {
	o := makeOrder(t)
	o.SetReferralCode("REF123")
	if o.ReferralCode() != "REF123" {
		t.Errorf("ReferralCode: %q", o.ReferralCode())
	}
}
