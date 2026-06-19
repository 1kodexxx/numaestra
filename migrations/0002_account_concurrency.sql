-- Конкурентные слоты для Suno-аккаунтов.
-- Раньше занятость аккаунта моделировалась статусом 'busy' (один аккаунт — одна
-- генерация одновременно), что искусственно ограничивало пропускную способность.
-- Платные аккаунты Suno (Pro/Premier) умеют вести несколько генераций сразу,
-- поэтому переходим на счётчик слотов: аккаунт доступен, пока
-- concurrent_tasks < max_concurrent_tasks.

ALTER TABLE suno_accounts
    ADD COLUMN IF NOT EXISTS max_concurrent_tasks INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS concurrent_tasks INT NOT NULL DEFAULT 0;

-- Сбрасываем легаси-статус 'busy' в 'active': теперь занятость считается
-- через concurrent_tasks, а строки в 'busy' иначе навсегда выпали бы из пула.
UPDATE suno_accounts SET status = 'active' WHERE status = 'busy';

-- Защита от рассогласования счётчика.
ALTER TABLE suno_accounts
    ADD CONSTRAINT chk_suno_accounts_concurrency
    CHECK (concurrent_tasks >= 0 AND concurrent_tasks <= max_concurrent_tasks);

-- Частичный индекс для быстрого поиска доступного аккаунта с учётом слотов.
CREATE INDEX IF NOT EXISTS idx_suno_accounts_available_slots
ON suno_accounts (concurrent_tasks ASC, last_used_at ASC NULLS FIRST, created_at ASC)
WHERE status = 'active' AND token_balance > 0;
