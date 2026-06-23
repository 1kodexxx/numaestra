package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/numaestra/numaestra/internal/domain"
)

// Имена задач Asynq. Хранятся здесь, а не в domain: эта деталь инфраструктуры очреедей.
const (
	TaskTypeGenerateTrack = "suno:generate"
	TaskTypeCheckStatus   = "suno:check_status"
)

// GenerationTaskPayload - полезная нагрузка задачи на генерацию.
// Намеренно содержит только ID заказа: воркер сам загрузит актуальное состояние
// агрегата из репозитория, избегая рассинхронизации с моментом постановки задачи.
type GenerationTaskPayload struct {
	OrderID uuid.UUID `json:"order_id"`
}

// StatusCheckTaskPayload - полезная нагрузка задачи опроса статуса генерации в Suno.
type StatusCheckTaskPayload struct {
	OrderID   uuid.UUID `json:"order_id"`
	SunoJobID string    `json:"suno_job_id"`
}

// AsynqPublisher - реализация domain.QueuePublisher поверх клиента Asynq.
type AsynqPublisher struct {
	client *asynq.Client
}

// NewAsynqPublisher создаёт публикатор задач на основе сконфигурированного клиента Asynq.
func NewAsynqPublisher(client *asynq.Client) *AsynqPublisher {
	return &AsynqPublisher{client: client}
}

// Проверка на этапе компиляции, что AsynqPublisher реализует контракт домена.
var _ domain.QueuePublisher = (*AsynqPublisher)(nil)

func (p *AsynqPublisher) EnqueueGenerationTask(ctx context.Context, orderID uuid.UUID) error {
	payload, err := json.Marshal(GenerationTaskPayload{OrderID: orderID})
	if err != nil {
		return fmt.Errorf("сериализация задачи генерации: %w", err)
	}

	task := asynq.NewTask(TaskTypeGenerateTrack, payload)
	// TaskID = идентификатор заказа: дедупликация на уровне очереди. Если из-за гонки
	// (две доставки вебхука) задача для этого заказа уже стоит в очереди, повторная
	// постановка вернёт ErrTaskIDConflict — трактуем её как успех, чтобы не плодить
	// дубли генерации и двойной расход кредитов Suno.
	opts := []asynq.Option{
		asynq.Queue("generation"),
		asynq.MaxRetry(3),
		asynq.TaskID("generate:" + orderID.String()),
	}
	if _, err := p.client.EnqueueContext(ctx, task, opts...); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("постановка задачи генерации в очередь: %w", err)
	}
	return nil
}

func (p *AsynqPublisher) EnqueueStatusCheckTask(ctx context.Context, orderID uuid.UUID, sunoJobID string) error {
	payload, err := json.Marshal(StatusCheckTaskPayload{OrderID: orderID, SunoJobID: sunoJobID})

	if err != nil {
		return fmt.Errorf("сериализация задачи проверки статуса: %w", err)
	}
	task := asynq.NewTask(TaskTypeCheckStatus, payload)
	// MaxRetry для polling: трек генерируется ~2–3 минуты, опрашиваем каждые 15 сек.
	// 40 попыток × 15 сек = 10 минут максимального ожидания на один трек.
	// Без MaxRetry задача при исчерпании дефолтных ретраев тихо уходит в архив.
	if _, err := p.client.EnqueueContext(ctx, task, asynq.Queue("polling"), asynq.MaxRetry(40)); err != nil {
		return fmt.Errorf("постановка задачи проверки статуса в очередь: %w", err)
	}
	return nil
}
