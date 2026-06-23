package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/encryption"
)

type AccountRepository struct {
	pool   PgxPool
	cipher encryption.Cipher
}

func NewAccountRepository(pool PgxPool, cipher encryption.Cipher) *AccountRepository {
	return &AccountRepository{pool: pool, cipher: cipher}
}

func (r *AccountRepository) encryptSession(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	enc, err := r.cipher.Encrypt(plain)
	if err != nil {
		return "", fmt.Errorf("шифрование session: %w", err)
	}
	return enc, nil
}

func (r *AccountRepository) decryptSession(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plain, err := r.cipher.Decrypt(stored)
	if err != nil {
		return "", fmt.Errorf("расшифровка session: %w", err)
	}
	return plain, nil
}

// restoreAccount расшифровывает session в снэпшоте и восстанавливает доменный объект.
func (r *AccountRepository) restoreAccount(snap *domain.SunoAccountSnapshot) (*domain.SunoAccount, error) {
	plain, err := r.decryptSession(snap.EncryptedSession)
	if err != nil {
		return nil, err
	}
	snap.EncryptedSession = plain
	return domain.RestoreSunoAccount(*snap), nil
}

var _ domain.AccountRepository = (*AccountRepository)(nil)

// conn возвращает транзакцию из контекста (внутри Unit of Work) либо пул.
func (r *AccountRepository) conn(ctx context.Context) dbConn {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

func (r *AccountRepository) FetchAndLockAvailable(ctx context.Context) (*domain.SunoAccount, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback после Commit — no-op (ErrTxClosed игнорируется pgx), поэтому ошибку не проверяем.
	defer func() { _ = tx.Rollback(ctx) }()

	var snapshot domain.SunoAccountSnapshot

	// FOR UPDATE SKIP LOCKED: блокируем только выбранную строку, параллельные
	// горутины пропустят её и возьмут следующую. Доступность определяется не
	// статусом Busy, а свободным слотом concurrent_tasks < max_concurrent_tasks,
	// поэтому один платный аккаунт обслуживает несколько генераций параллельно.
	// Сортируем по загрузке (concurrent_tasks), чтобы равномерно распределять
	// нагрузку между аккаунтами пула.
	query := `
		SELECT id, email, encrypted_session, status, token_balance, failure_count, max_concurrent_tasks, concurrent_tasks, cooldown_until, last_used_at, created_at, updated_at
		FROM suno_accounts
		WHERE status = 'active'
		  AND token_balance > 0
		  AND concurrent_tasks < max_concurrent_tasks
		  AND (cooldown_until IS NULL OR cooldown_until < NOW() AT TIME ZONE 'UTC')
		ORDER BY concurrent_tasks ASC, last_used_at ASC NULLS FIRST, created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	err = tx.QueryRow(ctx, query).Scan(
		&snapshot.ID, &snapshot.Email, &snapshot.EncryptedSession, &snapshot.Status,
		&snapshot.TokenBalance, &snapshot.FailureCount, &snapshot.MaxConcurrentTasks, &snapshot.ConcurrentTasks,
		&snapshot.CooldownUntil, &snapshot.LastUsedAt, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoAvailableAccount
	}
	if err != nil {
		return nil, fmt.Errorf("select and lock available account: %w", err)
	}

	account, err := r.restoreAccount(&snapshot)
	if err != nil {
		return nil, fmt.Errorf("восстановление аккаунта после SELECT: %w", err)
	}

	if err := account.AcquireSlot(time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("domain acquire slot: %w", err)
	}

	snapUpdate := account.Snapshot()
	updateQuery := `UPDATE suno_accounts SET concurrent_tasks = $1, last_used_at = $2, updated_at = $3 WHERE id = $4`
	_, err = tx.Exec(ctx, updateQuery, snapUpdate.ConcurrentTasks, snapUpdate.LastUsedAt, snapUpdate.UpdatedAt, snapUpdate.ID)
	if err != nil {
		return nil, fmt.Errorf("update locked account slot: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return account, nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SunoAccount, error) {
	var snapshot domain.SunoAccountSnapshot
	query := `SELECT id, email, encrypted_session, status, token_balance, failure_count, max_concurrent_tasks, concurrent_tasks, cooldown_until, last_used_at, created_at, updated_at FROM suno_accounts WHERE id = $1`
	err := r.conn(ctx).QueryRow(ctx, query, id).Scan(
		&snapshot.ID, &snapshot.Email, &snapshot.EncryptedSession, &snapshot.Status,
		&snapshot.TokenBalance, &snapshot.FailureCount, &snapshot.MaxConcurrentTasks, &snapshot.ConcurrentTasks,
		&snapshot.CooldownUntil, &snapshot.LastUsedAt, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account by id: %w", err)
	}
	account, err := r.restoreAccount(&snapshot)
	if err != nil {
		return nil, fmt.Errorf("восстановление аккаунта %s: %w", id, err)
	}
	return account, nil
}

func (r *AccountRepository) Create(ctx context.Context, account *domain.SunoAccount) error {
	snap := account.Snapshot()
	encSession, err := r.encryptSession(snap.EncryptedSession)
	if err != nil {
		return fmt.Errorf("create suno account: %w", err)
	}
	query := `
		INSERT INTO suno_accounts (id, email, encrypted_session, status, token_balance, failure_count, max_concurrent_tasks, concurrent_tasks, cooldown_until, last_used_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = r.conn(ctx).Exec(ctx, query,
		snap.ID, snap.Email, encSession, snap.Status,
		snap.TokenBalance, snap.FailureCount, snap.MaxConcurrentTasks, snap.ConcurrentTasks,
		snap.CooldownUntil, snap.LastUsedAt, snap.CreatedAt, snap.UpdatedAt,
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
		SET status = $1, token_balance = $2, failure_count = $3, max_concurrent_tasks = $4, concurrent_tasks = $5, cooldown_until = $6, last_used_at = $7, updated_at = $8
		WHERE id = $9
	`
	cmd, err := r.conn(ctx).Exec(ctx, query, snap.Status, snap.TokenBalance, snap.FailureCount, snap.MaxConcurrentTasks, snap.ConcurrentTasks, snap.CooldownUntil, snap.LastUsedAt, snap.UpdatedAt, snap.ID)
	if err != nil {
		return fmt.Errorf("update suno account: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrAccountNotFound
	}
	return nil
}

func (r *AccountRepository) ListByStatus(ctx context.Context, status domain.AccountStatus) ([]*domain.SunoAccount, error) {
	query := `SELECT id, email, encrypted_session, status, token_balance, failure_count, max_concurrent_tasks, concurrent_tasks, cooldown_until, last_used_at, created_at, updated_at FROM suno_accounts WHERE status = $1`
	rows, err := r.conn(ctx).Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("list accounts by status: %w", err)
	}
	defer rows.Close()

	var accounts []*domain.SunoAccount
	for rows.Next() {
		var snapshot domain.SunoAccountSnapshot
		err := rows.Scan(
			&snapshot.ID, &snapshot.Email, &snapshot.EncryptedSession, &snapshot.Status,
			&snapshot.TokenBalance, &snapshot.FailureCount, &snapshot.MaxConcurrentTasks, &snapshot.ConcurrentTasks,
			&snapshot.CooldownUntil, &snapshot.LastUsedAt, &snapshot.CreatedAt, &snapshot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan account row: %w", err)
		}
		acc, err := r.restoreAccount(&snapshot)
		if err != nil {
			return nil, fmt.Errorf("восстановление аккаунта %s: %w", snapshot.ID, err)
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

// List возвращает все аккаунты пула без фильтрации (Admin API).
func (r *AccountRepository) List(ctx context.Context) ([]*domain.SunoAccount, error) {
	query := `SELECT id, email, encrypted_session, status, token_balance, failure_count, max_concurrent_tasks, concurrent_tasks, cooldown_until, last_used_at, created_at, updated_at FROM suno_accounts ORDER BY created_at DESC`
	rows, err := r.conn(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*domain.SunoAccount
	for rows.Next() {
		var snapshot domain.SunoAccountSnapshot
		if err := rows.Scan(
			&snapshot.ID, &snapshot.Email, &snapshot.EncryptedSession, &snapshot.Status,
			&snapshot.TokenBalance, &snapshot.FailureCount, &snapshot.MaxConcurrentTasks, &snapshot.ConcurrentTasks,
			&snapshot.CooldownUntil, &snapshot.LastUsedAt, &snapshot.CreatedAt, &snapshot.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account row: %w", err)
		}
		acc, err := r.restoreAccount(&snapshot)
		if err != nil {
			return nil, fmt.Errorf("восстановление аккаунта %s: %w", snapshot.ID, err)
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

// SetStatus атомарно меняет статус аккаунта (Admin API).
func (r *AccountRepository) SetStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
	now := time.Now().UTC()
	query := `UPDATE suno_accounts SET status = $1, updated_at = $2 WHERE id = $3`
	cmd, err := r.conn(ctx).Exec(ctx, query, status, now, id)
	if err != nil {
		return fmt.Errorf("set account status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrAccountNotFound
	}
	return nil
}
