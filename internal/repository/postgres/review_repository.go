package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

type ReviewRepository struct {
	db PgxPool
}

func NewReviewRepository(db PgxPool) *ReviewRepository {
	return &ReviewRepository{db: db}
}

var _ domain.ReviewRepository = (*ReviewRepository)(nil)

const reviewColumns = `id, author_name, rating, body, admin_reply, admin_reply_at, is_published, created_at, updated_at`

func scanReview(row pgx.Row) (*domain.Review, error) {
	var s domain.ReviewSnapshot
	if err := row.Scan(&s.ID, &s.AuthorName, &s.Rating, &s.Body, &s.AdminReply, &s.AdminReplyAt, &s.IsPublished, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return domain.RestoreReview(s), nil
}

func (r *ReviewRepository) listReviews(ctx context.Context, where string, limit, offset int) ([]*domain.Review, error) {
	query := "SELECT " + reviewColumns + " FROM reviews " + where + " ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query reviews: %w", err)
	}
	defer rows.Close()

	var reviews []*domain.Review
	for rows.Next() {
		rev, err := scanReview(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviews: %w", err)
	}
	return reviews, nil
}

func (r *ReviewRepository) Create(ctx context.Context, rev *domain.Review) error {
	s := rev.Snapshot()
	_, err := r.db.Exec(ctx, `
		INSERT INTO reviews (id, author_name, rating, body, admin_reply, admin_reply_at, is_published, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, s.ID, s.AuthorName, s.Rating, s.Body, s.AdminReply, s.AdminReplyAt, s.IsPublished, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	rev, err := scanReview(r.db.QueryRow(ctx, "SELECT "+reviewColumns+" FROM reviews WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrReviewNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get review by id: %w", err)
	}
	return rev, nil
}

func (r *ReviewRepository) ListPublished(ctx context.Context, limit, offset int) ([]*domain.Review, error) {
	return r.listReviews(ctx, "WHERE is_published = TRUE", limit, offset)
}

func (r *ReviewRepository) CountPublished(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM reviews WHERE is_published = TRUE").Scan(&count); err != nil {
		return 0, fmt.Errorf("count published reviews: %w", err)
	}
	return count, nil
}

func (r *ReviewRepository) RatingStats(ctx context.Context) (int, float64, error) {
	var count int
	var avg float64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM reviews WHERE is_published = TRUE").Scan(&count, &avg)
	if err != nil {
		return 0, 0, fmt.Errorf("rating stats: %w", err)
	}
	return count, avg, nil
}

func (r *ReviewRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.Review, error) {
	return r.listReviews(ctx, "", limit, offset)
}

func (r *ReviewRepository) Update(ctx context.Context, rev *domain.Review) error {
	s := rev.Snapshot()
	cmd, err := r.db.Exec(ctx, `
		UPDATE reviews
		SET admin_reply = $1, admin_reply_at = $2, is_published = $3, updated_at = $4
		WHERE id = $5
	`, s.AdminReply, s.AdminReplyAt, s.IsPublished, s.UpdatedAt, s.ID)
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrReviewNotFound
	}
	return nil
}

func (r *ReviewRepository) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM reviews WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrReviewNotFound
	}
	return nil
}
