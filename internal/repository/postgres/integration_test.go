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
	"errors"
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
	"github.com/numaestra/numaestra/pkg/encryption"
	"github.com/numaestra/numaestra/pkg/migrate"
)

// testCipher — фиксированный 32-байтовый ключ для тестовых данных.
var testCipher = func() encryption.Cipher {
	c, err := encryption.New(make([]byte, 32))
	if err != nil {
		panic(err)
	}
	return c
}()

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
	if err := NewAccountRepository(pool, testCipher).Create(context.Background(), acc); err != nil {
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
	order, err := domain.NewOrder(inv, "user@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 150000)
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

// TestIntegration_OrderWithoutCategory_NullableColumnsScanCleanly — регрессионный
// тест: mustOrder создаёт заказ со свободным сценарием (без категории), поэтому
// category_id и suno_prompt физически NULL в БД (nullableString("") -> nil при
// INSERT). pgx v5 по умолчанию падает с "cannot scan NULL into *string" при чтении
// such NULL-колонки в Go string без COALESCE на стороне SQL — проверяем, что
// репозиторий оборачивает эти колонки в COALESCE и GetByID/GetByInvoiceID/
// GetByAccessToken не падают для самого частого сценария заказа (без категории).
func TestIntegration_OrderWithoutCategory_NullableColumnsScanCleanly(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	order := mustOrder(t, pool)

	byID, err := repo.GetByID(ctx, order.ID())
	if err != nil {
		t.Fatalf("GetByID не должен падать на NULL category_id/suno_prompt: %v", err)
	}
	if byID.CategoryID() != "" || byID.SunoPrompt() != "" {
		t.Errorf("ожидали пустые CategoryID/SunoPrompt, получили %q/%q", byID.CategoryID(), byID.SunoPrompt())
	}
	if byID.AdminFeedback() != "" || byID.AdminFeedbackAt() != nil {
		t.Errorf("новый заказ не должен иметь обратной связи, получили %q/%v", byID.AdminFeedback(), byID.AdminFeedbackAt())
	}

	if _, err := repo.GetByInvoiceID(ctx, order.InvoiceID()); err != nil {
		t.Fatalf("GetByInvoiceID не должен падать на NULL category_id/suno_prompt: %v", err)
	}
	if _, err := repo.GetByAccessToken(ctx, order.AccessToken()); err != nil {
		t.Fatalf("GetByAccessToken не должен падать на NULL category_id/suno_prompt: %v", err)
	}
}

func TestIntegration_SetAdminFeedback(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	order := mustOrder(t, pool)
	at := time.Now().UTC().Truncate(time.Second)

	if err := repo.SetAdminFeedback(ctx, order.ID(), "Уточните дату праздника", at); err != nil {
		t.Fatalf("SetAdminFeedback: %v", err)
	}

	got, err := repo.GetByID(ctx, order.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.AdminFeedback() != "Уточните дату праздника" {
		t.Errorf("ожидали сохранённую обратную связь, получили %q", got.AdminFeedback())
	}
	if got.AdminFeedbackAt() == nil || !got.AdminFeedbackAt().Equal(at) {
		t.Errorf("ожидали метку времени %v, получили %v", at, got.AdminFeedbackAt())
	}
}

func TestIntegration_SetAdminFeedback_OrderNotFound(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)

	err := repo.SetAdminFeedback(context.Background(), uuid.New(), "сообщение", time.Now())
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("ожидали ErrOrderNotFound, получили %v", err)
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
	repo := NewAccountRepository(pool, testCipher)
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
	repo := NewAccountRepository(pool, testCipher)
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

// Конкурентное освобождение слотов одной строки не должно терять декременты.
// Это прямой тест фикса гонки read-modify-write на concurrent_tasks: захватываем
// 5 слотов, затем параллельно освобождаем их классическим GetByID→ReleaseSlot→
// Update. При абсолютной записи снапшота два релиза затирали бы друг друга и
// счётчик «застревал» бы выше нуля; атомарная дельта (col = col - 1) обязана
// привести его ровно в 0.
func TestIntegration_ConcurrentSlotRelease_NoCounterDrift(t *testing.T) {
	pool := setup(t)
	repo := NewAccountRepository(pool, testCipher)
	ctx := context.Background()

	const slots = 5
	acc := mustAccount(t, pool, 100, slots)

	for i := 0; i < slots; i++ {
		if _, err := repo.FetchAndLockAvailable(ctx); err != nil {
			t.Fatalf("захват слота %d: %v", i+1, err)
		}
	}

	var wg sync.WaitGroup
	errs := make([]error, slots)
	for i := 0; i < slots; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			a, err := repo.GetByID(ctx, acc.ID())
			if err != nil {
				errs[idx] = err
				return
			}
			a.ReleaseSlot()
			errs[idx] = repo.Update(ctx, a)
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("параллельное освобождение %d: %v", i, e)
		}
	}

	got, _ := repo.GetByID(ctx, acc.ID())
	if got.ConcurrentTasks() != 0 {
		t.Errorf("после %d параллельных освобождений ожидали concurrent_tasks=0, получили %d (дрейф счётчика)", slots, got.ConcurrentTasks())
	}
}

// --- Unit of Work: атомарное сохранение заказа и аккаунта ---

func TestIntegration_TxManager_CommitsBothAggregates(t *testing.T) {
	pool := setup(t)
	orderRepo := NewOrderRepository(pool)
	accRepo := NewAccountRepository(pool, testCipher)
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

	orders, err := orderRepo.ListByCustomerEmail(ctx, "user@example.com", 20, 0)
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

// --- GetByInvoiceID ---

func TestIntegration_GetByInvoiceID(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	order := mustOrder(t, pool)
	got, err := repo.GetByInvoiceID(ctx, order.InvoiceID())
	if err != nil {
		t.Fatalf("GetByInvoiceID: %v", err)
	}
	if got.ID() != order.ID() {
		t.Errorf("GetByInvoiceID вернул неверный заказ: хотели %s, получили %s", order.ID(), got.ID())
	}

	_, err = repo.GetByInvoiceID(ctx, -1)
	if err != domain.ErrOrderNotFound {
		t.Errorf("ожидали ErrOrderNotFound для несуществующего invoiceID, получили %v", err)
	}
}

// --- GetByAccessToken ---

func TestIntegration_GetByAccessToken(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	order := mustOrder(t, pool)
	got, err := repo.GetByAccessToken(ctx, order.AccessToken())
	if err != nil {
		t.Fatalf("GetByAccessToken: %v", err)
	}
	if got.ID() != order.ID() {
		t.Errorf("GetByAccessToken вернул неверный заказ")
	}

	_, err = repo.GetByAccessToken(ctx, "nonexistent-token")
	if err != domain.ErrOrderNotFound {
		t.Errorf("ожидали ErrOrderNotFound для несуществующего токена, получили %v", err)
	}
}

// --- ListByCustomerPhone ---

func TestIntegration_ListByPhone(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	inv, _ := repo.NextInvoiceID(ctx)
	order, _ := domain.NewOrder(inv, "phone@example.com", "+79991234567", "Бриф по телефону", "", "", domain.CurrentConsentDocVersion, 150000)
	if err := repo.Create(ctx, order); err != nil {
		t.Fatalf("Create: %v", err)
	}

	orders, err := repo.ListByCustomerPhone(ctx, "+79991234567", 20, 0)
	if err != nil {
		t.Fatalf("ListByCustomerPhone: %v", err)
	}
	if len(orders) != 1 || orders[0].ID() != order.ID() {
		t.Errorf("ListByCustomerPhone: ожидали 1 заказ с правильным ID")
	}
}

// --- ListAll + CountAll ---

func TestIntegration_ListAll_CountAll(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	const n = 3
	for i := 0; i < n; i++ {
		mustOrder(t, pool)
	}

	total, err := repo.CountAll(ctx)
	if err != nil {
		t.Fatalf("CountAll: %v", err)
	}
	if total != n {
		t.Errorf("CountAll: ожидали %d, получили %d", n, total)
	}

	page, err := repo.ListAll(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListAll(limit=2): %v", err)
	}
	if len(page) != 2 {
		t.Errorf("ListAll(limit=2): ожидали 2 записи, получили %d", len(page))
	}

	page2, err := repo.ListAll(ctx, 10, 2)
	if err != nil {
		t.Fatalf("ListAll(offset=2): %v", err)
	}
	if len(page2) != n-2 {
		t.Errorf("ListAll(offset=2): ожидали %d записи, получили %d", n-2, len(page2))
	}
}

// --- ListStuckProcessing ---

func TestIntegration_ListStuckProcessing(t *testing.T) {
	pool := setup(t)
	orderRepo := NewOrderRepository(pool)
	ctx := context.Background()

	acc := mustAccount(t, pool, 10, 2)

	// Заказ 1: застрял в processing.
	processing := mustOrder(t, pool)
	_ = processing.MarkPaid()
	_ = processing.Enqueue()
	_ = processing.StartProcessing(acc.ID())
	if err := orderRepo.Update(ctx, processing); err != nil {
		t.Fatalf("Update processing order: %v", err)
	}

	// Заказ 2: ещё в queued — не должен попасть в результат.
	queued := mustOrder(t, pool)
	_ = queued.MarkPaid()
	_ = queued.Enqueue()
	if err := orderRepo.Update(ctx, queued); err != nil {
		t.Fatalf("Update queued order: %v", err)
	}

	// Порог в будущем: updated_at < now+1h — обязан вернуть processing-заказ.
	stuck, err := orderRepo.ListStuckProcessing(ctx, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("ListStuckProcessing: %v", err)
	}
	if len(stuck) != 1 {
		t.Fatalf("ожидали 1 застрявший заказ, получили %d", len(stuck))
	}
	if stuck[0].ID() != processing.ID() {
		t.Errorf("ожидали заказ %s, получили %s", processing.ID(), stuck[0].ID())
	}

	// Порог в прошлом: updated_at < now-1h — заказ слишком свежий, не должен попасть.
	fresh, err := orderRepo.ListStuckProcessing(ctx, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListStuckProcessing (прошлое): %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("порог в прошлом не должен возвращать свежие заказы, получили %d", len(fresh))
	}
}

// --- Account: List, ListByStatus, SetStatus ---

func TestIntegration_Account_List_SetStatus_ListByStatus(t *testing.T) {
	pool := setup(t)
	repo := NewAccountRepository(pool, testCipher)
	ctx := context.Background()

	acc1 := mustAccount(t, pool, 10, 1)
	acc2 := mustAccount(t, pool, 20, 2)

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List: ожидали 2 аккаунта, получили %d", len(all))
	}

	// Ban первого аккаунта через SetStatus.
	if err := repo.SetStatus(ctx, acc1.ID(), domain.AccountStatusBanned); err != nil {
		t.Fatalf("SetStatus banned: %v", err)
	}

	active, err := repo.ListByStatus(ctx, domain.AccountStatusActive)
	if err != nil {
		t.Fatalf("ListByStatus active: %v", err)
	}
	if len(active) != 1 || active[0].ID() != acc2.ID() {
		t.Errorf("после бана должен остаться 1 активный аккаунт — acc2")
	}

	banned, err := repo.ListByStatus(ctx, domain.AccountStatusBanned)
	if err != nil {
		t.Fatalf("ListByStatus banned: %v", err)
	}
	if len(banned) != 1 || banned[0].ID() != acc1.ID() {
		t.Errorf("после бана должен быть 1 забаненный аккаунт — acc1")
	}
}

// --- Account: Update (BanForInvalidSession round-trip) ---

func TestIntegration_Account_Update_RoundTrip(t *testing.T) {
	pool := setup(t)
	repo := NewAccountRepository(pool, testCipher)
	ctx := context.Background()

	acc := mustAccount(t, pool, 50, 2)

	// Симулируем регистрацию ошибки и обновление.
	acc.RegisterFailure(3)
	if err := repo.Update(ctx, acc); err != nil {
		t.Fatalf("Update после RegisterFailure: %v", err)
	}

	loaded, err := repo.GetByID(ctx, acc.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if loaded.FailureCount() != 1 {
		t.Errorf("ожидали failure_count=1, получили %d", loaded.FailureCount())
	}

	// BanForInvalidSession.
	loaded.BanForInvalidSession()
	if err := repo.Update(ctx, loaded); err != nil {
		t.Fatalf("Update после BanForInvalidSession: %v", err)
	}

	banned, err := repo.GetByID(ctx, acc.ID())
	if err != nil {
		t.Fatalf("GetByID после бана: %v", err)
	}
	if banned.Status() != domain.AccountStatusBanned {
		t.Errorf("ожидали status=banned, получили %q", banned.Status())
	}
}
