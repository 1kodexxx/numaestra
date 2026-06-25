package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

type inMemGenreRepo struct {
	mu              sync.Mutex
	genres          map[int]domain.Genre
	nextID          int
	categoryGenres  map[string][]int
	createErr       error
}

func newInMemGenreRepo() *inMemGenreRepo {
	return &inMemGenreRepo{
		genres:         make(map[int]domain.Genre),
		categoryGenres: make(map[string][]int),
		nextID:         1,
	}
}

func (r *inMemGenreRepo) seed(g domain.Genre) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g.ID == 0 {
		g.ID = r.nextID
		r.nextID++
	}
	r.genres[g.ID] = g
}

func (r *inMemGenreRepo) GetAll(_ context.Context, activeOnly bool) ([]domain.Genre, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Genre
	for _, g := range r.genres {
		if activeOnly && !g.IsActive {
			continue
		}
		out = append(out, g)
	}
	return out, nil
}

func (r *inMemGenreRepo) GetByID(_ context.Context, id int) (*domain.Genre, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.genres[id]
	if !ok {
		return nil, domain.ErrGenreNotFound
	}
	cp := g
	return &cp, nil
}

func (r *inMemGenreRepo) GetForCategory(_ context.Context, categoryID string) ([]domain.Genre, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids, ok := r.categoryGenres[categoryID]
	if !ok || len(ids) == 0 {
		var all []domain.Genre
		for _, g := range r.genres {
			if g.IsActive {
				all = append(all, g)
			}
		}
		return all, nil
	}
	var out []domain.Genre
	for _, id := range ids {
		if g, ok := r.genres[id]; ok && g.IsActive {
			out = append(out, g)
		}
	}
	return out, nil
}

func (r *inMemGenreRepo) Create(_ context.Context, g *domain.Genre) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	for _, existing := range r.genres {
		if existing.Slug == g.Slug {
			return domain.ErrGenreAlreadyExists
		}
	}
	g.ID = r.nextID
	r.nextID++
	r.genres[g.ID] = *g
	return nil
}

func (r *inMemGenreRepo) Update(_ context.Context, g *domain.Genre) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.genres[g.ID]; !ok {
		return domain.ErrGenreNotFound
	}
	r.genres[g.ID] = *g
	return nil
}

func (r *inMemGenreRepo) Delete(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.genres[id]; !ok {
		return domain.ErrGenreNotFound
	}
	delete(r.genres, id)
	return nil
}

func (r *inMemGenreRepo) SetCategoryGenres(_ context.Context, categoryID string, genreIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.categoryGenres[categoryID] = append([]int(nil), genreIDs...)
	return nil
}

func (r *inMemGenreRepo) GetCategoryGenreIDs(_ context.Context, categoryID string) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.categoryGenres[categoryID]
	return append([]int(nil), ids...), nil
}

var _ domain.GenreRepository = (*inMemGenreRepo)(nil)

func TestGenreUseCase_CreateAndList(t *testing.T) {
	repo := newInMemGenreRepo()
	uc := NewGenreUseCase(repo, testLogger())

	g, err := uc.Create(context.Background(), "pop", "Поп", "modern pop", 10)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if g.ID == 0 || g.Slug != "pop" {
		t.Errorf("неверный жанр: %+v", g)
	}

	all, err := uc.List(context.Background(), "", false)
	if err != nil || len(all) != 1 {
		t.Fatalf("List: err=%v len=%d", err, len(all))
	}
}

func TestGenreUseCase_CreateDuplicateSlug(t *testing.T) {
	repo := newInMemGenreRepo()
	uc := NewGenreUseCase(repo, testLogger())
	_, _ = uc.Create(context.Background(), "pop", "Поп", "modern pop", 10)
	_, err := uc.Create(context.Background(), "pop", "Поп 2", "pop", 20)
	if !errors.Is(err, domain.ErrGenreAlreadyExists) {
		t.Fatalf("ожидали ErrGenreAlreadyExists, получили %v", err)
	}
}

func TestGenreUseCase_SetCategoryGenres(t *testing.T) {
	repo := newInMemGenreRepo()
	repo.seed(domain.Genre{Slug: "pop", Label: "Поп", SunoValue: "pop", IsActive: true})
	repo.seed(domain.Genre{ID: 2, Slug: "rock", Label: "Рок", SunoValue: "rock", IsActive: true})
	uc := NewGenreUseCase(repo, testLogger())

	if err := uc.SetCategoryGenres(context.Background(), "wedding", []int{1}); err != nil {
		t.Fatalf("SetCategoryGenres: %v", err)
	}
	genres, err := uc.List(context.Background(), "wedding", true)
	if err != nil || len(genres) != 1 || genres[0].Slug != "pop" {
		t.Fatalf("ожидали 1 жанр pop для wedding, получили %+v err=%v", genres, err)
	}
	ids, _ := uc.GetCategoryGenreIDs(context.Background(), "wedding")
	if len(ids) != 1 || ids[0] != 1 {
		t.Errorf("ожидали genre_ids [1], получили %v", ids)
	}
}

func TestGenreUseCase_UpdateAndDelete(t *testing.T) {
	repo := newInMemGenreRepo()
	uc := NewGenreUseCase(repo, testLogger())
	g, _ := uc.Create(context.Background(), "folk", "Фолк", "folk", 5)

	updated, err := uc.Update(context.Background(), g.ID, "Фолк-рок", "folk rock", 15, false)
	if err != nil || updated.Label != "Фолк-рок" || updated.IsActive {
		t.Fatalf("Update: %+v err=%v", updated, err)
	}
	if err := uc.Delete(context.Background(), g.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := uc.GetByID(context.Background(), g.ID); !errors.Is(err, domain.ErrGenreNotFound) {
		t.Fatalf("ожидали not found после delete, получили %v", err)
	}
}
