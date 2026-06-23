package suno

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

// mockInner — заглушка APIClient для тестирования withBreaker.
type mockInner struct {
	createFn func() (string, error)
	getFn    func() (Task, error)
}

func (m *mockInner) CreateMusicTask(_ context.Context, _ MusicInput) (string, error) {
	if m.createFn != nil {
		return m.createFn()
	}
	return "task-1", nil
}

func (m *mockInner) GetTask(_ context.Context, _ string) (Task, error) {
	if m.getFn != nil {
		return m.getFn()
	}
	return Task{ID: "task-1", Status: StatusSuccess}, nil
}

func TestWithBreaker_CreateMusicTask_PassesThrough(t *testing.T) {
	wb := &withBreaker{inner: &mockInner{}, breaker: circuitbreaker.New("test", 5, time.Second)}
	id, err := wb.CreateMusicTask(context.Background(), MusicInput{Description: "test"})
	if err != nil {
		t.Fatalf("CreateMusicTask: %v", err)
	}
	if id != "task-1" {
		t.Errorf("неверный ответ: %q", id)
	}
}

func TestWithBreaker_GetTask_PassesThrough(t *testing.T) {
	wb := &withBreaker{inner: &mockInner{}, breaker: circuitbreaker.New("test", 5, time.Second)}
	task, err := wb.GetTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Status != StatusSuccess {
		t.Errorf("неверный ответ: %+v", task)
	}
}

func TestWithBreaker_CreateMusicTask_ReturnsInnerError(t *testing.T) {
	sentinel := errors.New("suno api error")
	wb := &withBreaker{
		inner:   &mockInner{createFn: func() (string, error) { return "", sentinel }},
		breaker: circuitbreaker.New("test", 5, time.Second),
	}
	if _, err := wb.CreateMusicTask(context.Background(), MusicInput{}); !errors.Is(err, sentinel) {
		t.Errorf("ожидали sentinel error, получили %v", err)
	}
}

func TestWithBreaker_GetTask_ReturnsInnerError(t *testing.T) {
	sentinel := errors.New("get task error")
	wb := &withBreaker{
		inner:   &mockInner{getFn: func() (Task, error) { return Task{}, sentinel }},
		breaker: circuitbreaker.New("test", 5, time.Second),
	}
	if _, err := wb.GetTask(context.Background(), "id"); !errors.Is(err, sentinel) {
		t.Errorf("ожидали sentinel error, получили %v", err)
	}
}

func TestWithBreaker_OpensAfterThresholdErrors(t *testing.T) {
	const threshold = 3
	callCount := 0
	wb := &withBreaker{
		inner: &mockInner{createFn: func() (string, error) {
			callCount++
			return "", errors.New("suno down")
		}},
		breaker: circuitbreaker.New("test", threshold, time.Minute),
	}

	for i := 0; i < threshold; i++ {
		wb.CreateMusicTask(context.Background(), MusicInput{}) //nolint:errcheck
	}
	if callCount != threshold {
		t.Fatalf("ожидали %d вызовов внутреннего клиента, получили %d", threshold, callCount)
	}

	_, err := wb.CreateMusicTask(context.Background(), MusicInput{})
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Errorf("после %d ошибок ожидали ErrCircuitOpen, получили %v", threshold, err)
	}
	if callCount != threshold {
		t.Errorf("при разомкнутой цепи inner не должен вызываться, callCount=%d", callCount)
	}
}

func TestNewClientWithBreaker_ImplementsAPIClient(t *testing.T) {
	var _ APIClient = NewClientWithBreaker("http://localhost", "key")
}
