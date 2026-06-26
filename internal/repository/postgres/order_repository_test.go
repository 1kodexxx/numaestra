package postgres

import (
	"context"
	"errors"
	"testing"

	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/numaestra/numaestra/internal/domain"
)

func testNow() time.Time { return time.Now().UTC() }

// orderCols — колонки в SELECT-ах orders (в порядке Scan).
// Должен совпадать с orderSelectColumns в order_columns.go.
var orderCols = []string{
	"id", "invoice_id", "customer_email", "customer_phone", "brief", "category_id", "suno_prompt",
	"amount_kopecks", "currency", "payment_status", "generation_status", "generation_phase", "generation_progress", "tracks_ready",
	"assigned_account_id",
	"failure_reason", "access_token", "admin_feedback", "admin_feedback_at",
	"consent_given_at", "consent_doc_version", "share_revoked_at",
	"promo_code_id", "original_amount_kopecks", "discount_kopecks", "referral_code",
	"created_at", "updated_at", "paid_at", "completed_at",
	"demo_status", "demo_url", "demo_account_id",
}

// orderRowValues — одна строка заказа для pgxmock (без треков, без nullable-времён).
func orderRowValues(id uuid.UUID, invoice int64) []any {
	now := time.Now().UTC()
	return []any{
		id, invoice, "user@example.com", "", "Бриф", "", "",
		int64(200000), "RUB", domain.PaymentStatusPaid, domain.GenerationStatusCompleted,
		domain.GenerationPhaseCompleted, 100, 4,
		(*uuid.UUID)(nil),
		"", "tok", "", (*time.Time)(nil), (*time.Time)(nil), "", (*time.Time)(nil),
		(*uuid.UUID)(nil), int64(0), int64(0), "",
		now, now, (*time.Time)(nil), (*time.Time)(nil),
		domain.DemoStatusNone, "", (*uuid.UUID)(nil),
	}
}

// emptyTrackRows — пустой набор треков (5 колонок getTracksForOrder).
func emptyTrackRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "index", "audio_url", "duration_sec", "suno_track_id"})
}

// emptyBatchTrackRows — пустой набор для getTracksForOrders (6 колонок, с order_id).
func emptyBatchTrackRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "order_id", "index", "audio_url", "duration_sec", "suno_track_id"})
}

func TestOrderRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM orders WHERE id =").WithArgs(anyArgs(1)...).
		WillReturnRows(pgxmock.NewRows(orderCols).AddRow(orderRowValues(id, 7)...))
	mock.ExpectQuery("FROM tracks WHERE order_id =").WithArgs(anyArgs(1)...).WillReturnRows(emptyTrackRows())

	repo := NewOrderRepository(mock)
	order, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if order.ID() != id || order.InvoiceID() != 7 {
		t.Errorf("неверно разобран заказ: id=%s invoice=%d", order.ID(), order.InvoiceID())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_GetByInvoiceID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM orders WHERE invoice_id =").WithArgs(anyArgs(1)...).
		WillReturnRows(pgxmock.NewRows(orderCols).AddRow(orderRowValues(id, 42)...))
	mock.ExpectQuery("FROM tracks WHERE order_id =").WithArgs(anyArgs(1)...).WillReturnRows(emptyTrackRows())

	repo := NewOrderRepository(mock)
	order, err := repo.GetByInvoiceID(context.Background(), 42)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if order.InvoiceID() != 42 {
		t.Errorf("ожидали invoice 42, получили %d", order.InvoiceID())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_GetByAccessToken_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM orders WHERE access_token =").WithArgs(anyArgs(1)...).
		WillReturnRows(pgxmock.NewRows(orderCols).AddRow(orderRowValues(id, 8)...))
	mock.ExpectQuery("FROM tracks WHERE order_id =").WithArgs(anyArgs(1)...).WillReturnRows(emptyTrackRows())

	repo := NewOrderRepository(mock)
	if _, err := repo.GetByAccessToken(context.Background(), "tok"); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_ListAll_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM orders").WithArgs(anyArgs(2)...).
		WillReturnRows(pgxmock.NewRows(orderCols).
			AddRow(orderRowValues(uuid.New(), 1)...).
			AddRow(orderRowValues(uuid.New(), 2)...))
	mock.ExpectQuery("FROM tracks").WithArgs(anyArgs(1)...).WillReturnRows(emptyBatchTrackRows())

	repo := NewOrderRepository(mock)
	orders, err := repo.ListAll(context.Background(), 20, 0)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(orders) != 2 {
		t.Errorf("ожидали 2 заказа, получили %d", len(orders))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_ListByCustomerEmail_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM orders WHERE customer_email =").WithArgs(anyArgs(3)...).
		WillReturnRows(pgxmock.NewRows(orderCols).AddRow(orderRowValues(uuid.New(), 1)...))
	mock.ExpectQuery("FROM tracks").WithArgs(anyArgs(1)...).WillReturnRows(emptyBatchTrackRows())

	repo := NewOrderRepository(mock)
	orders, err := repo.ListByCustomerEmail(context.Background(), "user@example.com", 20, 0)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(orders) != 1 {
		t.Errorf("ожидали 1 заказ, получили %d", len(orders))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_ListByCustomerPhone_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM orders WHERE customer_phone =").WithArgs(anyArgs(3)...).
		WillReturnRows(pgxmock.NewRows(orderCols).AddRow(orderRowValues(uuid.New(), 1)...))
	mock.ExpectQuery("FROM tracks").WithArgs(anyArgs(1)...).WillReturnRows(emptyBatchTrackRows())

	repo := NewOrderRepository(mock)
	orders, err := repo.ListByCustomerPhone(context.Background(), "+70000000000", 20, 0)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(orders) != 1 {
		t.Errorf("ожидали 1 заказ, получили %d", len(orders))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_ListStuckProcessing_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("generation_status = 'processing'").WithArgs(anyArgs(1)...).
		WillReturnRows(pgxmock.NewRows(orderCols).AddRow(orderRowValues(uuid.New(), 1)...))
	mock.ExpectQuery("FROM tracks").WithArgs(anyArgs(1)...).WillReturnRows(emptyBatchTrackRows())

	repo := NewOrderRepository(mock)
	orders, err := repo.ListStuckProcessing(context.Background(), testNow())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(orders) != 1 {
		t.Errorf("ожидали 1 заказ, получили %d", len(orders))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func pendingOrder() *domain.Order {
	return domain.RestoreOrder(domain.OrderSnapshot{
		ID: uuid.New(), InvoiceID: 1001, CustomerEmail: "user@example.com",
		AmountKopecks: 200000, Currency: "RUB",
		PaymentStatus: domain.PaymentStatusPaid, GenerationStatus: domain.GenerationStatusQueued,
	})
}

func TestOrderRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM orders WHERE id =").WithArgs(anyArgs(1)...).WillReturnError(pgx.ErrNoRows)

	repo := NewOrderRepository(mock)
	if _, err := repo.GetByID(context.Background(), uuid.New()); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("ожидали ErrOrderNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

// ApplyPaymentSuccess — самый чувствительный путь: защита от двойной оплаты.
// UPDATE ... WHERE payment_status='pending' выполняет переход ровно один раз.

func TestOrderRepository_ApplyPaymentSuccess_Applied(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Первая доставка вебхука: затронута одна строка → переход применён.
	mock.ExpectExec("UPDATE orders").WithArgs(anyArgs(7)...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewOrderRepository(mock)
	applied, err := repo.ApplyPaymentSuccess(context.Background(), pendingOrder())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !applied {
		t.Fatal("ожидали applied=true при RowsAffected=1")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_ApplyPaymentSuccess_Idempotent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Повторная доставка того же вебхука: строка уже не 'pending', RowsAffected=0
	// → applied=false, без ошибки. Это исключает двойную постановку генерации.
	mock.ExpectExec("UPDATE orders").WithArgs(anyArgs(7)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewOrderRepository(mock)
	applied, err := repo.ApplyPaymentSuccess(context.Background(), pendingOrder())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if applied {
		t.Fatal("ожидали applied=false при RowsAffected=0 (идемпотентность)")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

// TestOrderRepository_ApplyPaymentSuccess_PersistsGenerationPhase проверяет, что
// ApplyPaymentSuccess сохраняет generation_phase/generation_progress, а не только
// generation_status. Order.Enqueue() в памяти переводит фазу в "queued" (3%) —
// до этого фикса SQL-запрос эти колонки не трогал, и страница статуса показывала
// неверный текст ("Создаём музыку…") пока воркер не забирал задачу в обработку.
func TestOrderRepository_ApplyPaymentSuccess_PersistsGenerationPhase(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	order, err := domain.NewOrder(1001, "user@example.com", "", "бриф", "", "", domain.CurrentConsentDocVersion, 200000)
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	if err := order.MarkPaid(); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	if err := order.Enqueue(); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if order.GenerationPhase() != domain.GenerationPhaseQueued || order.GenerationProgress() != 3 {
		t.Fatalf("предусловие теста нарушено: phase=%q progress=%d", order.GenerationPhase(), order.GenerationProgress())
	}

	args := append([]any{domain.PaymentStatusPaid, domain.GenerationStatusQueued, domain.GenerationPhaseQueued, 3}, anyArgs(3)...)
	mock.ExpectExec("UPDATE orders").
		WithArgs(args...).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewOrderRepository(mock)
	applied, err := repo.ApplyPaymentSuccess(context.Background(), order)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !applied {
		t.Fatal("ожидали applied=true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания (generation_phase/progress не переданы в запрос?): %v", err)
	}
}

func TestOrderRepository_Update_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Update оборачивается в транзакцию (runAtomic). RowsAffected=0 → откат и ErrOrderNotFound.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders").WithArgs(anyArgs(12)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	mock.ExpectRollback()

	repo := NewOrderRepository(mock)
	if err := repo.Update(context.Background(), pendingOrder()); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("ожидали ErrOrderNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_SetAdminFeedback_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE orders SET admin_feedback").WithArgs(anyArgs(3)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewOrderRepository(mock)
	if err := repo.SetAdminFeedback(context.Background(), uuid.New(), "готово", testNow()); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("ожидали ErrOrderNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_Delete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	mock.ExpectExec("DELETE FROM orders").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 1))

	repo := NewOrderRepository(mock)
	if err := repo.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestOrderRepository_Delete_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM orders").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 0))

	repo := NewOrderRepository(mock)
	if err := repo.Delete(context.Background(), uuid.New()); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("ожидали ErrOrderNotFound, получили %v", err)
	}
}

func TestOrderRepository_NextInvoiceID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"nextval"}).AddRow(int64(42))
	mock.ExpectQuery("nextval").WillReturnRows(rows)

	repo := NewOrderRepository(mock)
	id, err := repo.NextInvoiceID(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if id != 42 {
		t.Fatalf("ожидали 42, получили %d", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_Stats(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"total", "paid", "revenue", "completed", "processing", "failed", "today"}).
		AddRow(10, 7, int64(1400000), 5, 2, 1, 8)
	mock.ExpectQuery("FROM orders").WillReturnRows(rows)

	repo := NewOrderRepository(mock)
	s, err := repo.Stats(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if s.TotalOrders != 10 || s.PaidOrders != 7 || s.RevenueKopecks != 1400000 {
		t.Errorf("неверные агрегаты: %+v", s)
	}
	if s.Completed != 5 || s.Processing != 2 || s.Failed != 1 || s.OrdersToday != 8 {
		t.Errorf("неверная разбивка по статусам: %+v", s)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestOrderRepository_CountAll(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"count"}).AddRow(7)
	mock.ExpectQuery("COUNT").WillReturnRows(rows)

	repo := NewOrderRepository(mock)
	count, err := repo.CountAll(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if count != 7 {
		t.Fatalf("ожидали 7, получили %d", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}
