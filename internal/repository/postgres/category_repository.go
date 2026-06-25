package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/numaestra/numaestra/internal/domain"
)

// pgErrCode возвращает SQLSTATE-код ошибки Postgres, если err — *pgconn.PgError.
func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

const (
	pgErrUniqueViolation     = "23505"
	pgErrForeignKeyViolation = "23503"
)

type CategoryRepository struct {
	db     PgxPool
	genres domain.GenreRepository
}

func NewCategoryRepository(db PgxPool) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) WithGenres(genres domain.GenreRepository) *CategoryRepository {
	r.genres = genres
	return r
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
		var snap domain.CategorySnapshot
		if err := rows.Scan(&snap.ID, &snap.Title, &snap.Description, &snap.CoverImageURL, &snap.SeoTags); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, domain.RestoreCategory(snap))
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
						'option_source', q.option_source,
						'config',        q.config,
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
							  AND q.option_source = 'inline'
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

	var snap domain.CategorySnapshot
	var questionsJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&snap.ID,
		&snap.Title,
		&snap.Description,
		&snap.CoverImageURL,
		&snap.SeoTags,
		&snap.BasePromptTemplate,
		&questionsJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("category %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query category by id: %w", err)
	}

	if err := json.Unmarshal(questionsJSON, &snap.Questions); err != nil {
		return nil, fmt.Errorf("unmarshal questions: %w", err)
	}

	if err := r.enrichGenreOptions(ctx, id, snap.Questions); err != nil {
		return nil, err
	}

	return domain.RestoreCategory(snap), nil
}

func (r *CategoryRepository) enrichGenreOptions(ctx context.Context, categoryID string, questions []domain.Question) error {
	if r.genres == nil {
		return nil
	}
	genres, err := r.genres.GetForCategory(ctx, categoryID)
	if err != nil {
		return err
	}
	genreOptions := make([]domain.Option, 0, len(genres))
	for _, g := range genres {
		genreOptions = append(genreOptions, g.ToOption())
	}
	for i := range questions {
		if questions[i].OptionSource == domain.OptionSourceGenres {
			questions[i].Options = append([]domain.Option(nil), genreOptions...)
		}
	}
	return nil
}

// Create сохраняет новую категорию каталога.
func (r *CategoryRepository) Create(ctx context.Context, c *domain.Category) error {
	snap := c.Snapshot()
	_, err := r.db.Exec(ctx, `
		INSERT INTO categories (id, title, description, cover_image_url, seo_tags, base_prompt_template)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, snap.ID, snap.Title, snap.Description, snap.CoverImageURL, snap.SeoTags, snap.BasePromptTemplate)
	if err != nil {
		if pgErrCode(err) == pgErrUniqueViolation {
			return domain.ErrCategoryAlreadyExists
		}
		return fmt.Errorf("insert category: %w", err)
	}
	return nil
}

// Update перезаписывает изменяемые поля категории (ID неизменен).
func (r *CategoryRepository) Update(ctx context.Context, c *domain.Category) error {
	snap := c.Snapshot()
	cmd, err := r.db.Exec(ctx, `
		UPDATE categories
		SET title = $1, description = $2, cover_image_url = $3, seo_tags = $4, base_prompt_template = $5
		WHERE id = $6
	`, snap.Title, snap.Description, snap.CoverImageURL, snap.SeoTags, snap.BasePromptTemplate, snap.ID)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

// Delete удаляет категорию; вопросы и варианты ответов удаляются каскадно (ON DELETE CASCADE).
func (r *CategoryRepository) Delete(ctx context.Context, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

// AddQuestion добавляет новый вопрос (и его варианты ответов) к категории.
func (r *CategoryRepository) AddQuestion(ctx context.Context, categoryID string, q domain.Question) (domain.Question, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO questions (category_id, step_number, question_text, ui_type, mapping_key, is_required, option_source, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, categoryID, q.StepNumber, q.QuestionText, q.UIType, q.MappingKey, q.IsRequired, q.OptionSource, q.Config.ToMap()).Scan(&q.ID)
	if err != nil {
		if pgErrCode(err) == pgErrForeignKeyViolation {
			return domain.Question{}, domain.ErrCategoryNotFound
		}
		return domain.Question{}, fmt.Errorf("insert question: %w", err)
	}

	if q.OptionSource == "" {
		q.OptionSource = domain.OptionSourceInline
	}
	if err := r.insertOptions(ctx, q.ID, q.Options); err != nil {
		return domain.Question{}, err
	}
	return q, nil
}

// UpdateQuestion перезаписывает вопрос и полностью заменяет его варианты ответов.
func (r *CategoryRepository) UpdateQuestion(ctx context.Context, categoryID string, q domain.Question) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE questions
		SET step_number = $1, question_text = $2, ui_type = $3, mapping_key = $4, is_required = $5,
		    option_source = $6, config = $7
		WHERE id = $8 AND category_id = $9
	`, q.StepNumber, q.QuestionText, q.UIType, q.MappingKey, q.IsRequired, q.OptionSource, q.Config.ToMap(), q.ID, categoryID)
	if err != nil {
		return fmt.Errorf("update question: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrQuestionNotFound
	}

	if _, err := r.db.Exec(ctx, `DELETE FROM question_options WHERE question_id = $1`, q.ID); err != nil {
		return fmt.Errorf("clear question options: %w", err)
	}
	return r.insertOptions(ctx, q.ID, q.Options)
}

// DeleteQuestion удаляет вопрос; варианты ответов удаляются каскадно.
func (r *CategoryRepository) DeleteQuestion(ctx context.Context, categoryID string, questionID int) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM questions WHERE id = $1 AND category_id = $2`, questionID, categoryID)
	if err != nil {
		return fmt.Errorf("delete question: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrQuestionNotFound
	}
	return nil
}

func (r *CategoryRepository) insertOptions(ctx context.Context, questionID int, options []domain.Option) error {
	for _, o := range options {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO question_options (question_id, label, value) VALUES ($1, $2, $3)
		`, questionID, o.Label, o.Value); err != nil {
			return fmt.Errorf("insert question option: %w", err)
		}
	}
	return nil
}
