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
	// 1. Загружаем все заказы клиента одним запросом.
	orderQuery := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE customer_email = $1 ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, orderQuery, email)
	if err != nil {
		return nil, fmt.Errorf("list orders by email: %w", err)
	}
	defer rows.Close()

	var snaps []domain.OrderSnapshot
	var orderIDs []uuid.UUID
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
		snaps = append(snaps, snap)
		orderIDs = append(orderIDs, snap.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order rows: %w", err)
	}
	if len(snaps) == 0 {
		return nil, nil
	}

	// 2. Загружаем треки для всех заказов одним батч-запросом (вместо N отдельных).
	tracksByOrder, err := r.getTracksForOrders(ctx, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("batch load tracks: %w", err)
	}

	// 3. Раскладываем треки по снапшотам и восстанавливаем агрегаты.
	orders := make([]*domain.Order, 0, len(snaps))
	for _, snap := range snaps {
		snap.Tracks = tracksByOrder[snap.ID]
		orders = append(orders, domain.RestoreOrder(snap))
	}
	return orders, nil
}

// getTracksForOrders загружает треки для набора заказов одним запросом.
// Возвращает map[orderID][]Track для O(1) доступа при сборке агрегатов.
func (r *OrderRepository) getTracksForOrders(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]domain.Track, error) {
	query := `
		SELECT id, order_id, index, audio_url, duration_sec, suno_track_id
		FROM tracks
		WHERE order_id = ANY($1)
		ORDER BY order_id, index ASC
	`
	rows, err := r.pool.Query(ctx, query, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("batch select tracks: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]domain.Track)
	for rows.Next() {
		var t domain.Track
		var orderID uuid.UUID
		if err := rows.Scan(&t.ID, &orderID, &t.Index, &t.AudioURL, &t.DurationSec, &t.SunoTrackID); err != nil {
			return nil, fmt.Errorf("scan track row: %w", err)
		}
		result[orderID] = append(result[orderID], t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate track rows: %w", err)
	}
	return result, nil
}

// SaveWithAccount атомарно сохраняет изменения заказа и аккаунта в одной транзакции.
// Решает проблему "Busy-leak": если после SubmitGeneration упадёт сохранение заказа,
// аккаунт откатится вместе с ним и не застрянет в статусе Busy навсегда.
func (r *OrderRepository) SaveWithAccount(ctx context.Context, order *domain.Order, account *domain.SunoAccount) error {
	orderSnap := order.Snapshot()
	accSnap := account.Snapshot()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	orderQuery := `
		UPDATE orders
		SET payment_status = $1, generation_status = $2, assigned_account_id = $3, failure_reason = $4, updated_at = $5, paid_at = $6, completed_at = $7
		WHERE id = $8
	`
	cmd, err := tx.Exec(ctx, orderQuery,
		orderSnap.PaymentStatus, orderSnap.GenerationStatus, orderSnap.AssignedAccountID,
		orderSnap.FailureReason, orderSnap.UpdatedAt, orderSnap.PaidAt, orderSnap.CompletedAt,
		orderSnap.ID,
	)
	if err != nil {
		return fmt.Errorf("update order in SaveWithAccount: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}

	if err := r.saveTracks(ctx, tx, orderSnap.ID, orderSnap.Tracks); err != nil {
		return fmt.Errorf("save tracks in SaveWithAccount: %w", err)
	}

	accQuery := `
		UPDATE suno_accounts
		SET status = $1, token_balance = $2, failure_count = $3, cooldown_until = $4, last_used_at = $5, updated_at = $6
		WHERE id = $7
	`
	cmd, err = tx.Exec(ctx, accQuery,
		accSnap.Status, accSnap.TokenBalance, accSnap.FailureCount,
		accSnap.CooldownUntil, accSnap.LastUsedAt, accSnap.UpdatedAt,
		accSnap.ID,
	)
	if err != nil {
		return fmt.Errorf("update account in SaveWithAccount: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrAccountNotFound
	}

	return tx.Commit(ctx)
}

// NextInvoiceID возвращает следующий уникальный InvId из PostgreSQL sequence.
func (r *OrderRepository) NextInvoiceID(ctx context.Context) (int64, error) {
	var id int64
	if err := r.pool.QueryRow(ctx, "SELECT nextval('invoice_id_seq')").Scan(&id); err != nil {
		return 0, fmt.Errorf("nextval invoice_id_seq: %w", err)
	}
	return id, nil
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
