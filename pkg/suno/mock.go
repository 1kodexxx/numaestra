package suno

import "context"

// MockClient — конфигурируемая заглушка suno.APIClient для тестов и локальной
// разработки без обращения к реальному Sunor API.
//
// Поведение задаётся через CreateMusicTaskFunc / GetTaskFunc. Если функция не
// задана, возвращаются разумные значения по умолчанию (готовая задача с двумя
// клипами), чтобы клиент работал «из коробки».
type MockClient struct {
	CreateMusicTaskFunc func(ctx context.Context, in MusicInput) (string, error)
	GetTaskFunc         func(ctx context.Context, taskID string) (Task, error)
}

var _ APIClient = (*MockClient)(nil)

// CreateMusicTask возвращает результат CreateMusicTaskFunc либо фиктивный task_id.
func (m *MockClient) CreateMusicTask(ctx context.Context, in MusicInput) (string, error) {
	if m.CreateMusicTaskFunc != nil {
		return m.CreateMusicTaskFunc(ctx, in)
	}
	return "mock-task-id", nil
}

// GetTask возвращает результат GetTaskFunc либо успешную задачу с двумя клипами.
func (m *MockClient) GetTask(ctx context.Context, taskID string) (Task, error) {
	if m.GetTaskFunc != nil {
		return m.GetTaskFunc(ctx, taskID)
	}
	return Task{
		ID:     taskID,
		Status: StatusSuccess,
		Clips: []Clip{
			{ID: taskID + "-1", AudioURL: "https://mock.suno.local/" + taskID + "-1.mp3", DurationSec: 180},
			{ID: taskID + "-2", AudioURL: "https://mock.suno.local/" + taskID + "-2.mp3", DurationSec: 175},
		},
	}, nil
}
