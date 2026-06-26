package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/numaestra/numaestra/internal/domain"
)

type PromoCodeRepository struct {
	pool PgxPool
}

func NewPromoCodeRepository(pool PgxPool) *PromoCodeRepository {
	return &PromoCodeRepository{pool: pool}
}

var _ domain.PromoCodeRepository = (*PromoCodeRepository)(nil)

const promoSelectColumns = `
	id, code, discount_type, discount_value,
	max_uses, current_uses,
	valid_from, valid_until,
	is_active, COALESCE(description, '') AS description,
	created_at`

func scanPromo(row pgx.Row) (domain.PromoCodeSnapshot, error) {
	var s domain.PromoCodeSnapshot
	err := row.Scan(
		&s.ID, &s.Code, &s.DiscountType, &s.DiscountValue,
		&s.MaxUses, &s.CurrentUses,
		&s.ValidFrom, &s.ValidUntil,
		&s.IsActive, &s.Description,
		&s.CreatedAt,
	)
	if err != nil {
		return domain.PromoCodeSnapshot{}, fmt.Errorf("scan promo_code: %w", err)
	}
	return s, nil
}

func (r *PromoCodeRepository) GetByCode(ctx context.Context, code string) (*domain.PromoCode, error) {
	query := `SELECT ` + promoSelectColumns + ` FROM promo_codes WHERE code = $1`
	s, err := scanPromo(r.pool.QueryRow(ctx, query, strings.ToUpper(code)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPromoCodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain.RestorePromoCode(s), nil
}

func (r *PromoCodeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PromoCode, error) {
	query := `SELECT ` + promoSelectColumns + ` FROM promo_codes WHERE id = $1`
	s, err := scanPromo(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPromoCodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain.RestorePromoCode(s), nil
}

func (r *PromoCodeRepository) Create(ctx context.Context, promo *domain.PromoCode) error {
	snap := promo.Snapshot()
	query := `
		INSERT INTO promo_codes (id, code, discount_type, discount_value, max_uses, current_uses, valid_from, valid_until, is_active, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.pool.Exec(ctx, query,
		snap.ID, strings.ToUpper(snap.Code), snap.DiscountType, snap.DiscountValue,
		snap.MaxUses, snap.CurrentUses,
		snap.ValidFrom, snap.ValidUntil,
		snap.IsActive, nullableString(snap.Description),
		snap.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert promo_code: %w", err)
	}
	return nil
}

func (r *PromoCodeRepository) Update(ctx context.Context, promo *domain.PromoCode) error {
	snap := promo.Snapshot()
	query := `
		UPDATE promo_codes
		SET discount_type = $1, discount_value = $2, max_uses = $3,
		    valid_from = $4, valid_until = $5, is_active = $6, description = $7
		WHERE id = $8
	`
	cmd, err := r.pool.Exec(ctx, query,
		snap.DiscountType, snap.DiscountValue, snap.MaxUses,
		snap.ValidFrom, snap.ValidUntil, snap.IsActive,
		nullableString(snap.Description), snap.ID,
	)
	if err != nil {
		return fmt.Errorf("update promo_code: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrPromoCodeNotFound
	}
	return nil
}

func (r *PromoCodeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM promo_codes WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrForeignKeyViolation {
			return domain.ErrPromoCodeInUse
		}
		return fmt.Errorf("delete promo_code: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrPromoCodeNotFound
	}
	return nil
}

func (r *PromoCodeRepository) List(ctx context.Context, limit, offset int) ([]*domain.PromoCode, error) {
	query := `SELECT ` + promoSelectColumns + ` FROM promo_codes ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list promo_codes: %w", err)
	}
	defer rows.Close()

	var result []*domain.PromoCode
	for rows.Next() {
		var s domain.PromoCodeSnapshot
		if err := rows.Scan(
			&s.ID, &s.Code, &s.DiscountType, &s.DiscountValue,
			&s.MaxUses, &s.CurrentUses,
			&s.ValidFrom, &s.ValidUntil,
			&s.IsActive, &s.Description,
			&s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan promo_code row: %w", err)
		}
		result = append(result, domain.RestorePromoCode(s))
	}
	return result, rows.Err()
}

func (r *PromoCodeRepository) Count(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM promo_codes`).Scan(&n)
	return n, err
}

// IncrementUses атомарно увеличивает счётчик, уважая max_uses.
func (r *PromoCodeRepository) IncrementUses(ctx context.Context, id uuid.UUID) (bool, error) {
	query := `
		UPDATE promo_codes
		SET current_uses = current_uses + 1
		WHERE id = $1
		  AND is_active = true
		  AND (max_uses IS NULL OR current_uses < max_uses)
		  AND (valid_until IS NULL OR valid_until > $2)
	`
	cmd, err := r.pool.Exec(ctx, query, id, time.Now().UTC())
	if err != nil {
		return false, fmt.Errorf("increment promo uses: %w", err)
	}
	return cmd.RowsAffected() > 0, nil
}
