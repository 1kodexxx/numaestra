package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

type GenreRepository struct {
	db PgxPool
}

func NewGenreRepository(db PgxPool) *GenreRepository {
	return &GenreRepository{db: db}
}

func (r *GenreRepository) GetAll(ctx context.Context, activeOnly bool) ([]domain.Genre, error) {
	query := `SELECT id, slug, label, suno_value, sort_order, is_active FROM genres`
	if activeOnly {
		query += ` WHERE is_active = true`
	}
	query += ` ORDER BY sort_order, id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query genres: %w", err)
	}
	defer rows.Close()

	var out []domain.Genre
	for rows.Next() {
		var g domain.Genre
		if err := rows.Scan(&g.ID, &g.Slug, &g.Label, &g.SunoValue, &g.SortOrder, &g.IsActive); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *GenreRepository) GetByID(ctx context.Context, id int) (*domain.Genre, error) {
	var g domain.Genre
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, label, suno_value, sort_order, is_active FROM genres WHERE id = $1
	`, id).Scan(&g.ID, &g.Slug, &g.Label, &g.SunoValue, &g.SortOrder, &g.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query genre: %w", err)
	}
	return &g, nil
}

func (r *GenreRepository) GetForCategory(ctx context.Context, categoryID string) ([]domain.Genre, error) {
	rows, err := r.db.Query(ctx, `
		SELECT g.id, g.slug, g.label, g.suno_value,
		       COALESCE(cg.sort_order, g.sort_order), g.is_active
		FROM genres g
		LEFT JOIN category_genres cg ON cg.genre_id = g.id AND cg.category_id = $1
		WHERE g.is_active = true
		  AND (
		    EXISTS (SELECT 1 FROM category_genres x WHERE x.category_id = $1)
		      AND cg.genre_id IS NOT NULL
		    OR NOT EXISTS (SELECT 1 FROM category_genres x WHERE x.category_id = $1)
		  )
		ORDER BY COALESCE(cg.sort_order, g.sort_order), g.id
	`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("query category genres: %w", err)
	}
	defer rows.Close()

	var out []domain.Genre
	for rows.Next() {
		var g domain.Genre
		if err := rows.Scan(&g.ID, &g.Slug, &g.Label, &g.SunoValue, &g.SortOrder, &g.IsActive); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *GenreRepository) Create(ctx context.Context, g *domain.Genre) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO genres (slug, label, suno_value, sort_order, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, g.Slug, g.Label, g.SunoValue, g.SortOrder, g.IsActive).Scan(&g.ID)
	if err != nil {
		if pgErrCode(err) == pgErrUniqueViolation {
			return domain.ErrGenreAlreadyExists
		}
		return fmt.Errorf("insert genre: %w", err)
	}
	return nil
}

func (r *GenreRepository) Update(ctx context.Context, g *domain.Genre) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE genres SET label = $1, suno_value = $2, sort_order = $3, is_active = $4
		WHERE id = $5
	`, g.Label, g.SunoValue, g.SortOrder, g.IsActive, g.ID)
	if err != nil {
		return fmt.Errorf("update genre: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrGenreNotFound
	}
	return nil
}

func (r *GenreRepository) Delete(ctx context.Context, id int) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM genres WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete genre: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrGenreNotFound
	}
	return nil
}

func (r *GenreRepository) SetCategoryGenres(ctx context.Context, categoryID string, genreIDs []int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM category_genres WHERE category_id = $1`, categoryID); err != nil {
		return fmt.Errorf("clear category genres: %w", err)
	}
	for i, gid := range genreIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO category_genres (category_id, genre_id, sort_order) VALUES ($1, $2, $3)
		`, categoryID, gid, i*10); err != nil {
			return fmt.Errorf("insert category genre: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *GenreRepository) GetCategoryGenreIDs(ctx context.Context, categoryID string) ([]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT genre_id FROM category_genres WHERE category_id = $1 ORDER BY sort_order, genre_id
	`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("query category genre ids: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan genre id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
