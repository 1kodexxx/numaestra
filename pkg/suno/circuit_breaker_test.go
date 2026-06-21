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
	generateFn func() ([]Clip, error)
	getFeedFn  func() ([]Clip, error)
}

func (m *mockInner) Generate(_ context.Context, _ GenerateRequest) ([]Clip, error) {
	if m.generateFn != nil {
		return m.generateFn()
	}
	return []Clip{{ID: "c1"}}, nil
}

func (m *mockInner) GetFeed(_ context.Context, _ []string) ([]Clip, error) {
	if m.getFeedFn != nil {
		return m.getFeedFn()
	}
	return []Clip{{ID: "c2"}}, nil
}

func TestWithBreaker_Generate_PassesThrough(t *testing.T) {
	wb := &withBreaker{
		inner:   &mockInner{},
		breaker: circuitbreaker.New("test", 5, time.Second),
	}
	clips, err := wb.Generate(context.Background(), GenerateRequest{Prompt: "test"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(clips) != 1 || clips[0].ID != "c1" {
		t.Errorf("неверный ответ: %+v", clips)
	}
}

func TestWithBreaker_GetFeed_PassesThrough(t *testing.T) {
	wb := &withBreaker{
		inner:   &mockInner{},
		breaker: circuitbreaker.New("test", 5, time.Second),
	}
	clips, err := wb.GetFeed(context.Background(), []string{"id1"})
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if len(clips) != 1 || clips[0].ID != "c2" {
		t.Errorf("неверный ответ: %+v", clips)
	}
}

func TestWithBreaker_Generate_ReturnsInnerError(t *testing.T) {
	sentinel := errors.New("suno api error")
	wb := &withBreaker{
		inner:   &mockInner{generateFn: func() ([]Clip, error) { return nil, sentinel }},
		breaker: circuitbreaker.New("test", 5, time.Second),
	}
	_, err := wb.Generate(context.Background(), GenerateRequest{})
	if !errors.Is(err, sentinel) {
		t.Errorf("ожидали sentinel error, получили %v", err)
	}
}

func TestWithBreaker_GetFeed_ReturnsInnerError(t *testing.T) {
	sentinel := errors.New("feed error")
	wb := &withBreaker{
		inner:   &mockInner{getFeedFn: func() ([]Clip, error) { return nil, sentinel }},
		breaker: circuitbreaker.New("test", 5, time.Second),
	}
	_, err := wb.GetFeed(context.Background(), []string{"id1"})
	if !errors.Is(err, sentinel) {
		t.Errorf("ожидали sentinel error, получили %v", err)
	}
}

func TestWithBreaker_OpensAfterThresholdErrors(t *testing.T) {
	const threshold = 3
	callCount := 0
	wb := &withBreaker{
		inner: &mockInner{generateFn: func() ([]Clip, error) {
			callCount++
			return nil, errors.New("suno down")
		}},
		breaker: circuitbreaker.New("test", threshold, time.Minute),
	}

	// threshold ошибок → цепь разомкнута.
	for i := 0; i < threshold; i++ {
		wb.Generate(context.Background(), GenerateRequest{}) //nolint:errcheck
	}

	if callCount != threshold {
		t.Fatalf("ожидали %d вызовов внутреннего клиента, получили %d", threshold, callCount)
	}

	// Следующий вызов должен вернуть ErrCircuitOpen без обращения к inner.
	_, err := wb.Generate(context.Background(), GenerateRequest{})
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Errorf("после %d ошибок ожидали ErrCircuitOpen, получили %v", threshold, err)
	}
	if callCount != threshold {
		t.Errorf("при разомкнутой цепи inner не должен вызываться, callCount=%d", callCount)
	}
}

func TestNewClientWithBreaker_ImplementsAPIClient(t *testing.T) {
	// Статическая проверка: NewClientWithBreaker возвращает APIClient.
	var _ APIClient = NewClientWithBreaker("http://localhost", "key")
}
