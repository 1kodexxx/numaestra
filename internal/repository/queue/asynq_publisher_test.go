package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

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
