package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/numaestra/numaestra/internal/domain"
)

type CategoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetAll забирает только базовую информацию для карточек на главной странице.
func (r *CategoryRepository) GetAll(ctx context.Context) ([]*domain.Category, error) {
	query := `SELECT id, title, description, cover_image_url, seo_tags FROM categories ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	var categories []*domain.Category
	for rows.Next() {
		var cat domain.Category
		if err := rows.Scan(&cat.ID, &cat.Title, &cat.Description, &cat.CoverImageURL, &cat.SeoTags); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, &cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}
	return categories, nil
}

// GetByID собирает категорию вместе со всеми вопросами и вариантами ответов
// одним запросом через json_agg, чтобы избежать N+1.
func (r *CategoryRepository) GetByID(ctx context.Context, id string) (*domain.Category, error) {
	query := `
		SELECT
			c.id,
			c.title,
			c.description,
			c.cover_image_url,
			c.seo_tags,
			c.base_prompt_template,
			COALESCE(
				json_agg(
					json_build_object(
						'id',            q.id,
						'step_number',   q.step_number,
						'question_text', q.question_text,
						'ui_type',       q.ui_type,
						'mapping_key',   q.mapping_key,
						'is_required',   q.is_required,
						'options', (
							SELECT COALESCE(
								json_agg(
									json_build_object(
										'label', o.label,
										'value', o.value
									) ORDER BY o.id
								),
								'[]'::json
							)
							FROM question_options o
							WHERE o.question_id = q.id
						)
					) ORDER BY q.step_number
				) FILTER (WHERE q.id IS NOT NULL),
				'[]'::json
			) AS questions
		FROM categories c
		LEFT JOIN questions q ON q.category_id = c.id
		WHERE c.id = $1
		GROUP BY c.id
	`

	var cat domain.Category
	var questionsJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&cat.ID,
		&cat.Title,
		&cat.Description,
		&cat.CoverImageURL,
		&cat.SeoTags,
		&cat.BasePromptTemplate,
		&questionsJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("category %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query category by id: %w", err)
	}

	if err := json.Unmarshal(questionsJSON, &cat.Questions); err != nil {
		return nil, fmt.Errorf("unmarshal questions: %w", err)
	}

	return &cat, nil
}
