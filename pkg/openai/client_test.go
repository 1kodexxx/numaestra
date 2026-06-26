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

func TestSplitLyricVariants(t *testing.T) {
	sep := lyricVariantSeparator
	content := "Вариант 1\n" + sep + "\nВариант 2\n" + sep + "\nВариант 3"
	variants := splitLyricVariants(content)
	if len(variants) != 3 {
		t.Fatalf("ожидали 3 варианта, получили %d", len(variants))
	}
	if variants[0] != "Вариант 1" {
		t.Errorf("неверный первый вариант: %q", variants[0])
	}
}

func TestSplitLyricVariants_EmptyParts(t *testing.T) {
	sep := lyricVariantSeparator
	content := sep + "\n\nВариант A\n" + sep
	variants := splitLyricVariants(content)
	if len(variants) != 1 {
		t.Fatalf("ожидали 1 вариант, получили %d", len(variants))
	}
	if variants[0] != "Вариант A" {
		t.Errorf("неверный вариант: %q", variants[0])
	}
}

func TestContainsVariant(t *testing.T) {
	existing := []string{"Текст A", "Текст B"}
	if !containsVariant(existing, "Текст A") {
		t.Error("должен найти точное совпадение")
	}
	if !containsVariant(existing, "  Текст B  ") {
		t.Error("должен игнорировать пробелы по краям")
	}
	if containsVariant(existing, "Текст C") {
		t.Error("не должен находить отсутствующий вариант")
	}
	if containsVariant(nil, "что-то") {
		t.Error("не должен находить в пустом срезе")
	}
}

func TestGenerateLyricsVariants_Count1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"[Verse 1]\nСтрока"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	variants, err := c.GenerateLyricsVariants(context.Background(), "факты", 1)
	if err != nil {
		t.Fatalf("GenerateLyricsVariants(count=1): %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("ожидали 1 вариант, получили %d", len(variants))
	}
}

func TestGenerateLyricsVariants_CountZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"один"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	variants, err := c.GenerateLyricsVariants(context.Background(), "факты", 0)
	if err != nil {
		t.Fatalf("GenerateLyricsVariants(count=0): %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("ожидали 1 вариант для count=0, получили %d", len(variants))
	}
}

func TestGenerateLyricsVariants_MultipleFromSeparator(t *testing.T) {
	sep := lyricVariantSeparator
	body := `{"choices":[{"message":{"role":"assistant","content":"Вариант 1\n` + sep + `\nВариант 2"}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	variants, err := c.GenerateLyricsVariants(context.Background(), "факты", 2)
	if err != nil {
		t.Fatalf("GenerateLyricsVariants(count=2): %v", err)
	}
	if len(variants) != 2 {
		t.Fatalf("ожидали 2 варианта, получили %d: %v", len(variants), variants)
	}
}

func TestGenerateLyricsVariants_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("ошибка сервера"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	_, err := c.GenerateLyricsVariants(context.Background(), "факты", 2)
	if err == nil {
		t.Fatal("ожидали ошибку при 500 от сервера")
	}
}
