package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

type ExampleRepository struct {
	db PgxPool
}

func NewExampleRepository(db PgxPool) *ExampleRepository {
	return &ExampleRepository{db: db}
}

var _ domain.ExampleRepository = (*ExampleRepository)(nil)

const exampleColumns = `id, title, category, description, mood, audio_url, cover_url, sort_order, is_active`

func scanExample(row pgx.Row) (*domain.Example, error) {
	var s domain.ExampleSnapshot
	if err := row.Scan(&s.ID, &s.Title, &s.Category, &s.Description, &s.Mood, &s.AudioURL, &s.CoverURL, &s.SortOrder, &s.IsActive); err != nil {
		return nil, err
	}
	return domain.RestoreExample(s), nil
}

func (r *ExampleRepository) list(ctx context.Context, where string) ([]*domain.Example, error) {
	query := "SELECT " + exampleColumns + " FROM examples " + where + " ORDER BY sort_order ASC, id ASC"
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query examples: %w", err)
	}
	defer rows.Close()

	var examples []*domain.Example
	for rows.Next() {
		e, err := scanExample(rows)
		if err != nil {
			return nil, fmt.Errorf("scan example: %w", err)
		}
		examples = append(examples, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate examples: %w", err)
	}
	return examples, nil
}

func (r *ExampleRepository) GetAll(ctx context.Context) ([]*domain.Example, error) {
	return r.list(ctx, "")
}

func (r *ExampleRepository) GetActive(ctx context.Context) ([]*domain.Example, error) {
	return r.list(ctx, "WHERE is_active = TRUE")
}

func (r *ExampleRepository) GetByID(ctx context.Context, id string) (*domain.Example, error) {
	query := "SELECT " + exampleColumns + " FROM examples WHERE id = $1"
	e, err := scanExample(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrExampleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get example by id: %w", err)
	}
	return e, nil
}

func (r *ExampleRepository) Create(ctx context.Context, e *domain.Example) error {
	s := e.Snapshot()
	_, err := r.db.Exec(ctx, `
		INSERT INTO examples (id, title, category, description, mood, audio_url, cover_url, sort_order, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, s.ID, s.Title, s.Category, s.Description, s.Mood, s.AudioURL, s.CoverURL, s.SortOrder, s.IsActive)
	if err != nil {
		if pgErrCode(err) == pgErrUniqueViolation {
			return domain.ErrExampleAlreadyExists
		}
		return fmt.Errorf("insert example: %w", err)
	}
	return nil
}

func (r *ExampleRepository) Update(ctx context.Context, e *domain.Example) error {
	s := e.Snapshot()
	cmd, err := r.db.Exec(ctx, `
		UPDATE examples
		SET title = $1, category = $2, description = $3, mood = $4, audio_url = $5, cover_url = $6, sort_order = $7, is_active = $8, updated_at = NOW()
		WHERE id = $9
	`, s.Title, s.Category, s.Description, s.Mood, s.AudioURL, s.CoverURL, s.SortOrder, s.IsActive, s.ID)
	if err != nil {
		return fmt.Errorf("update example: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrExampleNotFound
	}
	return nil
}

func (r *ExampleRepository) Delete(ctx context.Context, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM examples WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete example: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrExampleNotFound
	}
	return nil
}
