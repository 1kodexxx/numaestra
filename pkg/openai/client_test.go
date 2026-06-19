package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateLyrics_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("неверный путь: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer key123" {
			t.Errorf("ожидали Bearer key123, получили %q", auth)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"[Verse 1]\nТекст песни"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key123")
	got, err := c.GenerateLyrics(context.Background(), "факты о юбилее")
	if err != nil {
		t.Fatalf("GenerateLyrics упал: %v", err)
	}
	if got == "" {
		t.Error("ожидали непустой текст")
	}
}

func TestGenerateLyrics_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad")
	if _, err := c.GenerateLyrics(context.Background(), "x"); err == nil {
		t.Fatal("ожидали ошибку при 401")
	}
}

func TestGenerateLyrics_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	if _, err := c.GenerateLyrics(context.Background(), "x"); err == nil {
		t.Fatal("ожидали ошибку при пустом списке choices")
	}
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	// Пустой baseURL не должен паниковать; клиент создаётся с дефолтом.
	c := NewClient("", "key")
	if c == nil {
		t.Fatal("клиент не должен быть nil")
	}
}
