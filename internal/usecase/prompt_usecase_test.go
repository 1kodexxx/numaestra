package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

func TestNewNoopPromptUseCase(t *testing.T) {
	uc := NewNoopPromptUseCase()
	if uc == nil {
		t.Fatal("NewNoopPromptUseCase вернул nil")
	}
}

func TestPromptUseCase_InvalidateCache_Noop(t *testing.T) {
	uc := NewNoopPromptUseCase()
	uc.InvalidateCache()
}

func TestPromptUseCase_GetAllCategories_RepoError(t *testing.T) {
	repo := newInMemCategoryRepo()
	repo.getErr = errors.New("db down")
	uc := NewPromptUseCase(repo)
	_, err := uc.GetAllCategories(context.Background())
	if err == nil {
		t.Fatal("ожидали ошибку при недоступном репозитории")
	}
}

func TestPromptUseCase_GetAllCategories_CacheHit(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("c1", "Тест", "описание", "", nil, "base_prompt")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)
	ctx := context.Background()

	first, err := uc.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("первый вызов: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("ожидали 1 категорию, получили %d", len(first))
	}
	// Кеш прогрет — репозиторий "падает", но GetAll возвращает из кеша.
	repo.getErr = errors.New("db down")
	second, err := uc.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("второй вызов (из кеша): %v", err)
	}
	if len(second) != 1 {
		t.Errorf("ожидали 1 категорию из кеша, получили %d", len(second))
	}
}

func TestPromptUseCase_InvalidateCacheClears(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("c1", "Тест", "описание", "", nil, "base_prompt")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)
	ctx := context.Background()

	_, _ = uc.GetAllCategories(ctx) // прогрев кеша
	uc.InvalidateCache()
	repo.getErr = errors.New("db down")
	_, err := uc.GetAllCategories(ctx)
	if err == nil {
		t.Fatal("после инвалидации кеша должна вернуться ошибка репозитория")
	}
}

func TestPromptUseCase_GetCategoryWizard(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("wedding", "Свадьба", "для свадьбы", "", nil, "Create a wedding song: [BRIEF]")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)

	got, err := uc.GetCategoryWizard(context.Background(), "wedding")
	if err != nil {
		t.Fatalf("GetCategoryWizard: %v", err)
	}
	if got.ID() != "wedding" {
		t.Errorf("неверный ID: %q", got.ID())
	}

	_, err = uc.GetCategoryWizard(context.Background(), "missing")
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Errorf("ожидали ErrCategoryNotFound, получили %v", err)
	}
}

func TestPromptUseCase_BuildFinalPrompt(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("bday", "День рождения", "праздник", "", nil, "Create a birthday song: [NAME]")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)

	prompt, err := uc.BuildFinalPrompt(context.Background(), "bday", map[string]string{
		"NAME": "Иван",
	})
	if err != nil {
		t.Fatalf("BuildFinalPrompt: %v", err)
	}
	if prompt == "" {
		t.Error("BuildFinalPrompt вернул пустую строку")
	}
}

func TestPromptUseCase_BuildFinalPrompt_DropsUnfilledOptionalPlaceholder(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("anniv", "Годовщина", "праздник", "", nil,
		"Create a [MOOD] song. The lyrics must be in Russian language. Celebrating [YEARS] years together. How they met: [MEET_STORY].")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)

	// MEET_STORY — необязательный вопрос, клиент его пропустил (нет в answers).
	prompt, err := uc.BuildFinalPrompt(context.Background(), "anniv", map[string]string{
		"YEARS": "5",
		"MOOD":  "romantic",
		"VOCAL": "male vocals",
	})
	if err != nil {
		t.Fatalf("BuildFinalPrompt: %v", err)
	}
	if strings.Contains(prompt, "[MEET_STORY]") {
		t.Errorf("неподставленный плейсхолдер просочился в Suno-промпт:\n%s", prompt)
	}
	if !strings.Contains(prompt, "5") {
		t.Errorf("заполненные ответы должны остаться, получили:\n%s", prompt)
	}
}

func TestPromptUseCase_BuildFinalPrompt_NoDuplicateCustomLyrics(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("bday", "День рождения", "праздник", "", nil,
		"Song for [NAME]. Lyrics:\n[CUSTOM_LYRICS]")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)

	lyrics := "Строка один\nСтрока два"
	prompt, err := uc.BuildFinalPrompt(context.Background(), "bday", map[string]string{
		"NAME":          "Иван",
		"CUSTOM_LYRICS": lyrics,
		"GENRE":         "modern pop",
		"VOCAL":         "male vocals",
	})
	if err != nil {
		t.Fatalf("BuildFinalPrompt: %v", err)
	}
	if strings.Count(prompt, lyrics) != 1 {
		t.Errorf("CUSTOM_LYRICS не должен дублироваться, count=%d", strings.Count(prompt, lyrics))
	}
}

func TestPromptUseCase_BuildFinalPrompt_CustomGenreInDescription(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("bday", "День рождения", "праздник", "", nil, "Song: [NAME]")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)

	prompt, err := uc.BuildFinalPrompt(context.Background(), "bday", map[string]string{
		"NAME":  "Иван",
		"GENRE": "modern pop, Трэп",
		"VOCAL": "male vocals",
	})
	if err != nil {
		t.Fatalf("BuildFinalPrompt: %v", err)
	}
	if strings.Contains(prompt, "Трэп") {
		enc, ok := suno.DecodePrompt(prompt)
		if !ok || strings.Contains(enc.Tags, "Трэп") {
			t.Error("Трэп не должен быть в tags")
		}
	}
	if !strings.Contains(prompt, "Preferred genre style: Трэп") {
		t.Errorf("ожидали Трэп в description: %q", prompt)
	}
}

func TestPromptUseCase_BuildFinalPrompt_NotFound(t *testing.T) {
	uc := NewPromptUseCase(newInMemCategoryRepo())
	_, err := uc.BuildFinalPrompt(context.Background(), "missing", nil)
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Errorf("ожидали ErrCategoryNotFound, получили %v", err)
	}
}
