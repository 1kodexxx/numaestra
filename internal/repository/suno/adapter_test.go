package sunorepo

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

func TestSubmitGeneration_TwoLyricBriefs(t *testing.T) {
	var calls int32
	var inputs []suno.MusicInput
	mock := &suno.MockClient{
		CreateMusicTaskFunc: func(_ context.Context, in suno.MusicInput) (string, error) {
			inputs = append(inputs, in)
			n := atomic.AddInt32(&calls, 1)
			return "task-" + string('0'+n), nil
		},
	}
	a := NewProviderAdapter(mock)

	jobID, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{
		Briefs:     []string{"текст A", "текст B"},
		TrackCount: domain.DefaultTrackCount,
	})
	if err != nil {
		t.Fatalf("SubmitGeneration упал: %v", err)
	}
	if calls != 2 {
		t.Errorf("ожидали 2 задачи (по одной на текст), создано %d", calls)
	}
	if len(inputs) != 2 || inputs[0].Description != "текст A" || inputs[1].Description != "текст B" {
		t.Errorf("разные briefs не проброшены: %+v", inputs)
	}
	if jobID != "task-1,task-2" {
		t.Errorf("ожидали 'task-1,task-2', получили %q", jobID)
	}
}

func TestSubmitGeneration_SubmitsMultipleTasksForFourVersions(t *testing.T) {
	var calls int32
	mock := &suno.MockClient{
		CreateMusicTaskFunc: func(_ context.Context, in suno.MusicInput) (string, error) {
			if in.Description != "текст" {
				t.Errorf("описание не проброшено: %q", in.Description)
			}
			n := atomic.AddInt32(&calls, 1)
			return "task-" + string('0'+n), nil
		},
	}
	a := NewProviderAdapter(mock)

	jobID, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{
		Brief:      "текст",
		TrackCount: domain.DefaultTrackCount, // 4 версии → 2 задачи по 2 клипа
	})
	if err != nil {
		t.Fatalf("SubmitGeneration упал: %v", err)
	}
	if calls != 2 {
		t.Errorf("для 4 версий ожидали 2 задачи, создано %d", calls)
	}
	if jobID != "task-1,task-2" {
		t.Errorf("ожидали 'task-1,task-2', получили %q", jobID)
	}
}

func TestSubmitGeneration_PartialSuccess(t *testing.T) {
	var calls int32
	mock := &suno.MockClient{
		CreateMusicTaskFunc: func(_ context.Context, _ suno.MusicInput) (string, error) {
			// Первая задача создаётся, вторая падает — заказ всё равно стартует.
			if atomic.AddInt32(&calls, 1) == 1 {
				return "task-ok", nil
			}
			return "", errors.New("temporary")
		},
	}
	a := NewProviderAdapter(mock)

	jobID, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{TrackCount: 4})
	if err != nil {
		t.Fatalf("частичный успех не должен возвращать ошибку: %v", err)
	}
	if jobID != "task-ok" {
		t.Errorf("ожидали единственный успешный id, получили %q", jobID)
	}
}

func TestSubmitGeneration_AllFail_MapsInvalidKey(t *testing.T) {
	mock := &suno.MockClient{
		CreateMusicTaskFunc: func(_ context.Context, _ suno.MusicInput) (string, error) {
			return "", suno.ErrInvalidAPIKey
		},
	}
	a := NewProviderAdapter(mock)

	_, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{TrackCount: 4})
	if !errors.Is(err, domain.ErrProviderSessionExpired) {
		t.Fatalf("401 по всем задачам должен маппиться в ErrProviderSessionExpired, получили %v", err)
	}
}

func TestSubmitGeneration_AllFail_OtherError(t *testing.T) {
	mock := &suno.MockClient{
		CreateMusicTaskFunc: func(_ context.Context, _ suno.MusicInput) (string, error) {
			return "", errors.New("network")
		},
	}
	a := NewProviderAdapter(mock)

	if _, err := a.SubmitGeneration(context.Background(), domain.MusicGenerationRequest{TrackCount: 4}); err == nil {
		t.Fatal("ожидали ошибку, когда все задачи провалились")
	}
}

func TestFetchResult_AllComplete_AggregatesClips(t *testing.T) {
	mock := &suno.MockClient{
		GetTaskFunc: func(_ context.Context, id string) (suno.Task, error) {
			return suno.Task{
				ID:     id,
				Status: suno.StatusSuccess,
				Clips: []suno.Clip{
					{ID: id + "-a", AudioURL: "u-" + id + "-a", DurationSec: 180},
					{ID: id + "-b", AudioURL: "u-" + id + "-b", DurationSec: 175},
				},
			}, nil
		},
	}
	a := NewProviderAdapter(mock)

	res, err := a.FetchResult(context.Background(), "t1,t2")
	if err != nil {
		t.Fatalf("FetchResult упал: %v", err)
	}
	if res.Status != domain.MusicGenerationStatusCompleted {
		t.Errorf("ожидали completed, получили %q", res.Status)
	}
	if len(res.Tracks) != 4 {
		t.Errorf("ожидали 4 трека (2 задачи × 2 клипа), получили %d", len(res.Tracks))
	}
}

func TestFetchResult_StillRunning(t *testing.T) {
	mock := &suno.MockClient{
		GetTaskFunc: func(_ context.Context, id string) (suno.Task, error) {
			if id == "t1" {
				return suno.Task{ID: id, Status: suno.StatusSuccess, Clips: []suno.Clip{{ID: "c", AudioURL: "u"}}}, nil
			}
			return suno.Task{ID: id, Status: suno.StatusRunning}, nil
		},
	}
	a := NewProviderAdapter(mock)

	res, _ := a.FetchResult(context.Background(), "t1,t2")
	if res.Status != domain.MusicGenerationStatusRunning {
		t.Errorf("пока хоть одна задача не готова — running, получили %q", res.Status)
	}
}

func TestFetchResult_PartialFailureStillCompletes(t *testing.T) {
	mock := &suno.MockClient{
		GetTaskFunc: func(_ context.Context, id string) (suno.Task, error) {
			if id == "t1" {
				return suno.Task{ID: id, Status: suno.StatusSuccess, Clips: []suno.Clip{
					{ID: "c1", AudioURL: "u1", DurationSec: 180},
					{ID: "c2", AudioURL: "u2", DurationSec: 175},
				}}, nil
			}
			return suno.Task{ID: id, Status: suno.StatusFailure, FailReason: "moderation"}, nil
		},
	}
	a := NewProviderAdapter(mock)

	// Одна задача упала, но есть 2 клипа — выдаём что есть, а не валим заказ.
	res, _ := a.FetchResult(context.Background(), "t1,t2")
	if res.Status != domain.MusicGenerationStatusCompleted {
		t.Errorf("при наличии клипов ожидали completed, получили %q", res.Status)
	}
	if len(res.Tracks) != 2 {
		t.Errorf("ожидали 2 трека от успешной задачи, получили %d", len(res.Tracks))
	}
}

func TestFetchResult_AllFailed(t *testing.T) {
	mock := &suno.MockClient{
		GetTaskFunc: func(_ context.Context, id string) (suno.Task, error) {
			return suno.Task{ID: id, Status: suno.StatusFailure, FailReason: "moderation failed"}, nil
		},
	}
	a := NewProviderAdapter(mock)

	res, _ := a.FetchResult(context.Background(), "t1,t2")
	if res.Status != domain.MusicGenerationStatusFailed {
		t.Errorf("ожидали failed, получили %q", res.Status)
	}
	if res.Error != "moderation failed" {
		t.Errorf("ожидали проброс причины, получили %q", res.Error)
	}
}

func TestFetchResult_ClientError(t *testing.T) {
	mock := &suno.MockClient{
		GetTaskFunc: func(_ context.Context, _ string) (suno.Task, error) {
			return suno.Task{}, errors.New("network")
		},
	}
	a := NewProviderAdapter(mock)

	if _, err := a.FetchResult(context.Background(), "t1"); err == nil {
		t.Fatal("ожидали проброс ошибки клиента")
	}
}

func TestFetchResult_EmptyJobID(t *testing.T) {
	a := NewProviderAdapter(&suno.MockClient{})
	res, err := a.FetchResult(context.Background(), "")
	if err != nil {
		t.Fatalf("пустой jobID не должен возвращать error: %v", err)
	}
	if res.Status != domain.MusicGenerationStatusFailed {
		t.Errorf("пустой jobID должен дать failed, получили %q", res.Status)
	}
}
