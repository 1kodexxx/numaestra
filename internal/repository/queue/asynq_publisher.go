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
	// Демо-таски — отдельная полоса на очереди "demo" (низший приоритет), чтобы
	// бесплатные демо никогда не вытесняли платные генерации из пула аккаунтов.
	TaskTypeDemoGenerate = "demo:generate"
	TaskTypeDemoCheck    = "demo:check_status"
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

// DemoTaskPayload — задача запуска демо. Только ID заказа: воркер сам загрузит заказ.
type DemoTaskPayload struct {
	OrderID uuid.UUID `json:"order_id"`
}

// DemoCheckTaskPayload — задача опроса демо. Кроме заказа и suno job несёт
// account_id захваченного слота: демо НЕ хранит его на агрегате заказа (это поле
// принадлежит платному потоку), поэтому account_id путешествует в payload и нужен
// для освобождения слота по завершении опроса.
type DemoCheckTaskPayload struct {
	OrderID   uuid.UUID `json:"order_id"`
	SunoJobID string    `json:"suno_job_id"`
	AccountID uuid.UUID `json:"account_id"`
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
	// MaxRetry для polling: трек генерируется ~2–3 минуты, в пике до 15+ минут.
	// 80 попыток × 15 сек = 20 минут максимального ожидания.
	// При исчерпании ретраев HandleDeadTask переводит заказ в failed с уведомлением.
	if _, err := p.client.EnqueueContext(ctx, task, asynq.Queue("polling"), asynq.MaxRetry(80)); err != nil {
		return fmt.Errorf("постановка задачи проверки статуса в очередь: %w", err)
	}
	return nil
}

// EnqueueDemoTask ставит задачу генерации бесплатного демо на очередь "demo"
// (низший приоритет). TaskID дедуплицирует повторные триггеры одного заказа.
// MaxRetry мал: демо — best-effort, при нехватке аккаунтов лучше тихо не показать
// демо, чем отнимать пул у платных заказов. ErrTaskIDConflict трактуем как успех.
func (p *AsynqPublisher) EnqueueDemoTask(ctx context.Context, orderID uuid.UUID) error {
	payload, err := json.Marshal(DemoTaskPayload{OrderID: orderID})
	if err != nil {
		return fmt.Errorf("сериализация задачи демо: %w", err)
	}
	task := asynq.NewTask(TaskTypeDemoGenerate, payload)
	opts := []asynq.Option{
		asynq.Queue("demo"),
		asynq.MaxRetry(3),
		asynq.TaskID("demo:" + orderID.String()),
	}
	if _, err := p.client.EnqueueContext(ctx, task, opts...); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil // демо для этого заказа уже поставлено — не плодим дубли
		}
		return fmt.Errorf("постановка задачи демо в очередь: %w", err)
	}
	return nil
}

// EnqueueDemoCheckTask ставит задачу опроса демо на очередь "demo".
func (p *AsynqPublisher) EnqueueDemoCheckTask(ctx context.Context, orderID uuid.UUID, sunoJobID string, accountID uuid.UUID) error {
	payload, err := json.Marshal(DemoCheckTaskPayload{OrderID: orderID, SunoJobID: sunoJobID, AccountID: accountID})
	if err != nil {
		return fmt.Errorf("сериализация задачи опроса демо: %w", err)
	}
	task := asynq.NewTask(TaskTypeDemoCheck, payload)
	// 40 × 15 сек = 10 минут ожидания одной демо-задачи (1 клип, обычно быстрее).
	if _, err := p.client.EnqueueContext(ctx, task, asynq.Queue("demo"), asynq.MaxRetry(40)); err != nil {
		return fmt.Errorf("постановка задачи опроса демо в очередь: %w", err)
	}
	return nil
}
