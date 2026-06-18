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

type AccountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

var _ domain.AccountRepository = (*AccountRepository)(nil)

func (r *AccountRepository) FetchAndLockAvailable(ctx context.Context) (*domain.SunoAccount, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var snapshot domain.SunoAccountSnapshot
	
	// Используем FOR UPDATE SKIP LOCKED: блокируем только ту строку, которую берем, 
	// параллельные горутины пропустят её и возьмут следующие. Полный параллелизм без взаимных блокировок!
	query := `
		SELECT id, email, encrypted_session, status, token_balance, failure_count, cooldown_until, last_used_at, created_at, updated_at
		FROM suno_accounts
		WHERE status = 'active'
		  AND token_balance > 0
		  AND (cooldown_until IS NULL OR cooldown_until < NOW() AT TIME ZONE 'UTC')
		ORDER BY last_used_at ASC NULLS FIRST, created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	err = tx.QueryRow(ctx, query).Scan(
		&snapshot.ID, &snapshot.Email, &snapshot.EncryptedSession, &snapshot.Status,
		&snapshot.TokenBalance, &snapshot.FailureCount, &snapshot.CooldownUntil,
		&snapshot.LastUsedAt, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoAvailableAccount
	}
	if err != nil {
		return nil, fmt.Errorf("select and lock available account: %w", err)
	}

	// Переводим в домене в статус Busy
	account := domain.RestoreSunoAccount(snapshot)
	if err := account.MarkBusy(); err != nil {
		return nil, fmt.Errorf("domain transition to busy: %w", err)
	}

	// Фиксируем изменение статуса в рамках той же транзакции
	snapUpdate := account.Snapshot()
	updateQuery := `UPDATE suno_accounts SET status = $1, updated_at = $2 WHERE id = $3`
	_, err = tx.Exec(ctx, updateQuery, snapUpdate.Status, snapUpdate.UpdatedAt, snapUpdate.ID)
	if err != nil {
		return nil, fmt.Errorf("update locked account state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return account, nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SunoAccount, error) {
	var snapshot domain.SunoAccountSnapshot
	query := `SELECT id, email, encrypted_session, status, token_balance, failure_count, cooldown_until, last_used_at, created_at, updated_at FROM suno_accounts WHERE id = $1`
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&snapshot.ID, &snapshot.Email, &snapshot.EncryptedSession, &snapshot.Status,
		&snapshot.TokenBalance, &snapshot.FailureCount, &snapshot.CooldownUntil,
		&snapshot.LastUsedAt, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account by id: %w", err)
	}
	return domain.RestoreSunoAccount(snapshot), nil
}

func (r *AccountRepository) Create(ctx context.Context, account *domain.SunoAccount) error {
	snap := account.Snapshot()
	query := `
		INSERT INTO suno_accounts (id, email, encrypted_session, status, token_balance, failure_count, cooldown_until, last_used_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		snap.ID, snap.Email, snap.EncryptedSession, snap.Status,
		snap.TokenBalance, snap.FailureCount, snap.CooldownUntil,
		snap.LastUsedAt, snap.CreatedAt, snap.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create suno account: %w", err)
	}
	return nil
}

func (r *AccountRepository) Update(ctx context.Context, account *domain.SunoAccount) error {
	snap := account.Snapshot()
	query := `
		UPDATE suno_accounts
		SET status = $1, token_balance = $2, failure_count = $3, cooldown_until = $4, last_used_at = $5, updated_at = $6
		WHERE id = $7
	`
	cmd, err := r.pool.Exec(ctx, query, snap.Status, snap.TokenBalance, snap.FailureCount, snap.CooldownUntil, snap.LastUsedAt, snap.UpdatedAt, snap.ID)
	if err != nil {
		return fmt.Errorf("update suno account: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrAccountNotFound
	}
	return nil
}

func (r *AccountRepository) ListByStatus(ctx context.Context, status domain.AccountStatus) ([]*domain.SunoAccount, error) {
	query := `SELECT id, email, encrypted_session, status, token_balance, failure_count, cooldown_until, last_used_at, created_at, updated_at FROM suno_accounts WHERE status = $1`
	rows, err := r.pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("list accounts by status: %w", err)
	}
	defer rows.Close()

	var accounts []*domain.SunoAccount
	for rows.Next() {
		var snapshot domain.SunoAccountSnapshot
		err := rows.Scan(
			&snapshot.ID, &snapshot.Email, &snapshot.EncryptedSession, &snapshot.Status,
			&snapshot.TokenBalance, &snapshot.FailureCount, &snapshot.CooldownUntil,
			&snapshot.LastUsedAt, &snapshot.CreatedAt, &snapshot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan account row: %w", err)
		}
		accounts = append(accounts, domain.RestoreSunoAccount(snapshot))
	}
	return accounts, nil
}