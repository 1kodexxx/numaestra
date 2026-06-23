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
	mock.ExpectExec("UPDATE orders").WithArgs(anyArgs(5)...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

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
	mock.ExpectExec("UPDATE orders").WithArgs(anyArgs(5)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

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

func TestOrderRepository_Update_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Update оборачивается в транзакцию (runAtomic). RowsAffected=0 → откат и ErrOrderNotFound.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders").WithArgs(anyArgs(8)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
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
