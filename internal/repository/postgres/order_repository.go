package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/numaestra/numaestra/internal/domain"
)

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

var _ domain.OrderRepository = (*OrderRepository)(nil)

func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	snap := order.Snapshot()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queryOrder := `
		INSERT INTO orders (id, invoice_id, customer_email, customer_phone, brief, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, created_at, updated_at, paid_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err = tx.Exec(ctx, queryOrder,
		snap.ID, snap.InvoiceID, snap.CustomerEmail, snap.CustomerPhone, snap.Brief,
		snap.AmountKopecks, snap.Currency, snap.PaymentStatus, snap.GenerationStatus,
		snap.AssignedAccountID, snap.FailureReason, snap.CreatedAt, snap.UpdatedAt,
		snap.PaidAt, snap.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	if err := r.saveTracks(ctx, tx, snap.ID, snap.Tracks); err != nil {
		return fmt.Errorf("save tracks: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	query := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE id = $1
	`
	var snap domain.OrderSnapshot
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
		&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
		&snap.AssignedAccountID, &snap.FailureReason, &snap.CreatedAt, &snap.UpdatedAt,
		&snap.PaidAt, &snap.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select order by id: %w", err)
	}

	tracks, err := r.getTracksForOrder(ctx, snap.ID)
	if err != nil {
		return nil, fmt.Errorf("get tracks for order: %w", err)
	}
	snap.Tracks = tracks

	return domain.RestoreOrder(snap), nil
}

func (r *OrderRepository) GetByInvoiceID(ctx context.Context, invoiceID int64) (*domain.Order, error) {
	query := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE invoice_id = $1
	`
	var snap domain.OrderSnapshot
	err := r.pool.QueryRow(ctx, query, invoiceID).Scan(
		&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
		&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
		&snap.AssignedAccountID, &snap.FailureReason, &snap.CreatedAt, &snap.UpdatedAt,
		&snap.PaidAt, &snap.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select order by invoice id: %w", err)
	}

	tracks, err := r.getTracksForOrder(ctx, snap.ID)
	if err != nil {
		return nil, fmt.Errorf("get tracks for order: %w", err)
	}
	snap.Tracks = tracks

	return domain.RestoreOrder(snap), nil
}

func (r *OrderRepository) Update(ctx context.Context, order *domain.Order) error {
	snap := order.Snapshot()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE orders
		SET payment_status = $1, generation_status = $2, assigned_account_id = $3, failure_reason = $4, updated_at = $5, paid_at = $6, completed_at = $7
		WHERE id = $8
	`
	cmd, err := tx.Exec(ctx, query, snap.PaymentStatus, snap.GenerationStatus, snap.AssignedAccountID, snap.FailureReason, snap.UpdatedAt, snap.PaidAt, snap.CompletedAt, snap.ID)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}

	if err := r.saveTracks(ctx, tx, snap.ID, snap.Tracks); err != nil {
		return fmt.Errorf("update tracks: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *OrderRepository) ListByCustomerEmail(ctx context.Context, email string) ([]*domain.Order, error) {
	query := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE customer_email = $1 ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("list orders by email: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		var snap domain.OrderSnapshot
		err := rows.Scan(
			&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
			&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
			&snap.AssignedAccountID, &snap.FailureReason, &snap.CreatedAt, &snap.UpdatedAt,
			&snap.PaidAt, &snap.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}

		tracks, err := r.getTracksForOrder(ctx, snap.ID)
		if err != nil {
			return nil, fmt.Errorf("get tracks for ordered item: %w", err)
		}
		snap.Tracks = tracks

		orders = append(orders, domain.RestoreOrder(snap))
	}
	return orders, nil
}

func (r *OrderRepository) saveTracks(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, tracks []domain.Track) error {
	if len(tracks) == 0 {
		return nil
	}
	query := `
		INSERT INTO tracks (id, order_id, index, audio_url, duration_sec, suno_track_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (order_id, index) DO UPDATE
		SET audio_url = EXCLUDED.audio_url, duration_sec = EXCLUDED.duration_sec, suno_track_id = EXCLUDED.suno_track_id
	`
	for _, t := range tracks {
		_, err := tx.Exec(ctx, query, t.ID, orderID, t.Index, t.AudioURL, t.DurationSec, t.SunoTrackID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *OrderRepository) getTracksForOrder(ctx context.Context, orderID uuid.UUID) ([]domain.Track, error) {
	query := `SELECT id, index, audio_url, duration_sec, suno_track_id FROM tracks WHERE order_id = $1 ORDER BY index ASC`
	rows, err := r.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []domain.Track
	for rows.Next() {
		var t domain.Track
		err := rows.Scan(&t.ID, &t.Index, &t.AudioURL, &t.DurationSec, &t.SunoTrackID)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}
