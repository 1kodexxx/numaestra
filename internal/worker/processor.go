// internal/worker/processor.go
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
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

// HandleDeadTask вызывается из asynq.ErrorHandler, когда у задачи исчерпаны все
// ретраи и она отправляется в архив. Без этого аккаунт навсегда застрянет в Busy,
// а заказ — в processing/queued. Переводим заказ в failed и освобождаем аккаунт.
func (p *OrderProcessor) HandleDeadTask(ctx context.Context, t *asynq.Task, taskErr error) {
	orderID, ok := orderIDFromTask(t)
	if !ok {
		p.log.Error("не удалось извлечь order_id из мёртвой задачи — ручной разбор",
			"task_type", t.Type(), "task_err", taskErr)
		return
	}

	reason := fmt.Sprintf("исчерпаны ретраи задачи %s: %v", t.Type(), taskErr)
	p.log.Error("задача исчерпала ретраи, переводим заказ в failed",
		"order_id", orderID, "task_type", t.Type(), "task_err", taskErr)

	if err := p.uc.FailGeneration(ctx, orderID, reason); err != nil {
		p.log.Error("не удалось обработать терминальный провал заказа",
			"order_id", orderID, "err", err)
	}
}

// orderIDFromTask извлекает идентификатор заказа из payload любой из фоновых задач.
func orderIDFromTask(t *asynq.Task) (uuid.UUID, bool) {
	switch t.Type() {
	case queue.TaskTypeGenerateTrack:
		var pl queue.GenerationTaskPayload
		if err := json.Unmarshal(t.Payload(), &pl); err != nil {
			return uuid.Nil, false
		}
		return pl.OrderID, pl.OrderID != uuid.Nil
	case queue.TaskTypeCheckStatus:
		var pl queue.StatusCheckTaskPayload
		if err := json.Unmarshal(t.Payload(), &pl); err != nil {
			return uuid.Nil, false
		}
		return pl.OrderID, pl.OrderID != uuid.Nil
	default:
		return uuid.Nil, false
	}
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
