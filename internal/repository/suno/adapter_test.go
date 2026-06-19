package sunorepo

import (
	"context"
	"errors"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

func TestSubmitGeneration_JoinsClipIDs(t *testing.T) {
	mock := &suno.MockClient{
		GenerateFunc: func(_ context.Context, _ suno.GenerateRequest) ([]suno.Clip, error) {
			return []suno.Clip{{ID: "id1"}, {ID: "id2"}}, nil
		},
	}
	a := NewProviderAdapter(mock)
	jobID, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{Brief: "текст"})
	if err != nil {
		t.Fatalf("SubmitGeneration упал: %v", err)
	}
	if jobID != "id1,id2" {
		t.Errorf("ожидали 'id1,id2', получили %q", jobID)
	}
}

func TestSubmitGeneration_EmptyClips(t *testing.T) {
	mock := &suno.MockClient{
		GenerateFunc: func(_ context.Context, _ suno.GenerateRequest) ([]suno.Clip, error) {
			return nil, nil
		},
	}
	a := NewProviderAdapter(mock)
	if _, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{}); err == nil {
		t.Fatal("ожидали ошибку при пустом списке клипов")
	}
}

func TestSubmitGeneration_ClientError(t *testing.T) {
	mock := &suno.MockClient{
		GenerateFunc: func(_ context.Context, _ suno.GenerateRequest) ([]suno.Clip, error) {
			return nil, errors.New("network")
		},
	}
	a := NewProviderAdapter(mock)
	if _, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{}); err == nil {
		t.Fatal("ожидали проброс ошибки клиента")
	}
}

func TestFetchResult_AllComplete(t *testing.T) {
	mock := &suno.MockClient{
		GetFeedFunc: func(_ context.Context, _ []string) ([]suno.Clip, error) {
			return []suno.Clip{
				{ID: "a", Status: suno.StatusComplete, AudioURL: "u1", Duration: 180},
				{ID: "b", Status: suno.StatusComplete, AudioURL: "u2", Duration: 175},
			}, nil
		},
	}
	a := NewProviderAdapter(mock)
	res, err := a.FetchResult(context.Background(), "a,b")
	if err != nil {
		t.Fatalf("FetchResult упал: %v", err)
	}
	if res.Status != domain.MusicGenerationStatusCompleted {
		t.Errorf("ожидали completed, получили %q", res.Status)
	}
	if len(res.Tracks) != 2 {
		t.Errorf("ожидали 2 трека, получили %d", len(res.Tracks))
	}
}

func TestFetchResult_StillRunning(t *testing.T) {
	mock := &suno.MockClient{
		GetFeedFunc: func(_ context.Context, _ []string) ([]suno.Clip, error) {
			return []suno.Clip{
				{ID: "a", Status: suno.StatusComplete, AudioURL: "u1"},
				{ID: "b", Status: suno.StatusStreaming},
			}, nil
		},
	}
	a := NewProviderAdapter(mock)
	res, _ := a.FetchResult(context.Background(), "a,b")
	if res.Status != domain.MusicGenerationStatusRunning {
		t.Errorf("если хоть один клип не готов — статус running, получили %q", res.Status)
	}
}

func TestFetchResult_HasError(t *testing.T) {
	mock := &suno.MockClient{
		GetFeedFunc: func(_ context.Context, _ []string) ([]suno.Clip, error) {
			return []suno.Clip{
				{ID: "a", Status: suno.StatusError, ErrorMessage: "moderation failed"},
			}, nil
		},
	}
	a := NewProviderAdapter(mock)
	res, _ := a.FetchResult(context.Background(), "a")
	if res.Status != domain.MusicGenerationStatusFailed {
		t.Errorf("ожидали failed, получили %q", res.Status)
	}
	if res.Error != "moderation failed" {
		t.Errorf("ожидали проброс текста ошибки, получили %q", res.Error)
	}
}

func TestFetchResult_EmptyJobID(t *testing.T) {
	a := NewProviderAdapter(&suno.MockClient{})
	res, err := a.FetchResult(context.Background(), "")
	if err != nil {
		t.Fatalf("пустой jobID не должен давать error возврата: %v", err)
	}
	if res.Status != domain.MusicGenerationStatusFailed {
		t.Errorf("пустой jobID должен дать failed, получили %q", res.Status)
	}
}
