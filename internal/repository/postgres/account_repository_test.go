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
	"github.com/numaestra/numaestra/pkg/encryption"
)

// Эти тесты проверяют SQL-логику репозиториев без живого PostgreSQL — через
// pgxmock. Они дополняют интеграционные тесты (//go:build integration), покрывая
// то, что важно и без реальной БД: маппинг ошибок драйвера на доменные, разбор
// RowsAffected и порядок begin/commit/rollback. Запускаются обычным `go test`.

// anyArgs возвращает n матчеров pgxmock.AnyArg() — когда тест проверяет
// поведение (маппинг ошибок, RowsAffected), а не конкретные значения аргументов.
func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func testCipherT(t *testing.T) encryption.Cipher {
	t.Helper()
	c, err := encryption.New(make([]byte, 32))
	if err != nil {
		t.Fatalf("encryption.New: %v", err)
	}
	return c
}

// accountColumns — порядок колонок в SELECT-ах suno_accounts.
var accountColumns = []string{
	"id", "email", "encrypted_session", "status", "token_balance", "failure_count",
	"max_concurrent_tasks", "concurrent_tasks", "cooldown_until", "last_used_at", "created_at", "updated_at",
}

// activeAccountRow — строка активного аккаунта со свободным слотом. Сессию
// оставляем пустой, чтобы не требовался реальный шифротекст для расшифровки.
func activeAccountRow(id uuid.UUID) []any {
	now := time.Now().UTC()
	return []any{
		id, "suno@example.com", "", domain.AccountStatusActive, 10, 0,
		3, 0, (*time.Time)(nil), (*time.Time)(nil), now, now,
	}
}

func TestAccountRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM suno_accounts WHERE id =").WithArgs(anyArgs(1)...).WillReturnError(pgx.ErrNoRows)

	repo := NewAccountRepository(mock, testCipherT(t))
	_, err = repo.GetByID(context.Background(), uuid.New())

	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("ожидали ErrAccountNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestAccountRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	rows := pgxmock.NewRows(accountColumns).AddRow(activeAccountRow(id)...)
	mock.ExpectQuery("FROM suno_accounts WHERE id =").WithArgs(anyArgs(1)...).WillReturnRows(rows)

	repo := NewAccountRepository(mock, testCipherT(t))
	acc, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if acc.ID() != id {
		t.Errorf("ID: ожидали %s, получили %s", id, acc.ID())
	}
	if acc.Status() != domain.AccountStatusActive {
		t.Errorf("Status: ожидали active, получили %s", acc.Status())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestAccountRepository_Update_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE suno_accounts").WithArgs(anyArgs(9)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewAccountRepository(mock, testCipherT(t))
	acc := domain.RestoreSunoAccount(domain.SunoAccountSnapshot{
		ID: uuid.New(), Email: "a@b.c", Status: domain.AccountStatusActive,
		TokenBalance: 5, MaxConcurrentTasks: 3,
	})

	// RowsAffected()==0 означает, что строки с таким id нет.
	if err := repo.Update(context.Background(), acc); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("ожидали ErrAccountNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestAccountRepository_SetStatus_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE suno_accounts SET status").WithArgs(anyArgs(3)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewAccountRepository(mock, testCipherT(t))
	if err := repo.SetStatus(context.Background(), uuid.New(), domain.AccountStatusBanned); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("ожидали ErrAccountNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestAccountRepository_FetchAndLockAvailable_NoneAvailable(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Пустой пул: транзакция открывается, SELECT ... FOR UPDATE SKIP LOCKED не
	// находит строк, транзакция откатывается, наружу — ErrNoAvailableAccount.
	mock.ExpectBegin()
	mock.ExpectQuery("FOR UPDATE SKIP LOCKED").WillReturnError(pgx.ErrNoRows)
	mock.ExpectRollback()

	repo := NewAccountRepository(mock, testCipherT(t))
	_, err = repo.FetchAndLockAvailable(context.Background())
	if !errors.Is(err, domain.ErrNoAvailableAccount) {
		t.Fatalf("ожидали ErrNoAvailableAccount, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestAccountRepository_FetchAndLockAvailable_AcquiresSlot(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	rows := pgxmock.NewRows(accountColumns).AddRow(activeAccountRow(id)...)

	// Успешный захват: begin → SELECT свободного аккаунта → UPDATE счётчика слотов → commit.
	mock.ExpectBegin()
	mock.ExpectQuery("FOR UPDATE SKIP LOCKED").WillReturnRows(rows)
	mock.ExpectExec("UPDATE suno_accounts SET concurrent_tasks").WithArgs(anyArgs(4)...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	repo := NewAccountRepository(mock, testCipherT(t))
	acc, err := repo.FetchAndLockAvailable(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if acc.ID() != id {
		t.Errorf("ID: ожидали %s, получили %s", id, acc.ID())
	}
	// Слот занят доменом перед записью — счётчик должен вырасти с 0 до 1.
	if acc.ConcurrentTasks() != 1 {
		t.Errorf("ConcurrentTasks: ожидали 1, получили %d", acc.ConcurrentTasks())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}
