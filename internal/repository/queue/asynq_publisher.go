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

// queuesWithGenTasks — очереди, в которых может находиться задача генерации.
// Inspector.RunTask требует явного имени очереди, поэтому проверяем все возможные.
var queuesWithGenTasks = []string{"generation", "default"}

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
	client    *asynq.Client
	redisConn asynq.RedisConnOpt // нужен Inspector для rescue задач из retry
}

// NewAsynqPublisher создаёт публикатор задач на основе сконфигурированного клиента Asynq.
func NewAsynqPublisher(client *asynq.Client, redisConn asynq.RedisConnOpt) *AsynqPublisher {
	return &AsynqPublisher{client: client, redisConn: redisConn}
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
	//
	// MaxRetry(20): заказ уже оплачен, поэтому исчерпание ретраев — это автоматический
	// permanent fail (см. worker.HandleDeadTask), требующий ручной перегенерации
	// администратором. ErrNoAvailableAccount (пул занят) — частый и проходящий за
	// минуты случай, а не поломка конкретного заказа; с MaxRetry(3) и стандартным
	// backoff Asynq заказ падал в failed уже через 5-10 минут банальной занятости
	// пула. 20 ретраев с фиксированной задержкой для ErrNoAvailableAccount (см.
	// RetryDelayFunc в cmd/server/main.go) дают многочасовое окно терпения, прежде
	// чем по-настоящему сломанный заказ потребует внимания администратора.
	opts := []asynq.Option{
		asynq.Queue("generation"),
		asynq.MaxRetry(20),
		asynq.TaskID("generate:" + orderID.String()),
	}
	if _, err := p.client.EnqueueContext(ctx, task, opts...); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			// Задача с таким ID уже существует в Asynq (pending, retry или scheduled).
			// При нормальной работе (дубль вебхука) это ожидаемо — молча игнорируем.
			// При recovery (задача застряла в retry с длинным backoff) мы хотим
			// запустить её немедленно, не дожидаясь таймаута backoff. Inspector.RunTask
			// перекладывает задачу из retry/scheduled → pending без изменения payload
			// и счётчика попыток.
			return p.rescueFromRetry("generate:" + orderID.String())
		}
		return fmt.Errorf("постановка задачи генерации в очередь: %w", err)
	}
	return nil
}

// rescueFromRetry немедленно активирует задачу из retry/scheduled, если она там есть.
// Используется когда задача с данным ID уже существует, но застряла в backoff.
// Проверяет все возможные очереди — Inspector.RunTask требует знать имя очереди явно.
// Если задача в pending/active — RunTask вернёт ошибку; это нормально, значит она
// уже будет обработана скоро. Возвращаем nil во всех случаях: вызывающей стороне
// достаточно знать, что задача "есть и будет обработана".
func (p *AsynqPublisher) rescueFromRetry(taskID string) error {
	if p.redisConn == nil {
		return nil
	}
	inspector := asynq.NewInspector(p.redisConn)
	defer inspector.Close() //nolint:errcheck
	for _, q := range queuesWithGenTasks {
		if err := inspector.RunTask(q, taskID); err == nil {
			return nil // задача переведена в pending, воркер возьмёт её немедленно
		}
	}
	return nil // задача уже в pending/active или не найдена — в любом случае не блокируем
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
