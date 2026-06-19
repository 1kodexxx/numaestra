//go:build integration

// Package postgres integration-тесты проверяют реальные SQL-запросы на эфемерном
// инстансе PostgreSQL, поднимаемом через testcontainers-go. В отличие от
// in-memory моков, здесь проверяются тонкие места, которые невозможно покрыть
// заглушками: FOR UPDATE SKIP LOCKED, инкремент слотов под конкуренцией,
// батч-загрузка треков через ANY($1) и атомарность Unit of Work.
//
// Запуск (нужен работающий Docker):
//
//	go test -tags=integration ./internal/repository/postgres/...
package postgres

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/migrations"
	"github.com/numaestra/numaestra/pkg/migrate"
)

var (
	testPool     *pgxpool.Pool
	testSetupErr error
)

func TestMain(m *testing.M) {
	code := func() int {
		ctx := context.Background()

		container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("numaestra_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForListeningPort("5432/tcp").WithStartupTimeout(90*time.Second),
			),
		)
		if err != nil {
			testSetupErr = fmt.Errorf("запуск контейнера postgres: %w", err)
			return m.Run()
		}
		defer func() { _ = container.Terminate(ctx) }()

		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			testSetupErr = fmt.Errorf("строка подключения: %w", err)
			return m.Run()
		}

		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			testSetupErr = fmt.Errorf("пул соединений: %w", err)
			return m.Run()
		}
		defer pool.Close()

		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		if err := migrate.Run(ctx, pool, migrations.FS, log); err != nil {
			testSetupErr = fmt.Errorf("применение миграций: %w", err)
			return m.Run()
		}

		testPool = pool
		return m.Run()
	}()
	os.Exit(code)
}

// setup готовит чистую БД к тесту: пропускает тест, если Docker недоступен, и
// очищает таблицы между тестами для детерминированности.
func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testSetupErr != nil {
		t.Skipf("интеграционная среда недоступна: %v", testSetupErr)
	}
	if testPool == nil {
		t.Skip("пул postgres не инициализирован")
	}
	_, err := testPool.Exec(context.Background(),
		`TRUNCATE orders, tracks, suno_accounts RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("очистка таблиц: %v", err)
	}
	return testPool
}

func mustAccount(t *testing.T, pool *pgxpool.Pool, tokens, maxConcurrent int) *domain.SunoAccount {
	t.Helper()
	acc, err := domain.NewSunoAccount(fmt.Sprintf("acc-%s@suno.local", uuidish()), "encrypted-session", tokens)
	if err != nil {
		t.Fatalf("создание аккаунта: %v", err)
	}
	if err := acc.SetMaxConcurrentTasks(maxConcurrent); err != nil {
		t.Fatalf("установка лимита слотов: %v", err)
	}
	if err := NewAccountRepository(pool).Create(context.Background(), acc); err != nil {
		t.Fatalf("сохранение аккаунта: %v", err)
	}
	return acc
}

func mustOrder(t *testing.T, pool *pgxpool.Pool) *domain.Order {
	t.Helper()
	repo := NewOrderRepository(pool)
	inv, err := repo.NextInvoiceID(context.Background())
	if err != nil {
		t.Fatalf("invoice id: %v", err)
	}
	order, err := domain.NewOrder(inv, "user@example.com", "", "Бриф", 150000)
	if err != nil {
		t.Fatalf("создание заказа: %v", err)
	}
	if err := repo.Create(context.Background(), order); err != nil {
		t.Fatalf("сохранение заказа: %v", err)
	}
	return order
}

// uuidish даёт уникальный суффикс для email, чтобы не нарушать UNIQUE(email).
func uuidish() string {
	return time.Now().Format("150405.000000000")
}

// --- Order round-trip ---

func TestIntegration_OrderRoundTrip(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	order := mustOrder(t, pool)
	got, err := repo.GetByID(ctx, order.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID() != order.ID() || got.InvoiceID() != order.InvoiceID() {
		t.Errorf("прочитанный заказ не совпадает: %+v", got)
	}
	if got.AmountKopecks() != 150000 {
		t.Errorf("ожидали сумму 150000, получили %d", got.AmountKopecks())
	}
}

func TestIntegration_NextInvoiceID_Monotonic(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	a, err := repo.NextInvoiceID(ctx)
	if err != nil {
		t.Fatalf("nextval: %v", err)
	}
	b, err := repo.NextInvoiceID(ctx)
	if err != nil {
		t.Fatalf("nextval: %v", err)
	}
	if b <= a {
		t.Errorf("invoice id должен монотонно расти: %d, затем %d", a, b)
	}
}

// --- ApplyPaymentSuccess идемпотентность на реальной БД ---

func TestIntegration_ApplyPaymentSuccess_OnlyFromPending(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	order := mustOrder(t, pool)
	if err := order.MarkPaid(); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	if err := order.Enqueue(); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	applied, err := repo.ApplyPaymentSuccess(ctx, order)
	if err != nil || !applied {
		t.Fatalf("первый переход должен примениться: applied=%v err=%v", applied, err)
	}
	applied2, err := repo.ApplyPaymentSuccess(ctx, order)
	if err != nil {
		t.Fatalf("повторный вызов не должен падать: %v", err)
	}
	if applied2 {
		t.Error("повторный переход из не-pending должен дать applied=false (условный UPDATE)")
	}
}

// --- FOR UPDATE SKIP LOCKED: параллельный захват разных аккаунтов ---

func TestIntegration_FetchAndLock_SkipLocked_DistributesAccounts(t *testing.T) {
	pool := setup(t)
	repo := NewAccountRepository(pool)
	ctx := context.Background()

	const n = 5
	for i := 0; i < n; i++ {
		mustAccount(t, pool, 10, 1) // каждый аккаунт обслуживает 1 задачу
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		ids  = map[string]int{}
		errs int
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			acc, err := repo.FetchAndLockAvailable(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs++
				return
			}
			ids[acc.ID().String()]++
		}()
	}
	wg.Wait()

	if errs != 0 {
		t.Errorf("при %d аккаунтах и %d воркерах все должны получить аккаунт, ошибок=%d", n, n, errs)
	}
	if len(ids) != n {
		t.Errorf("SKIP LOCKED должен выдать %d РАЗНЫХ аккаунтов, получили %d уникальных (%v)", n, len(ids), ids)
	}
	for id, cnt := range ids {
		if cnt != 1 {
			t.Errorf("аккаунт %s выдан %d раз — SKIP LOCKED нарушен", id, cnt)
		}
	}
}

// --- Слоты конкурентности: один аккаунт обслуживает несколько задач ---

func TestIntegration_FetchAndLock_RespectsConcurrencySlots(t *testing.T) {
	pool := setup(t)
	repo := NewAccountRepository(pool)
	ctx := context.Background()

	acc := mustAccount(t, pool, 10, 3) // лимит 3 одновременных задачи

	// Три последовательных захвата должны пройти, заняв слоты 1..3.
	for i := 1; i <= 3; i++ {
		got, err := repo.FetchAndLockAvailable(ctx)
		if err != nil {
			t.Fatalf("захват %d должен пройти при лимите 3: %v", i, err)
		}
		if got.ID() != acc.ID() {
			t.Fatalf("ожидали тот же аккаунт, получили другой")
		}
		if got.ConcurrentTasks() != i {
			t.Errorf("после захвата %d ожидали concurrent_tasks=%d, получили %d", i, i, got.ConcurrentTasks())
		}
	}

	// Слоты исчерпаны — следующий захват должен вернуть ErrNoAvailableAccount.
	if _, err := repo.FetchAndLockAvailable(ctx); err != domain.ErrNoAvailableAccount {
		t.Errorf("при исчерпанных слотах ожидали ErrNoAvailableAccount, получили %v", err)
	}

	// Освобождаем один слот через доменную операцию + Update и проверяем,
	// что аккаунт снова доступен.
	got, _ := repo.GetByID(ctx, acc.ID())
	got.ReleaseSlot()
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update после ReleaseSlot: %v", err)
	}
	if _, err := repo.FetchAndLockAvailable(ctx); err != nil {
		t.Errorf("после освобождения слота аккаунт снова должен выдаваться: %v", err)
	}
}

// --- Unit of Work: атомарное сохранение заказа и аккаунта ---

func TestIntegration_TxManager_CommitsBothAggregates(t *testing.T) {
	pool := setup(t)
	orderRepo := NewOrderRepository(pool)
	accRepo := NewAccountRepository(pool)
	tx := NewTxManager(pool)
	ctx := context.Background()

	acc := mustAccount(t, pool, 10, 2)
	order := mustOrder(t, pool)
	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(acc.ID())
	_ = acc.AcquireSlot(time.Now().UTC())

	err := tx.Do(ctx, func(ctx context.Context) error {
		if err := orderRepo.Update(ctx, order); err != nil {
			return err
		}
		return accRepo.Update(ctx, acc)
	})
	if err != nil {
		t.Fatalf("Unit of Work упал: %v", err)
	}

	gotOrder, _ := orderRepo.GetByID(ctx, order.ID())
	if gotOrder.GenerationStatus() != domain.GenerationStatusProcessing {
		t.Errorf("заказ должен быть processing, получили %q", gotOrder.GenerationStatus())
	}
	gotAcc, _ := accRepo.GetByID(ctx, acc.ID())
	if gotAcc.ConcurrentTasks() != 1 {
		t.Errorf("у аккаунта должен быть занят 1 слот, получили %d", gotAcc.ConcurrentTasks())
	}
}

func TestIntegration_TxManager_RollsBackOnError(t *testing.T) {
	pool := setup(t)
	orderRepo := NewOrderRepository(pool)
	tx := NewTxManager(pool)
	ctx := context.Background()

	order := mustOrder(t, pool)
	_ = order.MarkPaid()
	_ = order.Enqueue()

	sentinel := fmt.Errorf("искусственный сбой после изменения заказа")
	err := tx.Do(ctx, func(ctx context.Context) error {
		if err := orderRepo.Update(ctx, order); err != nil {
			return err
		}
		return sentinel // транзакция должна откатиться
	})
	if err != sentinel {
		t.Fatalf("ожидали проброс ошибки sentinel, получили %v", err)
	}

	// Изменения должны быть отменены: заказ всё ещё pending/new.
	got, _ := orderRepo.GetByID(ctx, order.ID())
	if got.PaymentStatus() != domain.PaymentStatusPending {
		t.Errorf("после отката заказ должен остаться pending, получили %q", got.PaymentStatus())
	}
	if got.GenerationStatus() != domain.GenerationStatusNew {
		t.Errorf("после отката заказ должен остаться new, получили %q", got.GenerationStatus())
	}
}

// --- Батч-загрузка треков через ANY($1) ---

func TestIntegration_ListByEmail_BatchLoadsTracks(t *testing.T) {
	pool := setup(t)
	orderRepo := NewOrderRepository(pool)
	tx := NewTxManager(pool)
	ctx := context.Background()

	acc := mustAccount(t, pool, 10, 1)

	// Готовим завершённый заказ с треками.
	order := mustOrder(t, pool)
	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(acc.ID())
	_ = order.Complete([]domain.Track{
		{ID: uuid.New(), Index: 1, AudioURL: "https://s3/1.mp3", DurationSec: 180, SunoTrackID: "s1"},
		{ID: uuid.New(), Index: 2, AudioURL: "https://s3/2.mp3", DurationSec: 200, SunoTrackID: "s2"},
	})
	if err := tx.Do(ctx, func(ctx context.Context) error {
		return orderRepo.Update(ctx, order)
	}); err != nil {
		t.Fatalf("сохранение завершённого заказа: %v", err)
	}

	orders, err := orderRepo.ListByCustomerEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("ListByCustomerEmail: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("ожидали 1 заказ, получили %d", len(orders))
	}
	if len(orders[0].Tracks()) != 2 {
		t.Errorf("ожидали 2 трека, загруженных батчем, получили %d", len(orders[0].Tracks()))
	}
}
