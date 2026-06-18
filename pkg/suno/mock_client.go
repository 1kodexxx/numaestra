package suno

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// MockClient - реализация Client в памяти. Позволяет разрабатывать и тестировать
// весь use-case слой и воркер до того, как выбран и подключён реальный провайдер.
type MockClient struct {
	mu   sync.Mutex
	jobs map[string]Job
}

func NewMockClient() *MockClient {
	return &MockClient{jobs: make(map[string]Job)}
}

func (m *MockClient) Generate(_ context.Context, req GenerateRequest) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	trackCount := req.TrackCount
	if trackCount == 0 {
		trackCount = 4
	}

	tracks := make([]Track, trackCount)
	for i := range tracks {
		tracks[i] = Track{
			ID:          uuid.NewString(),
			AudioURL:    "https://mock.local/tracks/" + uuid.NewString() + ".mp3",
			DurationSec: 180,
		}
	}

	job := Job{
		ID:     uuid.NewString(),
		Status: JobStatusCompleted, // мок завершает генерацию мгновенно для упрощения локальной разработки
		Tracks: tracks,
	}
	m.jobs[job.ID] = job
	return job, nil
}

func (m *MockClient) GetJob(_ context.Context, jobID string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return Job{}, ErrJobNotFound
	}
	return job, nil
}
