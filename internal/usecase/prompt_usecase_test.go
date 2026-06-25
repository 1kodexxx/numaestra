package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

// --- in-memory CategoryRepository ---

type inMemCategoryRepo struct {
	mu         sync.Mutex
	categories map[string]domain.CategorySnapshot
	getAllErr  error
	getByIDErr error
	calls      int // счётчик вызовов GetAll для проверки кеша
}

func newInMemCategoryRepo(snaps ...domain.CategorySnapshot) *inMemCategoryRepo {
	r := &inMemCategoryRepo{categories: make(map[string]domain.CategorySnapshot)}
	for _, s := range snaps {
		r.categories[s.ID] = s
	}
	return r
}

func (r *inMemCategoryRepo) GetAll(_ context.Context) ([]*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.getAllErr != nil {
		return nil, r.getAllErr
	}
	out := make([]*domain.Category, 0, len(r.categories))
	for _, s := range r.categories {
		out = append(out, domain.RestoreCategory(s))
	}
	return out, nil
}

func (r *inMemCategoryRepo) GetByID(_ context.Context, id string) (*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	s, ok := r.categories[id]
	if !ok {
		return nil, errors.New("категория не найдена")
	}
	return domain.RestoreCategory(s), nil
}

func (r *inMemCategoryRepo) Create(_ context.Context, c *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := c.Snapshot()
	if _, exists := r.categories[snap.ID]; exists {
		return domain.ErrCategoryAlreadyExists
	}
	r.categories[snap.ID] = snap
	return nil
}

func (r *inMemCategoryRepo) Update(_ context.Context, c *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := c.Snapshot()
	if _, exists := r.categories[snap.ID]; !exists {
		return domain.ErrCategoryNotFound
	}
	r.categories[snap.ID] = snap
	return nil
}

func (r *inMemCategoryRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.categories[id]; !exists {
		return domain.ErrCategoryNotFound
	}
	delete(r.categories, id)
	return nil
}

func (r *inMemCategoryRepo) AddQuestion(_ context.Context, categoryID string, q domain.Question) (domain.Question, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.categories[categoryID]
	if !ok {
		return domain.Question{}, domain.ErrCategoryNotFound
	}
	q.ID = len(snap.Questions) + 1
	snap.Questions = append(snap.Questions, q)
	r.categories[categoryID] = snap
	return q, nil
}

func (r *inMemCategoryRepo) UpdateQuestion(_ context.Context, categoryID string, q domain.Question) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.categories[categoryID]
	if !ok {
		return domain.ErrQuestionNotFound
	}
	for i, existing := range snap.Questions {
		if existing.ID == q.ID {
			snap.Questions[i] = q
			r.categories[categoryID] = snap
			return nil
		}
	}
	return domain.ErrQuestionNotFound
}

func (r *inMemCategoryRepo) DeleteQuestion(_ context.Context, categoryID string, questionID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.categories[categoryID]
	if !ok {
		return domain.ErrQuestionNotFound
	}
	for i, existing := range snap.Questions {
		if existing.ID == questionID {
			snap.Questions = append(snap.Questions[:i], snap.Questions[i+1:]...)
			r.categories[categoryID] = snap
			return nil
		}
	}
	return domain.ErrQuestionNotFound
}

var _ domain.CategoryRepository = (*inMemCategoryRepo)(nil)

// --- тесты ---

func TestPromptUseCase_GetAllCategories_LoadsFromRepo(t *testing.T) {
	repo := newInMemCategoryRepo(
		domain.CategorySnapshot{ID: "c1", Title: "Свадьба"},
		domain.CategorySnapshot{ID: "c2", Title: "День рождения"},
	)
	uc := NewPromptUseCase(repo)

	cats, err := uc.GetAllCategories(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("ожидали 2 категории, получили %d", len(cats))
	}
	if repo.calls != 1 {
		t.Errorf("ожидали 1 вызов к репозиторию, получили %d", repo.calls)
	}
}

func TestPromptUseCase_GetAllCategories_CacheHit(t *testing.T) {
	repo := newInMemCategoryRepo(domain.CategorySnapshot{ID: "c1", Title: "Свадьба"})
	uc := NewPromptUseCase(repo)

	// Первый вызов — промах кеша.
	_, _ = uc.GetAllCategories(context.Background())
	// Второй вызов — должен вернуть кеш, не обращаясь к репозиторию.
	_, _ = uc.GetAllCategories(context.Background())

	if repo.calls != 1 {
		t.Errorf("кеш должен был сработать: ожидали 1 вызов к репо, получили %d", repo.calls)
	}
}

func TestPromptUseCase_GetAllCategories_CacheExpires(t *testing.T) {
	repo := newInMemCategoryRepo(domain.CategorySnapshot{ID: "c1", Title: "Свадьба"})
	uc := NewPromptUseCase(repo)

	// Первый вызов заполняет кеш.
	_, _ = uc.GetAllCategories(context.Background())

	// Принудительно устаревляем кеш.
	uc.mu.Lock()
	uc.cachedAt = time.Now().Add(-2 * categoriesCacheTTL)
	uc.mu.Unlock()

	// Второй вызов должен снова обратиться к репо.
	_, _ = uc.GetAllCategories(context.Background())

	if repo.calls != 2 {
		t.Errorf("ожидали 2 вызова к репо (кеш устарел), получили %d", repo.calls)
	}
}

func TestPromptUseCase_GetAllCategories_RepoError(t *testing.T) {
	repo := newInMemCategoryRepo()
	repo.getAllErr = errors.New("ошибка БД")
	uc := NewPromptUseCase(repo)

	_, err := uc.GetAllCategories(context.Background())
	if err == nil {
		t.Fatal("ожидали ошибку, получили nil")
	}
}

func TestPromptUseCase_GetCategoryWizard_Found(t *testing.T) {
	repo := newInMemCategoryRepo(domain.CategorySnapshot{
		ID:    "wedding",
		Title: "Свадьба",
		Questions: []domain.Question{
			{ID: 1, MappingKey: "names", QuestionText: "Имена молодожёнов?"},
		},
	})
	uc := NewPromptUseCase(repo)

	cat, err := uc.GetCategoryWizard(context.Background(), "wedding")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cat.ID() != "wedding" {
		t.Errorf("ожидали id=wedding, получили %q", cat.ID())
	}
	if len(cat.Questions()) != 1 {
		t.Errorf("ожидали 1 вопрос, получили %d", len(cat.Questions()))
	}
}

func TestPromptUseCase_GetCategoryWizard_NotFound(t *testing.T) {
	repo := newInMemCategoryRepo()
	uc := NewPromptUseCase(repo)

	_, err := uc.GetCategoryWizard(context.Background(), "ghost")
	if err == nil {
		t.Fatal("ожидали ошибку для несуществующей категории")
	}
}

func TestPromptUseCase_BuildFinalPrompt_SubstitutesPlaceholders(t *testing.T) {
	repo := newInMemCategoryRepo(domain.CategorySnapshot{
		ID:                 "bday",
		Title:              "День рождения",
		BasePromptTemplate: "Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Birthday person is [name], age [age].",
	})
	uc := NewPromptUseCase(repo)

	prompt, err := uc.BuildFinalPrompt(context.Background(), "bday", map[string]string{
		"name":  "Коля",
		"age":   "40",
		"GENRE": "jazz lounge",
		"MOOD":  "warm, friendly",
		"VOCAL": "male vocals",
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !strings.Contains(prompt, suno.TagsMarker) {
		t.Errorf("ожидали кодированный промпт с tags, получили %q", prompt)
	}
	if !strings.Contains(prompt, "jazz lounge") {
		t.Errorf("tags не содержат жанр: %q", prompt)
	}
	if !strings.Contains(prompt, "Коля") {
		t.Errorf("описание не содержит имя: %q", prompt)
	}
}

func TestPromptUseCase_BuildFinalPrompt_UnknownCategory(t *testing.T) {
	repo := newInMemCategoryRepo()
	uc := NewPromptUseCase(repo)

	_, err := uc.BuildFinalPrompt(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("ожидали ошибку для несуществующей категории")
	}
}

func TestPromptUseCase_BuildFinalPrompt_MissingAnswerLeavesPlaceholder(t *testing.T) {
	repo := newInMemCategoryRepo(domain.CategorySnapshot{
		ID:                 "tpl",
		Title:              "Тест",
		BasePromptTemplate: "The song is about [name], age [age].",
	})
	uc := NewPromptUseCase(repo)

	prompt, err := uc.BuildFinalPrompt(context.Background(), "tpl", map[string]string{"name": "Вася"})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !strings.Contains(prompt, "Вася") || !strings.Contains(prompt, "[age]") {
		t.Errorf("незаменённый плейсхолдер должен остаться, получили %q", prompt)
	}
}
