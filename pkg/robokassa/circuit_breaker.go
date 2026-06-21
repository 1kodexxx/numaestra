package robokassa

import (
	"context"
	"time"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

// RefunderWithBreaker оборачивает *Client circuit breaker'ом для метода Refund.
// PaymentURL и VerifyWebhook — чистые функции без HTTP, circuit breaker им не нужен.
// 3 последовательных ошибки → размыкание на 30 секунд.
type RefunderWithBreaker struct {
	inner   *Client
	breaker *circuitbreaker.Breaker
}

// NewRefunderWithBreaker создаёт Refunder с circuit breaker поверх переданного клиента.
func NewRefunderWithBreaker(c *Client) *RefunderWithBreaker {
	return &RefunderWithBreaker{
		inner:   c,
		breaker: circuitbreaker.New("robokassa-refund", 3, 30*time.Second),
	}
}

func (r *RefunderWithBreaker) Refund(ctx context.Context, outSum string, invID int64) error {
	return r.breaker.Do(func() error {
		return r.inner.Refund(ctx, outSum, invID)
	})
}
