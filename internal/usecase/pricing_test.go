package usecase

import (
	"errors"
	"testing"
)

func TestStaticPricing_PriceFor(t *testing.T) {
	p := NewStaticPricing(map[string]int64{"standard": 150000, "premium": 290000}, "standard")

	if got, err := p.PriceFor("standard"); err != nil || got != 150000 {
		t.Errorf("standard: got %d, err %v", got, err)
	}
	if got, err := p.PriceFor("premium"); err != nil || got != 290000 {
		t.Errorf("premium: got %d, err %v", got, err)
	}
	// Пустой тариф → дефолтный.
	if got, err := p.PriceFor(""); err != nil || got != 150000 {
		t.Errorf("default: got %d, err %v", got, err)
	}
}

func TestStaticPricing_UnknownPlan(t *testing.T) {
	p := NewStaticPricing(map[string]int64{"standard": 150000}, "standard")
	if _, err := p.PriceFor("gold"); !errors.Is(err, ErrUnknownPlan) {
		t.Errorf("ожидали ErrUnknownPlan, получили %v", err)
	}
}

func TestStaticPricing_InvalidPrice(t *testing.T) {
	p := NewStaticPricing(map[string]int64{"broken": 0}, "broken")
	if _, err := p.PriceFor("broken"); err == nil {
		t.Error("ожидали ошибку для нулевой цены тарифа")
	}
}
