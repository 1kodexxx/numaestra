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
// model — параметр mv TTAPI (пустая строка → DefaultModel, chirp-v5-5).
func NewClientWithBreaker(baseURL, apiKey, model string) APIClient {
	return &withBreaker{
		inner:   NewClient(baseURL, apiKey, model),
		breaker: circuitbreaker.New("suno", 5, 30*time.Second),
	}
}

func (w *withBreaker) CreateMusicTask(ctx context.Context, in MusicInput) (string, error) {
	var taskID string
	err := w.breaker.Do(func() error {
		var err error
		taskID, err = w.inner.CreateMusicTask(ctx, in)
		return err
	})
	return taskID, err
}

func (w *withBreaker) GetTask(ctx context.Context, taskID string) (Task, error) {
	var task Task
	err := w.breaker.Do(func() error {
		var err error
		task, err = w.inner.GetTask(ctx, taskID)
		return err
	})
	return task, err
}
