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
		if r.URL.Path != "/suno/v1/music" {
			t.Errorf("неверный путь: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("ожидали POST, получили %s", r.Method)
		}
		if k := r.Header.Get("TT-API-KEY"); k != "secret" {
			t.Errorf("ожидали TT-API-KEY=secret, получили %q", k)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"gpt_description_prompt":"песня про лето"`) {
			t.Errorf("тело не содержит gpt_description_prompt: %s", body)
		}
		if !strings.Contains(string(body), `"mv":"chirp-v5"`) {
			t.Errorf("тело не содержит mv chirp-v5: %s", body)
		}
		if !strings.Contains(string(body), `"custom":false`) {
			t.Errorf("тело не содержит custom=false: %s", body)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"SUCCESS","message":"success","data":{"jobId":"job-123"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	id, err := c.CreateMusicTask(context.Background(), MusicInput{Description: "песня про лето"})
	if err != nil {
		t.Fatalf("CreateMusicTask упал: %v", err)
	}
	if id != "job-123" {
		t.Errorf("ожидали job-123, получили %q", id)
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

func TestHTTPClient_CreateMusicTask_EmptyJobID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"SUCCESS","message":"success","data":{"jobId":""}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if _, err := c.CreateMusicTask(context.Background(), MusicInput{Description: "x"}); err == nil {
		t.Fatal("ожидали ошибку при пустом jobId")
	}
}

func TestHTTPClient_GetTask_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/suno/v2/fetch" {
			t.Errorf("неверный путь: %s", r.URL.Path)
		}
		if r.URL.Query().Get("jobId") != "job-123" {
			t.Errorf("неверный jobId: %s", r.URL.Query().Get("jobId"))
		}
		if r.Method != http.MethodGet {
			t.Errorf("ожидали GET, получили %s", r.Method)
		}
		_, _ = w.Write([]byte(`{
			"status":"SUCCESS","message":"success",
			"data":{
				"jobId":"job-123",
				"musics":[
					{"musicId":"clip-1","audioUrl":"https://cdn/clip-1.mp3","duration":120},
					{"musicId":"clip-2","audioUrl":"https://cdn/clip-2.mp3","duration":118}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	task, err := c.GetTask(context.Background(), "job-123")
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
		_, _ = w.Write([]byte(`{"status":"ON_QUEUE","message":"processing","data":{"jobId":"t","musics":null}}`))
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
		_, _ = w.Write([]byte(`{"status":"FAILED","message":"moderation failed","data":{"jobId":"t","musics":null}}`))
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
		if r.Header.Get("TT-API-KEY") != "" {
			t.Errorf("при пустом ключе заголовок TT-API-KEY не должен ставиться")
		}
		_, _ = w.Write([]byte(`{"status":"SUCCESS","message":"success","data":{"jobId":"t"}}`))
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
