package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/numaestra/numaestra/internal/domain"
)

type CategoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetAll забирает только базовую информацию для карточек на главной странице
func (r *CategoryRepository) GetAll(ctx context.Context) ([]*domain.Category, error) {
	query := `SELECT id, title, description, cover_image_url, seo_tags FROM categories`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []*domain.Category
	for rows.Next() {
		var cat domain.Category
		if err := rows.Scan(&cat.ID, &cat.Title, &cat.Description, &cat.CoverImageURL, &cat.SeoTags); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, &cat)
	}

	return categories, nil
}

// GetByID собирает всю категорию вместе с вопросами (мастером)
func (r *CategoryRepository) GetByID(ctx context.Context, id string) (*domain.Category, error) {
	// Здесь будет чуть более сложный запрос.
	// Самый эффективный способ в PostgreSQL — собрать вопросы и опции через json_agg,
	// чтобы не делать N+1 запросов к базе.

	// Пока сделаем заглушку, чтобы код компилировался, а SQL напишем следующим этапом
	return &domain.Category{ID: id}, nil
}
