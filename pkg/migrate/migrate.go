// Package migrate реализует простой версионный запуск SQL-миграций.
// Не требует внешних зависимостей: использует только database/sql и embed.
//
// Соглашение об именовании файлов миграций:
//
//	migrations/
//	  0001_init.sql
//	  0002_add_email_index.sql
//	  ...
//
// Каждый файл применяется ровно один раз: статус хранится в таблице schema_migrations.
// Миграции применяются в лексикографическом порядке имён файлов — поэтому важен
// числовой префикс с ведущими нулями.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     VARCHAR(255) PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

// migrationsLockID — произвольный стабильный bigint для pg_advisory_lock.
// Все инстансы приложения используют одно значение, поэтому Postgres
// выстраивает их в очередь: второй инстанс ждёт, пока первый не завершит
// все миграции и не освободит блокировку.
const migrationsLockID = 8723456789012345678

// migrationLockTimeout — сколько ждём advisory-lock миграций, прежде чем сдаться.
// Без таймаута осиротевший после жёсткого рестарта лок (Docker SIGKILL'ит контейнер
// до того, как Postgres снял сессионную блокировку) подвешивал бы старт НАВСЕГДА → 502.
// С таймаутом инстанс падает, Docker его перезапускает, к этому моменту лок уже снят —
// сервис чинится сам за полминуты вместо вечного зависания.
const migrationLockTimeout = 30 * time.Second

// lockRetryInterval — пауза между неблокирующими попытками захватить лок.
const lockRetryInterval = 1 * time.Second

// Run применяет все ещё не применённые SQL-файлы из migrationsFS.
// migrationsFS — это fs.FS, обычно go:embed директива в вызывающем пакете.
// Уже применённые версии пропускаются; порядок применения — лексикографический.
//
// Для защиты от гонки при одновременном старте нескольких инстансов функция
// захватывает pg_advisory_lock перед применением миграций и освобождает его
// по завершении. Блокировка сессионная: она автоматически снимается при закрытии
// соединения, поэтому утечка невозможна даже при панике.
//
// Захват лока — НЕблокирующий с таймаутом (migrationLockTimeout): если лок завис
// на осиротевшей сессии (жёсткий рестарт), Run не виснет навсегда, а возвращает
// ошибку — процесс завершается, Docker перезапускает контейнер, и к этому моменту
// Postgres уже снял мёртвую блокировку. Так startup чинится сам, а не лежит 502.
func Run(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS, log *slog.Logger) error {
	// Получаем выделенное соединение для сессионного advisory lock.
	// pool.Acquire возвращает соединение, которое мы явно освободим в defer —
	// это гарантирует снятие блокировки ровно по завершении Run.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for migration lock: %w", err)
	}
	defer conn.Release()

	if err := acquireMigrationLock(ctx, conn, log); err != nil {
		return err
	}
	log.Debug("advisory lock захвачен, применяем миграции")

	// 1. Создаём таблицу версий, если ещё нет.
	if _, err := conn.Exec(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// 2. Читаем уже применённые версии.
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}

	// 3. Собираем список SQL-файлов из embed.FS.
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// 4. Применяем каждый ещё не применённый файл в отдельной транзакции.
	for _, name := range files {
		if applied[name] {
			log.Debug("миграция уже применена, пропускаем", "version", name)
			continue
		}

		sql, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", name, err)
		}

		if err := applyMigration(ctx, conn, name, string(sql)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		log.Info("миграция применена", "version", name)
	}

	return nil
}

// acquireMigrationLock захватывает advisory-lock миграций неблокирующе
// (pg_try_advisory_lock) с повторами до migrationLockTimeout. По истечении дедлайна
// или при отмене ctx возвращает ошибку — вызывающий код завершается, контейнер
// перезапускается и пробует снова (к тому времени осиротевший лок уже снят).
func acquireMigrationLock(ctx context.Context, conn *pgxpool.Conn, log *slog.Logger) error {
	return acquireLockWithRetry(ctx, log, migrationLockTimeout, lockRetryInterval, func() (bool, error) {
		var got bool
		if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, migrationsLockID).Scan(&got); err != nil {
			return false, err
		}
		return got, nil
	})
}

// acquireLockWithRetry повторяет tryLock каждые retry, пока не получит лок или не
// истечёт timeout. Логика дедлайна/повторов вынесена сюда отдельно от SQL, чтобы
// её можно было протестировать без реального Postgres.
func acquireLockWithRetry(ctx context.Context, log *slog.Logger, timeout, retry time.Duration, tryLock func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		got, err := tryLock()
		if err != nil {
			return fmt.Errorf("try migration advisory lock: %w", err)
		}
		if got {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("не удалось захватить advisory-lock миграций за %s "+
				"(вероятно, его держит другой/зависший инстанс) — завершаемся, контейнер перезапустится", timeout)
		}
		log.Info("ждём advisory-lock миграций — его держит другой инстанс…")
		select {
		case <-time.After(retry):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, version, sql string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// Rollback после Commit — no-op (ErrTxClosed игнорируется pgx), поэтому ошибку не проверяем.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("exec sql: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
	); err != nil {
		return fmt.Errorf("record version: %w", err)
	}

	return tx.Commit(ctx)
}
