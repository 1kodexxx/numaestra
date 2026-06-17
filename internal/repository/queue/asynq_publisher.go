package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/numaestra/numaestra/internal/domain"
)

// Имена задач Asynq. Хранятся здесь, а не в domain: это деталь инфраструктуры очередей.
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
	if _, err := p.client.EnqueueContext(ctx, task, asynq.Queue("generation"), asynq.MaxRetry(3)); err != nil {
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
	if _, err := p.client.EnqueueContext(ctx, task, asynq.Queue("polling")); err != nil {
		return fmt.Errorf("постановка задачи проверки статуса в очередь: %w", err)
	}
	return nil
}
