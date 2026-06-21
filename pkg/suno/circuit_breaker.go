package suno

import (
	"context"
	"time"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

type withBreaker struct {
	inner   APIClient
	breaker *circuitbreaker.Breaker
}

// NewClientWithBreaker создаёт Suno APIClient с защитой circuit breaker.
// 5 последовательных ошибок → размыкание на 30 секунд.
func NewClientWithBreaker(baseURL, apiKey string) APIClient {
	return &withBreaker{
		inner:   NewClient(baseURL, apiKey),
		breaker: circuitbreaker.New("suno", 5, 30*time.Second),
	}
}

func (w *withBreaker) Generate(ctx context.Context, req GenerateRequest) ([]Clip, error) {
	var result []Clip
	err := w.breaker.Do(func() error {
		var err error
		result, err = w.inner.Generate(ctx, req)
		return err
	})
	return result, err
}

func (w *withBreaker) GetFeed(ctx context.Context, ids []string) ([]Clip, error) {
	var result []Clip
	err := w.breaker.Do(func() error {
		var err error
		result, err = w.inner.GetFeed(ctx, ids)
		return err
	})
	return result, err
}
