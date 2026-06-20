package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func newTestPublisher(t *testing.T) *AsynqPublisher {
	t.Helper()
	mini := miniredis.RunT(t)
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mini.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewAsynqPublisher(client)
}

// Проверяем сериализацию/десериализацию payload — это контракт между
// публикатором и воркером, его легко сломать незаметно.

func TestGenerationTaskPayload_RoundTrip(t *testing.T) {
	id := uuid.New()
	data, err := json.Marshal(GenerationTaskPayload{OrderID: id})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got GenerationTaskPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.OrderID != id {
		t.Errorf("OrderID потерялся: было %s, стало %s", id, got.OrderID)
	}
}

func TestStatusCheckTaskPayload_RoundTrip(t *testing.T) {
	id := uuid.New()
	data, _ := json.Marshal(StatusCheckTaskPayload{OrderID: id, SunoJobID: "job-1,job-2"})
	var got StatusCheckTaskPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.OrderID != id || got.SunoJobID != "job-1,job-2" {
		t.Errorf("payload искажён: %+v", got)
	}
}

func TestPublisher_ImplementsInterface(t *testing.T) {
	// Конструктор не должен паниковать с валидным клиентом.
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: "localhost:6379"})
	defer client.Close()
	p := NewAsynqPublisher(client)
	if p == nil {
		t.Fatal("publisher не должен быть nil")
	}
	_ = context.Background()
}

// --- Integration tests with miniredis ---

func TestEnqueueGenerationTask_Success(t *testing.T) {
	p := newTestPublisher(t)
	orderID := uuid.New()
	if err := p.EnqueueGenerationTask(context.Background(), orderID); err != nil {
		t.Fatalf("EnqueueGenerationTask: %v", err)
	}
}

func TestEnqueueGenerationTask_Idempotent(t *testing.T) {
	p := newTestPublisher(t)
	orderID := uuid.New()
	// Первый вызов — успех.
	if err := p.EnqueueGenerationTask(context.Background(), orderID); err != nil {
		t.Fatalf("первый EnqueueGenerationTask: %v", err)
	}
	// Второй вызов с тем же orderID — TaskID конфликт трактуется как успех (идемпотентность).
	if err := p.EnqueueGenerationTask(context.Background(), orderID); err != nil {
		t.Fatalf("повторный EnqueueGenerationTask должен проходить: %v", err)
	}
}

func TestEnqueueStatusCheckTask_Success(t *testing.T) {
	p := newTestPublisher(t)
	orderID := uuid.New()
	if err := p.EnqueueStatusCheckTask(context.Background(), orderID, "job-abc-123"); err != nil {
		t.Fatalf("EnqueueStatusCheckTask: %v", err)
	}
}

func TestEnqueueGenerationTask_RedisDown_ReturnsError(t *testing.T) {
	mini := miniredis.RunT(t)
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mini.Addr()})
	defer client.Close()
	p := NewAsynqPublisher(client)

	mini.Close() // симулируем потерю соединения с Redis

	if err := p.EnqueueGenerationTask(context.Background(), uuid.New()); err == nil {
		t.Fatal("ожидали ошибку при недоступном Redis")
	}
}

func TestEnqueueStatusCheckTask_RedisDown_ReturnsError(t *testing.T) {
	mini := miniredis.RunT(t)
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mini.Addr()})
	defer client.Close()
	p := NewAsynqPublisher(client)

	mini.Close() // симулируем потерю соединения с Redis

	if err := p.EnqueueStatusCheckTask(context.Background(), uuid.New(), "job-xyz"); err == nil {
		t.Fatal("ожидали ошибку при недоступном Redis")
	}
}
