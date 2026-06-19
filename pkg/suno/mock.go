package suno

import "context"

// MockClient — конфигурируемая заглушка suno.APIClient для тестов и локальной
// разработки без обращения к реальному Suno API.
//
// Поведение задаётся через функции GenerateFunc / GetFeedFunc. Если функция не
// задана, возвращаются разумные значения по умолчанию (два готовых клипа),
// чтобы клиент можно было использовать «из коробки».
type MockClient struct {
	GenerateFunc func(ctx context.Context, req GenerateRequest) ([]Clip, error)
	GetFeedFunc  func(ctx context.Context, ids []string) ([]Clip, error)
}

var _ APIClient = (*MockClient)(nil)

// Generate возвращает результат GenerateFunc либо два клипа в статусе submitted по умолчанию.
func (m *MockClient) Generate(ctx context.Context, req GenerateRequest) ([]Clip, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, req)
	}
	return []Clip{
		{ID: "mock-clip-1", Status: StatusSubmitted},
		{ID: "mock-clip-2", Status: StatusSubmitted},
	}, nil
}

// GetFeed возвращает результат GetFeedFunc либо готовые клипы по запрошенным ids по умолчанию.
func (m *MockClient) GetFeed(ctx context.Context, ids []string) ([]Clip, error) {
	if m.GetFeedFunc != nil {
		return m.GetFeedFunc(ctx, ids)
	}
	clips := make([]Clip, 0, len(ids))
	for _, id := range ids {
		clips = append(clips, Clip{
			ID:       id,
			Status:   StatusComplete,
			AudioURL: "https://mock.suno.local/" + id + ".mp3",
			Duration: 180,
		})
	}
	return clips, nil
}
