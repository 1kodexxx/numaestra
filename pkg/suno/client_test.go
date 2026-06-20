package suno

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClient_Generate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/custom_generate" {
			t.Errorf("неверный путь: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("ожидали POST, получили %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Errorf("ожидали Bearer secret, получили %q", auth)
		}
		_ = json.NewEncoder(w).Encode([]Clip{
			{ID: "a", Status: StatusSubmitted},
			{ID: "b", Status: StatusSubmitted},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	clips, err := c.Generate(context.Background(), GenerateRequest{Prompt: "test"})
	if err != nil {
		t.Fatalf("Generate упал: %v", err)
	}
	if len(clips) != 2 {
		t.Fatalf("ожидали 2 клипа, получили %d", len(clips))
	}
}

func TestHTTPClient_Generate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	if _, err := c.Generate(context.Background(), GenerateRequest{Prompt: "x"}); err == nil {
		t.Fatal("ожидали ошибку при 500 от API")
	}
}

func TestHTTPClient_GetFeed_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/get") {
			t.Errorf("неверный путь: %s", r.URL.Path)
		}
		if ids := r.URL.Query().Get("ids"); ids != "a,b" {
			t.Errorf("ожидали ids=a,b, получили %q", ids)
		}
		_ = json.NewEncoder(w).Encode([]Clip{
			{ID: "a", Status: StatusComplete, AudioURL: "http://x/a.mp3", Duration: 180},
			{ID: "b", Status: StatusComplete, AudioURL: "http://x/b.mp3", Duration: 175},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	clips, err := c.GetFeed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("GetFeed упал: %v", err)
	}
	if len(clips) != 2 || clips[0].AudioURL == "" {
		t.Fatalf("неожиданный ответ GetFeed: %+v", clips)
	}
}

func TestHTTPClient_GetFeed_EmptyIDs(t *testing.T) {
	c := NewClient("http://unused", "secret")
	clips, err := c.GetFeed(context.Background(), nil)
	if err != nil {
		t.Fatalf("пустой список ids не должен вызывать ошибку: %v", err)
	}
	if clips != nil {
		t.Errorf("ожидали nil для пустого списка, получили %+v", clips)
	}
}

func TestHTTPClient_GetFeed_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	if _, err := c.GetFeed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("ожидали ошибку при 500 от GetFeed API")
	}
}

func TestHTTPClient_Generate_NoAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("при пустом apiKey не должен устанавливаться заголовок Authorization")
		}
		_ = json.NewEncoder(w).Encode([]Clip{{ID: "x"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "") // пустой ключ
	clips, err := c.Generate(context.Background(), GenerateRequest{Prompt: "test"})
	if err != nil {
		t.Fatalf("Generate упал: %v", err)
	}
	if len(clips) != 1 {
		t.Errorf("ожидали 1 клип, получили %d", len(clips))
	}
}

func TestHTTPClient_GetFeed_NoAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("при пустом apiKey заголовок Authorization не должен устанавливаться")
		}
		_ = json.NewEncoder(w).Encode([]Clip{{ID: "y", Status: StatusComplete}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	clips, err := c.GetFeed(context.Background(), []string{"y"})
	if err != nil {
		t.Fatalf("GetFeed упал: %v", err)
	}
	if len(clips) != 1 {
		t.Errorf("ожидали 1 клип, получили %d", len(clips))
	}
}

func TestHTTPClient_Generate_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // немедленно закрываем — следующий Do вернёт сетевую ошибку

	c := NewClient(srv.URL, "key")
	if _, err := c.Generate(context.Background(), GenerateRequest{Prompt: "x"}); err == nil {
		t.Fatal("ожидали ошибку при закрытом сервере")
	}
}

func TestHTTPClient_Generate_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	if _, err := c.Generate(context.Background(), GenerateRequest{Prompt: "x"}); err == nil {
		t.Fatal("ожидали ошибку при невалидном JSON-ответе")
	}
}

func TestHTTPClient_GetFeed_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	c := NewClient(srv.URL, "key")
	if _, err := c.GetFeed(context.Background(), []string{"a"}); err == nil {
		t.Fatal("ожидали ошибку при закрытом сервере")
	}
}

func TestHTTPClient_GetFeed_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{broken"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	if _, err := c.GetFeed(context.Background(), []string{"a"}); err == nil {
		t.Fatal("ожидали ошибку при невалидном JSON")
	}
}

func TestMockClient_CustomGetFeedFunc(t *testing.T) {
	m := &MockClient{
		GetFeedFunc: func(_ context.Context, ids []string) ([]Clip, error) {
			return []Clip{{ID: "custom-" + ids[0]}}, nil
		},
	}
	clips, err := m.GetFeed(context.Background(), []string{"z"})
	if err != nil {
		t.Fatalf("GetFeed упал: %v", err)
	}
	if len(clips) != 1 || clips[0].ID != "custom-z" {
		t.Errorf("должна вызываться кастомная GetFeedFunc, получили %+v", clips)
	}
}

// --- MockClient ---

func TestMockClient_Defaults(t *testing.T) {
	m := &MockClient{}
	clips, err := m.Generate(context.Background(), GenerateRequest{})
	if err != nil || len(clips) != 2 {
		t.Fatalf("дефолтный Generate должен вернуть 2 клипа без ошибки: %v, %+v", err, clips)
	}

	feed, err := m.GetFeed(context.Background(), []string{"x", "y"})
	if err != nil {
		t.Fatalf("дефолтный GetFeed упал: %v", err)
	}
	if len(feed) != 2 || feed[0].Status != StatusComplete {
		t.Errorf("дефолтный GetFeed должен вернуть готовые клипы, получили %+v", feed)
	}
}

func TestMockClient_CustomFuncs(t *testing.T) {
	m := &MockClient{
		GenerateFunc: func(_ context.Context, _ GenerateRequest) ([]Clip, error) {
			return []Clip{{ID: "custom"}}, nil
		},
	}
	clips, _ := m.Generate(context.Background(), GenerateRequest{})
	if len(clips) != 1 || clips[0].ID != "custom" {
		t.Errorf("должна вызываться кастомная GenerateFunc, получили %+v", clips)
	}
}
