package usecase

import (
	"errors"
	"fmt"
)

// ErrUnknownPlan возвращается, когда запрошен несуществующий тариф.
var ErrUnknownPlan = errors.New("неизвестный тариф")

// Pricing определяет цену заказа на стороне сервера.
//
// КРИТИЧНО: цену НЕЛЬЗЯ принимать из тела запроса клиента — иначе любой может
// создать заказ «студийной песни» за 1 копейку, оплатить его, и сверка суммы
// в вебхуке пройдёт успешно (сумма заказа подконтрольна злоумышленнику).
// Клиент выбирает только тариф (plan), а конкретную сумму определяет сервер.
type Pricing interface {
	PriceFor(plan string) (kopecks int64, err error)
}

// StaticPricing — прайс из статической таблицы тарифов, задаваемой конфигом.
type StaticPricing struct {
	plans       map[string]int64
	defaultPlan string
}

// NewStaticPricing создаёт прайс. defaultPlan используется, если клиент не указал тариф.
func NewStaticPricing(plans map[string]int64, defaultPlan string) *StaticPricing {
	return &StaticPricing{plans: plans, defaultPlan: defaultPlan}
}

func (p *StaticPricing) PriceFor(plan string) (int64, error) {
	if plan == "" {
		plan = p.defaultPlan
	}
	kopecks, ok := p.plans[plan]
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrUnknownPlan, plan)
	}
	if kopecks <= 0 {
		return 0, fmt.Errorf("некорректная цена тарифа %q", plan)
	}
	return kopecks, nil
}

var _ Pricing = (*StaticPricing)(nil)
