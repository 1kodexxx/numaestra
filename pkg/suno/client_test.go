package suno

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClient_CreateMusicTask_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/task" {
			t.Errorf("неверный путь: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("ожидали POST, получили %s", r.Method)
		}
		if k := r.Header.Get("x-api-key"); k != "secret" {
			t.Errorf("ожидали x-api-key=secret, получили %q", k)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"gpt_description_prompt":"песня про лето"`) {
			t.Errorf("тело не содержит описание: %s", body)
		}
		if !strings.Contains(string(body), `"task_type":"music"`) {
			t.Errorf("тело не содержит task_type music: %s", body)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":202,"data":{"task_id":"task-123","status":"pending"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	id, err := c.CreateMusicTask(context.Background(), MusicInput{Description: "песня про лето"})
	if err != nil {
		t.Fatalf("CreateMusicTask упал: %v", err)
	}
	if id != "task-123" {
		t.Errorf("ожидали task-123, получили %q", id)
	}
}

func TestHTTPClient_CreateMusicTask_ErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrInvalidAPIKey},
		{http.StatusPaymentRequired, ErrInsufficientCredits},
		{http.StatusTooManyRequests, ErrRateLimited},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))
		c := NewClient(srv.URL, "k")
		_, err := c.CreateMusicTask(context.Background(), MusicInput{Description: "x"})
		if !errors.Is(err, tc.want) {
			t.Errorf("статус %d: ожидали %v, получили %v", tc.status, tc.want, err)
		}
		srv.Close()
	}
}

func TestHTTPClient_CreateMusicTask_EmptyTaskID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":202,"data":{"task_id":"","status":"pending"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if _, err := c.CreateMusicTask(context.Background(), MusicInput{Description: "x"}); err == nil {
		t.Fatal("ожидали ошибку при пустом task_id")
	}
}

func TestHTTPClient_GetTask_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/task/task-123" {
			t.Errorf("неверный путь: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("ожидали GET, получили %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"task-123","status":"success","error":null,
			"output":{"task_type":"music","status":"completed","fail_reason":null,"result":[
				{"id":"clip-1","audio_url":"https://cdn/clip-1.mp3","metadata":{"duration":120}},
				{"id":"clip-2","audio_url":"https://cdn/clip-2.mp3","metadata":{"duration":118}}
			]}}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	task, err := c.GetTask(context.Background(), "task-123")
	if err != nil {
		t.Fatalf("GetTask упал: %v", err)
	}
	if task.Status != StatusSuccess {
		t.Errorf("ожидали success, получили %q", task.Status)
	}
	if len(task.Clips) != 2 {
		t.Fatalf("ожидали 2 клипа, получили %d", len(task.Clips))
	}
	if task.Clips[0].AudioURL != "https://cdn/clip-1.mp3" || task.Clips[0].DurationSec != 120 {
		t.Errorf("неверно разобран клип: %+v", task.Clips[0])
	}
}

func TestHTTPClient_GetTask_Running(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"t","status":"running",
			"output":{"task_type":"music","status":"processing","progress":"50%","result":null}}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	task, err := c.GetTask(context.Background(), "t")
	if err != nil {
		t.Fatalf("GetTask упал: %v", err)
	}
	if task.Status != StatusRunning {
		t.Errorf("ожидали running, получили %q", task.Status)
	}
	if len(task.Clips) != 0 {
		t.Errorf("у незавершённой задачи не должно быть клипов, получили %d", len(task.Clips))
	}
}

func TestHTTPClient_GetTask_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"data":{"task_id":"t","status":"failure",
			"error":"Upstream provider returned an error",
			"output":{"task_type":"music","status":"failed","fail_reason":"moderation failed","result":null}}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	task, err := c.GetTask(context.Background(), "t")
	if err != nil {
		t.Fatalf("GetTask упал: %v", err)
	}
	if task.Status != StatusFailure {
		t.Errorf("ожидали failure, получили %q", task.Status)
	}
	// fail_reason приоритетнее общего error.
	if task.FailReason != "moderation failed" {
		t.Errorf("ожидали 'moderation failed', получили %q", task.FailReason)
	}
}

func TestHTTPClient_GetTask_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if _, err := c.GetTask(context.Background(), "missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("ожидали ErrTaskNotFound, получили %v", err)
	}
}

func TestHTTPClient_GetTask_EmptyID(t *testing.T) {
	c := NewClient("http://unused", "k")
	if _, err := c.GetTask(context.Background(), ""); err == nil {
		t.Fatal("пустой taskID должен давать ошибку")
	}
}

func TestHTTPClient_NoAPIKey_OmitsHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "" {
			t.Errorf("при пустом ключе заголовок x-api-key не должен ставиться")
		}
		_, _ = w.Write([]byte(`{"code":202,"data":{"task_id":"t","status":"pending"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	if _, err := c.CreateMusicTask(context.Background(), MusicInput{Description: "x"}); err != nil {
		t.Fatalf("CreateMusicTask упал: %v", err)
	}
}

func TestHTTPClient_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // следующий запрос вернёт сетевую ошибку

	c := NewClient(srv.URL, "k")
	if _, err := c.GetTask(context.Background(), "t"); err == nil {
		t.Fatal("ожидали сетевую ошибку при закрытом сервере")
	}
}

func TestHTTPClient_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{broken"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if _, err := c.GetTask(context.Background(), "t"); err == nil {
		t.Fatal("ожидали ошибку декодирования невалидного JSON")
	}
}

// --- MockClient ---

func TestMockClient_Defaults(t *testing.T) {
	m := &MockClient{}
	id, err := m.CreateMusicTask(context.Background(), MusicInput{})
	if err != nil || id == "" {
		t.Fatalf("дефолтный CreateMusicTask должен вернуть id без ошибки: %v, %q", err, id)
	}
	task, err := m.GetTask(context.Background(), "tid")
	if err != nil {
		t.Fatalf("дефолтный GetTask упал: %v", err)
	}
	if task.Status != StatusSuccess || len(task.Clips) != 2 {
		t.Errorf("дефолтный GetTask должен вернуть success c 2 клипами, получили %+v", task)
	}
}

func TestMockClient_CustomFuncs(t *testing.T) {
	m := &MockClient{
		CreateMusicTaskFunc: func(_ context.Context, _ MusicInput) (string, error) {
			return "custom-id", nil
		},
		GetTaskFunc: func(_ context.Context, id string) (Task, error) {
			return Task{ID: id, Status: StatusFailure, FailReason: "boom"}, nil
		},
	}
	id, _ := m.CreateMusicTask(context.Background(), MusicInput{})
	if id != "custom-id" {
		t.Errorf("должна вызываться кастомная CreateMusicTaskFunc, получили %q", id)
	}
	task, _ := m.GetTask(context.Background(), "z")
	if task.Status != StatusFailure || task.FailReason != "boom" {
		t.Errorf("должна вызываться кастомная GetTaskFunc, получили %+v", task)
	}
}
