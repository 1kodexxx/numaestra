package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

type OrderRepository struct {
	pool PgxPool
}

func NewOrderRepository(pool PgxPool) *OrderRepository {
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
			INSERT INTO orders (id, invoice_id, customer_email, customer_phone, brief, category_id, suno_prompt, amount_kopecks, currency, payment_status, generation_status, generation_phase, generation_progress, tracks_ready, assigned_account_id, failure_reason, access_token, consent_given_at, consent_doc_version, promo_code_id, original_amount_kopecks, discount_kopecks, referral_code, created_at, updated_at, paid_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27)
		`
		_, err := db.Exec(ctx, queryOrder,
			snap.ID, snap.InvoiceID, snap.CustomerEmail, snap.CustomerPhone, snap.Brief,
			nullableString(snap.CategoryID), nullableString(snap.SunoPrompt),
			snap.AmountKopecks, snap.Currency, snap.PaymentStatus, snap.GenerationStatus,
			snap.GenerationPhase, snap.GenerationProgress, snap.TracksReady,
			snap.AssignedAccountID, snap.FailureReason, snap.AccessToken,
			snap.ConsentGivenAt, nullableString(snap.ConsentDocVersion),
			snap.PromoCodeID, nullableInt64(snap.OriginalAmountKopecks), snap.DiscountKopecks,
			nullableString(snap.ReferralCode),
			snap.CreatedAt, snap.UpdatedAt,
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
	query := `SELECT ` + orderSelectColumns + ` FROM orders WHERE id = $1`
	snap, err := scanOrderSnapshot(r.conn(ctx).QueryRow(ctx, query, id))
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
	query := `SELECT ` + orderSelectColumns + ` FROM orders WHERE invoice_id = $1`
	snap, err := scanOrderSnapshot(r.conn(ctx).QueryRow(ctx, query, invoiceID))
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
			SET payment_status = $1, generation_status = $2, generation_phase = $3, generation_progress = $4, tracks_ready = $5, assigned_account_id = $6, failure_reason = $7, share_revoked_at = $8, updated_at = $9, paid_at = $10, completed_at = $11
			WHERE id = $12
		`
		cmd, err := db.Exec(ctx, query, snap.PaymentStatus, snap.GenerationStatus, snap.GenerationPhase, snap.GenerationProgress, snap.TracksReady, snap.AssignedAccountID, snap.FailureReason, snap.ShareRevokedAt, snap.UpdatedAt, snap.PaidAt, snap.CompletedAt, snap.ID)
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

// UpdateDemo пишет ТОЛЬКО demo-колонки — это намеренная изоляция от платёжного
// пути: обычный Update не упоминает demo_*, поэтому параллельный платный апдейт
// не может затереть результат демо-генерации (и наоборот). updated_at трогаем,
// чтобы recovery-крон демо мог находить «застрявшие» processing по времени.
func (r *OrderRepository) UpdateDemo(ctx context.Context, order *domain.Order) error {
	snap := order.Snapshot()
	clipsJSON, err := json.Marshal(snap.DemoClips)
	if err != nil {
		return fmt.Errorf("marshal demo clips: %w", err)
	}
	query := `UPDATE orders SET demo_status = $1, demo_url = $2, demo_account_id = $3, demo_clips = $4::jsonb, updated_at = $5 WHERE id = $6`
	cmd, err := r.conn(ctx).Exec(ctx, query,
		string(snap.DemoStatus), nullableString(snap.DemoURL), snap.DemoAccountID, string(clipsJSON), snap.UpdatedAt, snap.ID,
	)
	if err != nil {
		return fmt.Errorf("update demo: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}

// ListStuckDemo возвращает заказы с демо в статусе processing дольше порога —
// их демо-задача потеряна (краш воркера). Recovery освобождает захваченный слот
// аккаунта и помечает демо failed, чтобы не отъедать ёмкость у платных генераций.
func (r *OrderRepository) ListStuckDemo(ctx context.Context, olderThan time.Time) ([]*domain.Order, error) {
	query := `SELECT ` + orderSelectColumns + ` FROM orders WHERE demo_status = 'processing' AND updated_at < $1 ORDER BY updated_at ASC`
	rows, err := r.conn(ctx).Query(ctx, query, olderThan)
	if err != nil {
		return nil, fmt.Errorf("list stuck demo: %w", err)
	}
	return r.ordersFromRows(ctx, rows)
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
		SET payment_status = $1, generation_status = $2, generation_phase = $3, generation_progress = $4, updated_at = $5, paid_at = $6
		WHERE id = $7 AND payment_status = 'pending'
	`
	cmd, err := r.conn(ctx).Exec(ctx, query,
		snap.PaymentStatus, snap.GenerationStatus, snap.GenerationPhase, snap.GenerationProgress, snap.UpdatedAt, snap.PaidAt, snap.ID,
	)
	if err != nil {
		return false, fmt.Errorf("apply payment success: %w", err)
	}
	return cmd.RowsAffected() > 0, nil
}

func (r *OrderRepository) ListByCustomerEmail(ctx context.Context, email string, limit, offset int) ([]*domain.Order, error) {
	orderQuery := `SELECT ` + orderSelectColumns + ` FROM orders WHERE customer_email = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.conn(ctx).Query(ctx, orderQuery, email, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list orders by email: %w", err)
	}
	return r.ordersFromRows(ctx, rows)
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

func (r *OrderRepository) ordersFromRows(ctx context.Context, rows pgx.Rows) ([]*domain.Order, error) {
	defer rows.Close()
	snaps, err := scanOrderRows(rows)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("load tracks for orders: %w", err)
	}
	orders := make([]*domain.Order, 0, len(snaps))
	for _, snap := range snaps {
		snap.Tracks = tracksMap[snap.ID]
		orders = append(orders, domain.RestoreOrder(snap))
	}
	return orders, nil
}

func (r *OrderRepository) ListByCustomerPhone(ctx context.Context, phone string, limit, offset int) ([]*domain.Order, error) {
	orderQuery := `SELECT ` + orderSelectColumns + ` FROM orders WHERE customer_phone = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.conn(ctx).Query(ctx, orderQuery, phone, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list orders by phone: %w", err)
	}
	return r.ordersFromRows(ctx, rows)
}

// GetByAccessToken находит заказ по токену доступа клиента.
func (r *OrderRepository) GetByAccessToken(ctx context.Context, token string) (*domain.Order, error) {
	query := `SELECT ` + orderSelectColumns + ` FROM orders WHERE access_token = $1`
	snap, err := scanOrderSnapshot(r.conn(ctx).QueryRow(ctx, query, token))
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

// SetAdminFeedback сохраняет сообщение администратора по заказу.
func (r *OrderRepository) SetAdminFeedback(ctx context.Context, id uuid.UUID, feedback string, at time.Time) error {
	cmd, err := r.conn(ctx).Exec(ctx, `
		UPDATE orders SET admin_feedback = $1, admin_feedback_at = $2, updated_at = $2 WHERE id = $3
	`, feedback, at, id)
	if err != nil {
		return fmt.Errorf("set admin feedback: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
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
	query := `SELECT ` + orderSelectColumns + ` FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.conn(ctx).Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list all orders: %w", err)
	}
	return r.ordersFromRows(ctx, rows)
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

// Stats возвращает агрегаты по заказам для дашборда админки одним запросом.
func (r *OrderRepository) Stats(ctx context.Context) (domain.OrderStats, error) {
	var s domain.OrderStats
	err := r.conn(ctx).QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE payment_status = 'paid'),
			COALESCE(SUM(amount_kopecks) FILTER (WHERE payment_status = 'paid'), 0),
			COUNT(*) FILTER (WHERE generation_status = 'completed'),
			COUNT(*) FILTER (WHERE generation_status IN ('queued', 'processing')),
			COUNT(*) FILTER (WHERE generation_status = 'failed'),
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '24 hours'),
			COUNT(*) FILTER (WHERE demo_status = 'ready'),
			COUNT(*) FILTER (WHERE demo_status = 'ready' AND created_at >= NOW() - INTERVAL '24 hours'),
			COUNT(*) FILTER (WHERE demo_status = 'ready' AND payment_status = 'paid')
		FROM orders
	`).Scan(&s.TotalOrders, &s.PaidOrders, &s.RevenueKopecks, &s.Completed, &s.Processing, &s.Failed, &s.OrdersToday,
		&s.DemosReady, &s.DemosToday, &s.DemosConverted)
	if err != nil {
		return domain.OrderStats{}, fmt.Errorf("order stats: %w", err)
	}
	return s, nil
}

// ListStuckProcessing возвращает заказы, застрявшие в статусе processing дольше
// порогового времени. Используется фоновым recovery-процессом для детектирования
// заказов, брошенных после краша пода/воркера.
func (r *OrderRepository) ListStuckProcessing(ctx context.Context, olderThan time.Time) ([]*domain.Order, error) {
	return r.listStuckByCondition(ctx, "generation_status = 'processing'", olderThan)
}

// ListStuckQueued возвращает оплаченные заказы, застрявшие в статусе queued дольше
// порогового времени: их Asynq-задача потеряна или исчерпала ретраи. Фильтр
// payment_status='paid' исключает неоплаченные заказы, которым генерация не положена.
func (r *OrderRepository) ListStuckQueued(ctx context.Context, olderThan time.Time) ([]*domain.Order, error) {
	return r.listStuckByCondition(ctx, "generation_status = 'queued' AND payment_status = 'paid'", olderThan)
}

// listStuckByCondition — общая выборка застрявших заказов по статусному условию
// (condition подставляется из доверенных литералов вызывающих методов, не из
// пользовательского ввода) и порогу времени по updated_at.
func (r *OrderRepository) listStuckByCondition(ctx context.Context, condition string, olderThan time.Time) ([]*domain.Order, error) {
	query := `SELECT ` + orderSelectColumns + ` FROM orders WHERE ` + condition + ` AND updated_at < $1 ORDER BY updated_at ASC`
	rows, err := r.conn(ctx).Query(ctx, query, olderThan)
	if err != nil {
		return nil, fmt.Errorf("list stuck orders: %w", err)
	}
	return r.ordersFromRows(ctx, rows)
}

// reconcileBatchLimit ограничивает число pending-заказов, опрашиваемых в Robokassa
// за один прогон сверки, чтобы не создавать всплеск запросов к OpStateExt по
// брошенным корзинам.
const reconcileBatchLimit = 200

// ListPendingPayment возвращает неоплаченные (payment_status='pending') заказы,
// созданные в окне [createdAfter, createdBefore]. Фоновая сверка платежей по ним
// опрашивает Robokassa (OpStateExt) и активирует фактически оплаченные, чей вебхук
// ResultURL не дошёл. Нижняя граница окна отсекает свежие заказы (клиент ещё на
// странице оплаты), верхняя — выход за платёжное окно.
func (r *OrderRepository) ListPendingPayment(ctx context.Context, createdAfter, createdBefore time.Time) ([]*domain.Order, error) {
	query := `SELECT ` + orderSelectColumns + ` FROM orders
		WHERE payment_status = 'pending' AND created_at >= $1 AND created_at <= $2
		ORDER BY created_at ASC LIMIT ` + strconv.Itoa(reconcileBatchLimit)
	rows, err := r.conn(ctx).Query(ctx, query, createdAfter, createdBefore)
	if err != nil {
		return nil, fmt.Errorf("list pending payment orders: %w", err)
	}
	return r.ordersFromRows(ctx, rows)
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

func (r *OrderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.conn(ctx).Exec(ctx, `DELETE FROM orders WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}
