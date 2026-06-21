package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// conn возвращает исполнитель запросов: транзакцию из контекста (если репозиторий
// вызван внутри Unit of Work) либо пул соединений. Так одни и те же методы
// работают и автономно, и внутри общей транзакции, открытой TxManager.
func (r *OrderRepository) conn(ctx context.Context) dbConn {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	snap := order.Snapshot()

	return runAtomic(ctx, r.pool, func(ctx context.Context, db dbConn) error {
		queryOrder := `
			INSERT INTO orders (id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, access_token, created_at, updated_at, paid_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		`
		_, err := db.Exec(ctx, queryOrder,
			snap.ID, snap.InvoiceID, snap.CustomerEmail, snap.CustomerPhone, snap.Brief,
			nullableString(snap.CategoryID), nullableString(snap.SunoPrompt),
			snap.AmountKopecks, snap.Currency, snap.PaymentStatus, snap.GenerationStatus,
			snap.AssignedAccountID, snap.FailureReason, snap.AccessToken, snap.CreatedAt, snap.UpdatedAt,
			snap.PaidAt, snap.CompletedAt,
		)
		if err != nil {
			return fmt.Errorf("insert order: %w", err)
		}

		if err := r.saveTracks(ctx, db, snap.ID, snap.Tracks); err != nil {
			return fmt.Errorf("save tracks: %w", err)
		}
		return nil
	})
}

func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	query := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, access_token, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE id = $1
	`
	var snap domain.OrderSnapshot
	err := r.conn(ctx).QueryRow(ctx, query, id).Scan(
		&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
		&snap.CategoryID, &snap.SunoPrompt,
		&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
		&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken, &snap.CreatedAt, &snap.UpdatedAt,
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
		SELECT id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, access_token, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE invoice_id = $1
	`
	var snap domain.OrderSnapshot
	err := r.conn(ctx).QueryRow(ctx, query, invoiceID).Scan(
		&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
		&snap.CategoryID, &snap.SunoPrompt,
		&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
		&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken, &snap.CreatedAt, &snap.UpdatedAt,
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

	return runAtomic(ctx, r.pool, func(ctx context.Context, db dbConn) error {
		query := `
			UPDATE orders
			SET payment_status = $1, generation_status = $2, assigned_account_id = $3, failure_reason = $4, updated_at = $5, paid_at = $6, completed_at = $7
			WHERE id = $8
		`
		cmd, err := db.Exec(ctx, query, snap.PaymentStatus, snap.GenerationStatus, snap.AssignedAccountID, snap.FailureReason, snap.UpdatedAt, snap.PaidAt, snap.CompletedAt, snap.ID)
		if err != nil {
			return fmt.Errorf("update order: %w", err)
		}
		if cmd.RowsAffected() == 0 {
			return domain.ErrOrderNotFound
		}

		if err := r.saveTracks(ctx, db, snap.ID, snap.Tracks); err != nil {
			return fmt.Errorf("update tracks: %w", err)
		}
		return nil
	})
}

// ApplyPaymentSuccess атомарно и идемпотентно переводит заказ в paid+queued.
// Условие WHERE payment_status = 'pending' гарантирует, что при двух параллельных
// доставках вебхука переход выполнит только одна транзакция (вторая получит
// RowsAffected()==0 и applied=false). Это исключает двойную постановку задачи
// генерации и двойной расход кредитов Suno без отдельной version-колонки.
func (r *OrderRepository) ApplyPaymentSuccess(ctx context.Context, order *domain.Order) (bool, error) {
	snap := order.Snapshot()

	query := `
		UPDATE orders
		SET payment_status = $1, generation_status = $2, updated_at = $3, paid_at = $4
		WHERE id = $5 AND payment_status = 'pending'
	`
	cmd, err := r.conn(ctx).Exec(ctx, query,
		snap.PaymentStatus, snap.GenerationStatus, snap.UpdatedAt, snap.PaidAt, snap.ID,
	)
	if err != nil {
		return false, fmt.Errorf("apply payment success: %w", err)
	}
	return cmd.RowsAffected() > 0, nil
}

func (r *OrderRepository) ListByCustomerEmail(ctx context.Context, email string, limit, offset int) ([]*domain.Order, error) {
	// 1. Загружаем заказы клиента одним запросом с пагинацией.
	orderQuery := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, access_token, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE customer_email = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.conn(ctx).Query(ctx, orderQuery, email, limit, offset)
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
			&snap.CategoryID, &snap.SunoPrompt,
			&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
			&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken, &snap.CreatedAt, &snap.UpdatedAt,
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
	rows, err := r.conn(ctx).Query(ctx, query, orderIDs)
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

func (r *OrderRepository) ListByCustomerPhone(ctx context.Context, phone string, limit, offset int) ([]*domain.Order, error) {
	orderQuery := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, access_token, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE customer_phone = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.conn(ctx).Query(ctx, orderQuery, phone, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list orders by phone: %w", err)
	}
	defer rows.Close()

	var snaps []domain.OrderSnapshot
	var orderIDs []uuid.UUID
	for rows.Next() {
		var snap domain.OrderSnapshot
		err := rows.Scan(
			&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
			&snap.CategoryID, &snap.SunoPrompt,
			&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
			&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken, &snap.CreatedAt, &snap.UpdatedAt,
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

	tracksByOrder, err := r.getTracksForOrders(ctx, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("batch load tracks: %w", err)
	}

	orders := make([]*domain.Order, 0, len(snaps))
	for _, snap := range snaps {
		snap.Tracks = tracksByOrder[snap.ID]
		orders = append(orders, domain.RestoreOrder(snap))
	}
	return orders, nil
}

// GetByAccessToken находит заказ по токену доступа клиента.
func (r *OrderRepository) GetByAccessToken(ctx context.Context, token string) (*domain.Order, error) {
	query := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt, amount_kopecks, currency, payment_status, generation_status, assigned_account_id, failure_reason, access_token, created_at, updated_at, paid_at, completed_at
		FROM orders WHERE access_token = $1
	`
	var snap domain.OrderSnapshot
	err := r.conn(ctx).QueryRow(ctx, query, token).Scan(
		&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
		&snap.CategoryID, &snap.SunoPrompt,
		&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
		&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken, &snap.CreatedAt, &snap.UpdatedAt,
		&snap.PaidAt, &snap.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("get order by access token: %w", err)
	}

	tracks, err := r.getTracksForOrder(ctx, snap.ID)
	if err != nil {
		return nil, fmt.Errorf("get tracks for order: %w", err)
	}
	snap.Tracks = tracks
	return domain.RestoreOrder(snap), nil
}

// NextInvoiceID возвращает следующий уникальный InvId из PostgreSQL sequence.
func (r *OrderRepository) NextInvoiceID(ctx context.Context) (int64, error) {
	var id int64
	if err := r.conn(ctx).QueryRow(ctx, "SELECT nextval('invoice_id_seq')").Scan(&id); err != nil {
		return 0, fmt.Errorf("nextval invoice_id_seq: %w", err)
	}
	return id, nil
}

func (r *OrderRepository) saveTracks(ctx context.Context, db dbConn, orderID uuid.UUID, tracks []domain.Track) error {
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
		_, err := db.Exec(ctx, query, t.ID, orderID, t.Index, t.AudioURL, t.DurationSec, t.SunoTrackID)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListAll возвращает страницу всех заказов для Admin API (без фильтрации по клиенту).
func (r *OrderRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.Order, error) {
	query := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt,
		       amount_kopecks, currency, payment_status, generation_status, assigned_account_id,
		       failure_reason, access_token, created_at, updated_at, paid_at, completed_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.conn(ctx).Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list all orders: %w", err)
	}
	defer rows.Close()

	var snaps []domain.OrderSnapshot
	for rows.Next() {
		var snap domain.OrderSnapshot
		if err := rows.Scan(
			&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
			&snap.CategoryID, &snap.SunoPrompt,
			&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
			&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken, &snap.CreatedAt, &snap.UpdatedAt,
			&snap.PaidAt, &snap.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}

	ids := make([]uuid.UUID, len(snaps))
	for i, s := range snaps {
		ids[i] = s.ID
	}
	tracksMap, err := r.getTracksForOrders(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load tracks for orders: %w", err)
	}

	orders := make([]*domain.Order, 0, len(snaps))
	for _, snap := range snaps {
		snap.Tracks = tracksMap[snap.ID]
		orders = append(orders, domain.RestoreOrder(snap))
	}
	return orders, nil
}

// CountAll возвращает общее количество заказов для пагинации в Admin API.
func (r *OrderRepository) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.conn(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count all orders: %w", err)
	}
	return count, nil
}

// ListStuckProcessing возвращает заказы, застрявшие в статусе processing дольше
// порогового времени. Используется фоновым recovery-процессом для детектирования
// заказов, брошенных после краша пода/воркера.
func (r *OrderRepository) ListStuckProcessing(ctx context.Context, olderThan time.Time) ([]*domain.Order, error) {
	query := `
		SELECT id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt,
		       amount_kopecks, currency, payment_status, generation_status, assigned_account_id,
		       failure_reason, access_token, created_at, updated_at, paid_at, completed_at
		FROM orders
		WHERE generation_status = 'processing' AND updated_at < $1
		ORDER BY updated_at ASC
	`
	rows, err := r.conn(ctx).Query(ctx, query, olderThan)
	if err != nil {
		return nil, fmt.Errorf("list stuck processing orders: %w", err)
	}
	defer rows.Close()

	var snaps []domain.OrderSnapshot
	for rows.Next() {
		var snap domain.OrderSnapshot
		if err := rows.Scan(
			&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
			&snap.CategoryID, &snap.SunoPrompt,
			&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
			&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken, &snap.CreatedAt, &snap.UpdatedAt,
			&snap.PaidAt, &snap.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stuck order row: %w", err)
		}
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stuck orders: %w", err)
	}
	if len(snaps) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(snaps))
	for i, s := range snaps {
		ids[i] = s.ID
	}
	tracksMap, err := r.getTracksForOrders(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load tracks for stuck orders: %w", err)
	}

	orders := make([]*domain.Order, 0, len(snaps))
	for _, snap := range snaps {
		snap.Tracks = tracksMap[snap.ID]
		orders = append(orders, domain.RestoreOrder(snap))
	}
	return orders, nil
}

func (r *OrderRepository) getTracksForOrder(ctx context.Context, orderID uuid.UUID) ([]domain.Track, error) {
	query := `SELECT id, index, audio_url, duration_sec, suno_track_id FROM tracks WHERE order_id = $1 ORDER BY index ASC`
	rows, err := r.conn(ctx).Query(ctx, query, orderID)
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
