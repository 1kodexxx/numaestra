package domain

import (
	"testing"
	"time"
)

func TestNewPromoCode_Valid(t *testing.T) {
	p, err := NewPromoCode("SUMMER20", DiscountTypePercent, 20, "летняя акция")
	if err != nil {
		t.Fatalf("ожидали успех, получили %v", err)
	}
	if p.Code() != "SUMMER20" {
		t.Errorf("неверный код: %q", p.Code())
	}
	if p.GetDiscountType() != DiscountTypePercent {
		t.Errorf("неверный тип скидки: %q", p.GetDiscountType())
	}
	if p.DiscountValue() != 20 {
		t.Errorf("неверное значение скидки: %d", p.DiscountValue())
	}
	if !p.Active() {
		t.Error("новый промокод должен быть активен")
	}
	if p.ID().String() == "" {
		t.Error("должен быть сгенерирован UUID")
	}
}

func TestNewPromoCode_FixedRub(t *testing.T) {
	p, err := NewPromoCode("FLAT500", DiscountTypeFixedRub, 500, "")
	if err != nil {
		t.Fatalf("ожидали успех: %v", err)
	}
	if p.GetDiscountType() != DiscountTypeFixedRub {
		t.Errorf("неверный тип: %q", p.GetDiscountType())
	}
}

func TestNewPromoCode_Validation(t *testing.T) {
	cases := []struct {
		code  string
		dtype DiscountType
		value int
	}{
		{"", DiscountTypePercent, 20},
		{"X", "unknown_type", 20},
		{"X", DiscountTypePercent, 0},
		{"X", DiscountTypePercent, -1},
		{"X", DiscountTypePercent, 101},
	}
	for _, c := range cases {
		_, err := NewPromoCode(c.code, c.dtype, c.value, "")
		if err == nil {
			t.Errorf("ожидали ошибку для code=%q dtype=%q value=%d", c.code, c.dtype, c.value)
		}
	}
}

func TestPromoCode_IsValid(t *testing.T) {
	p, _ := NewPromoCode("CODE", DiscountTypePercent, 10, "")

	if !p.IsValid() {
		t.Error("активный промокод без ограничений должен быть валидным")
	}

	// Деактивирован
	p.SetActive(false)
	if p.IsValid() {
		t.Error("деактивированный промокод не должен быть валидным")
	}
	p.SetActive(true)

	// Лимит исчерпан
	max := 3
	p.SetMaxUses(&max)
	snap := p.Snapshot()
	snap.CurrentUses = 3
	p2 := RestorePromoCode(snap)
	if p2.IsValid() {
		t.Error("исчерпанный лимит — не валиден")
	}

	// validFrom в будущем
	future := time.Now().Add(time.Hour)
	p.SetValidFrom(&future)
	if p.IsValid() {
		t.Error("не начавшийся промокод не должен быть валидным")
	}
	p.SetValidFrom(nil)

	// validUntil в прошлом
	past := time.Now().Add(-time.Hour)
	p.SetValidUntil(&past)
	if p.IsValid() {
		t.Error("истёкший промокод не должен быть валидным")
	}
}

func TestPromoCode_Apply_Percent(t *testing.T) {
	p, _ := NewPromoCode("P10", DiscountTypePercent, 10, "")
	d := p.Apply(150000)
	if d != 15000 {
		t.Errorf("ожидали 15000 (10%% от 150000), получили %d", d)
	}
}

func TestPromoCode_Apply_FixedRub(t *testing.T) {
	p, _ := NewPromoCode("F500", DiscountTypeFixedRub, 500, "")
	d := p.Apply(150000)
	if d != 50000 {
		t.Errorf("ожидали 50000 (500 руб = 50000 коп), получили %d", d)
	}
}

func TestPromoCode_Apply_CapsAtFullAmount(t *testing.T) {
	p, _ := NewPromoCode("F9999", DiscountTypeFixedRub, 9999, "")
	d := p.Apply(1000) // скидка больше суммы
	if d != 1000 {
		t.Errorf("скидка не должна превышать сумму заказа: %d", d)
	}
}

func TestPromoCode_SnapshotRestore(t *testing.T) {
	p, _ := NewPromoCode("SNAP", DiscountTypePercent, 15, "тест снимка")
	future := time.Now().Add(24 * time.Hour)
	p.SetValidUntil(&future)

	snap := p.Snapshot()
	if snap.Code != "SNAP" || snap.DiscountValue != 15 {
		t.Errorf("неверный снимок: %+v", snap)
	}

	restored := RestorePromoCode(snap)
	if restored.Code() != "SNAP" || restored.DiscountValue() != 15 {
		t.Errorf("неверное восстановление: code=%q val=%d", restored.Code(), restored.DiscountValue())
	}
	if restored.ValidUntil() == nil {
		t.Error("validUntil должен восстановиться")
	}
}

func TestPromoCode_Setters(t *testing.T) {
	p, _ := NewPromoCode("SET", DiscountTypePercent, 5, "")
	n := 10
	p.SetMaxUses(&n)
	if p.MaxUses() == nil || *p.MaxUses() != 10 {
		t.Errorf("SetMaxUses не сработал: %v", p.MaxUses())
	}
	p.SetMaxUses(nil)
	if p.MaxUses() != nil {
		t.Error("SetMaxUses(nil) должен обнулить лимит")
	}

	p.SetDescription("новое описание")
	if p.Description() != "новое описание" {
		t.Errorf("SetDescription не сработал: %q", p.Description())
	}

	now := time.Now()
	p.SetValidFrom(&now)
	if p.ValidFrom() == nil {
		t.Error("SetValidFrom не сработал")
	}
	p.SetValidFrom(nil)

	p.SetValidUntil(&now)
	if p.ValidUntil() == nil {
		t.Error("SetValidUntil не сработал")
	}
	p.SetValidUntil(nil)
}

func TestPromoCode_CurrentUsesAndCreatedAt(t *testing.T) {
	snap := PromoCodeSnapshot{
		Code:          "SNAP",
		DiscountType:  DiscountTypePercent,
		DiscountValue: 10,
		IsActive:      true,
		CurrentUses:   3,
		CreatedAt:     time.Now().UTC(),
	}
	p := RestorePromoCode(snap)
	if p.CurrentUses() != 3 {
		t.Errorf("CurrentUses: %d", p.CurrentUses())
	}
	if p.CreatedAt().IsZero() {
		t.Error("CreatedAt не должен быть нулевым")
	}
}
