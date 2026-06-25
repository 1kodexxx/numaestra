// internal/worker/processor.go
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/numaestra/numaestra/internal/repository/queue"
	"github.com/numaestra/numaestra/internal/usecase"
)

// Таймауты на обработку фоновых задач. Контекст от Asynq сам по себе не
// ограничен по времени, поэтому при зависании внешних API (Suno, S3, LLM)
// задача могла бы держать слот воркера бесконечно. Жёсткий дедлайн гарантирует
// освобождение слота и последующий retry задачи.
const (
	// generateTaskTimeout покрывает запуск генерации в Suno.
	generateTaskTimeout = 90 * time.Second
	// statusCheckTaskTimeout покрывает опрос статуса в Suno и перезаливку
	// готовых треков в S3 (несколько файлов по 3–5 МБ).
	statusCheckTaskTimeout = 3 * time.Minute
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

// taskLogger возвращает логгер с task_id (уникальный Asynq ID, не меняется при retry)
// и order_id — чтобы каждую строчку лога можно было привязать к конкретной задаче.
func (p *OrderProcessor) taskLogger(ctx context.Context, taskType, orderID string) *slog.Logger {
	l := p.log.With("task_type", taskType, "order_id", orderID)
	if taskID, ok := asynq.GetTaskID(ctx); ok {
		l = l.With("task_id", taskID)
	}
	return l
}

// HandleGenerateTask вызывается воркером, когда приходит задача на запуск генерации
func (p *OrderProcessor) HandleGenerateTask(ctx context.Context, t *asynq.Task) error {
	var payload queue.GenerationTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %w: %w", err, asynq.SkipRetry) // SkipRetry отменит задачу навсегда
	}

	log := p.taskLogger(ctx, t.Type(), payload.OrderID.String())
	log.Debug("воркер взял задачу на генерацию")

	ctx, cancel := context.WithTimeout(ctx, generateTaskTimeout)
	defer cancel()

	if err := p.uc.ProcessGenerationTask(ctx, payload.OrderID); err != nil {
		log.Error("ошибка при запуске генерации", "error", err)
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
		p.taskLogger(ctx, t.Type(), "unknown").Error(
			"не удалось извлечь order_id из мёртвой задачи — ручной разбор", "task_err", taskErr)
		return
	}

	log := p.taskLogger(ctx, t.Type(), orderID.String())
	reason := fmt.Sprintf("исчерпаны ретраи задачи %s: %v", t.Type(), taskErr)
	log.Error("задача исчерпала ретраи, переводим заказ в failed", "task_err", taskErr)

	if err := p.uc.FailGeneration(ctx, orderID, reason); err != nil {
		log.Error("не удалось обработать терминальный провал заказа", "err", err)
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
		return fmt.Errorf("json.Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}

	log := p.taskLogger(ctx, t.Type(), payload.OrderID.String())
	log.Debug("воркер проверяет статус генерации", "suno_job_id", payload.SunoJobID)

	ctx, cancel := context.WithTimeout(ctx, statusCheckTaskTimeout)
	defer cancel()

	err := p.uc.CheckGenerationStatus(ctx, payload.OrderID, payload.SunoJobID)
	if err != nil {
		if errors.Is(err, usecase.ErrGenerationNotReady) {
			log.Debug("треки ещё не готовы, откладываем проверку")
			return err
		}
		log.Error("критическая ошибка при проверке статуса", "error", err)
		return err
	}

	return nil
}
