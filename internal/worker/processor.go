// internal/worker/processor.go
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/numaestra/numaestra/internal/repository/queue"
	"github.com/numaestra/numaestra/internal/usecase"
)

// OrderProcessor содержит логику обработки фоновых задач из Asynq
type OrderProcessor struct {
	uc  *usecase.OrderUseCase
	log *slog.Logger
}

func NewOrderProcessor(uc *usecase.OrderUseCase, log *slog.Logger) *OrderProcessor {
	return &OrderProcessor{
		uc:  uc,
		log: log,
	}
}

// HandleGenerateTask вызывается воркером, когда приходит задача на запуск генерации
func (p *OrderProcessor) HandleGenerateTask(ctx context.Context, t *asynq.Task) error {
	var payload queue.GenerationTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry) // SkipRetry отменит задачу навсегда
	}

	p.log.Debug("воркер взял задачу на генерацию", "order_id", payload.OrderID)

	err := p.uc.ProcessGenerationTask(ctx, payload.OrderID)
	if err != nil {
		p.log.Error("ошибка при запуске генерации", "order_id", payload.OrderID, "error", err)
		return err // Возвращаем ошибку, чтобы Asynq сделал Retry
	}

	return nil
}

// HandleStatusCheckTask вызывается воркером для периодического опроса статуса
func (p *OrderProcessor) HandleStatusCheckTask(ctx context.Context, t *asynq.Task) error {
	var payload queue.StatusCheckTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	p.log.Debug("воркер проверяет статус генерации", "order_id", payload.OrderID, "suno_job_id", payload.SunoJobID)

	err := p.uc.CheckGenerationStatus(ctx, payload.OrderID, payload.SunoJobID)
	if err != nil {
		// Если статус еще генерируется (не ошибка провайдера, а просто нужно подождать)
		if errors.Is(err, usecase.ErrGenerationNotReady) {
			p.log.Debug("треки еще не готовы, откладываем проверку...", "order_id", payload.OrderID)
			// Мы возвращаем кастомную ошибку, чтобы сработал RetryDelayFunc (см. ниже)
			return err
		}

		p.log.Error("критическая ошибка при проверке статуса", "order_id", payload.OrderID, "error", err)
		return err
	}

	return nil
}
