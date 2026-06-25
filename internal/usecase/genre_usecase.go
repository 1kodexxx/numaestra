package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/numaestra/numaestra/internal/domain"
)

// GenreUseCase — справочник жанров и привязка жанров к категориям квиза.
type GenreUseCase struct {
	genreRepo domain.GenreRepository
	log       *slog.Logger
}

func NewGenreUseCase(genreRepo domain.GenreRepository, log *slog.Logger) *GenreUseCase {
	return &GenreUseCase{genreRepo: genreRepo, log: log}
}

func (uc *GenreUseCase) List(ctx context.Context, categoryID string, activeOnly bool) ([]domain.Genre, error) {
	if categoryID != "" {
		return uc.genreRepo.GetForCategory(ctx, categoryID)
	}
	return uc.genreRepo.GetAll(ctx, activeOnly)
}

func (uc *GenreUseCase) GetByID(ctx context.Context, id int) (*domain.Genre, error) {
	g, err := uc.genreRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (uc *GenreUseCase) Create(ctx context.Context, slug, label, sunoValue string, sortOrder int) (*domain.Genre, error) {
	g, err := domain.NewGenre(slug, label, sunoValue, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("валидация жанра: %w", err)
	}
	if err := uc.genreRepo.Create(ctx, g); err != nil {
		return nil, err
	}
	uc.log.Info("admin: создан жанр", "genre_id", g.ID, "slug", g.Slug)
	return g, nil
}

func (uc *GenreUseCase) Update(ctx context.Context, id int, label, sunoValue string, sortOrder int, isActive bool) (*domain.Genre, error) {
	g, err := uc.genreRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := g.Update(label, sunoValue, sortOrder, isActive); err != nil {
		return nil, fmt.Errorf("валидация жанра: %w", err)
	}
	if err := uc.genreRepo.Update(ctx, g); err != nil {
		return nil, err
	}
	uc.log.Info("admin: обновлён жанр", "genre_id", id)
	return g, nil
}

func (uc *GenreUseCase) Delete(ctx context.Context, id int) error {
	if err := uc.genreRepo.Delete(ctx, id); err != nil {
		return err
	}
	uc.log.Info("admin: удалён жанр", "genre_id", id)
	return nil
}

func (uc *GenreUseCase) GetCategoryGenreIDs(ctx context.Context, categoryID string) ([]int, error) {
	return uc.genreRepo.GetCategoryGenreIDs(ctx, categoryID)
}

func (uc *GenreUseCase) SetCategoryGenres(ctx context.Context, categoryID string, genreIDs []int) error {
	if err := uc.genreRepo.SetCategoryGenres(ctx, categoryID, genreIDs); err != nil {
		return err
	}
	uc.log.Info("admin: обновлены жанры категории", "category_id", categoryID, "count", len(genreIDs))
	return nil
}
