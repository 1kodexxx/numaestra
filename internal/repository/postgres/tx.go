package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// dbConn — минимальный интерфейс исполнителя запросов, который одинаково
// реализуют и *pgxpool.Pool, и pgx.Tx. Репозитории работают через него, поэтому
// один и тот же код выполняется как вне транзакции (на пуле), так и внутри
// транзакции Unit of Work.
type dbConn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// txCtxKey — приватный ключ для переноса активной транзакции через context.
// Транзакция кладётся в контекст в TxManager.Do и подхватывается репозиториями.
type txCtxKey struct{}

func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txCtxKey{}).(pgx.Tx)
	return tx, ok
}

// nullableString конвертирует пустую строку в nil для корректной записи
// в nullable TEXT/VARCHAR колонки PostgreSQL. Непустая строка возвращается как есть.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
} // открывает одну транзакцию БД и
// прокидывает её через context во все репозитории, вызванные внутри fn. Это
// позволяет UseCase атомарно изменять несколько независимых агрегатов (Order и
// SunoAccount), не давая репозиторию одного агрегата знать о таблицах другого.
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// Do выполняет fn в рамках единой транзакции. Если в контексте уже есть активная
// транзакция (вложенный вызов), переиспользует её, не открывая новую. При ошибке
// fn транзакция откатывается, иначе — фиксируется.
func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		// Уже внутри транзакции — просто выполняем, фиксацией управляет внешний Do.
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin unit of work: %w", err)
	}
	defer func() {
		// Rollback после Commit — это no-op (ErrTxClosed игнорируется pgx),
		// поэтому defer безопасен и страхует от паники внутри fn.
		_ = tx.Rollback(ctx)
	}()

	if err := fn(withTx(ctx, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit unit of work: %w", err)
	}
	return nil
}

// runAtomic выполняет fn в транзакции: переиспует транзакцию из контекста
// (Unit of Work) либо открывает собственную поверх пула, если её нет. Нужна
// методам, которые делают несколько записей и должны быть атомарны даже при
// одиночном вызове (например, заказ + его треки).
func runAtomic(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context, db dbConn) error) error {
	if tx, ok := txFromContext(ctx); ok {
		return fn(ctx, tx)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(withTx(ctx, tx), tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
