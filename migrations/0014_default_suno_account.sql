-- Служебный Suno-аккаунт в пуле, чтобы генерация работала «из коробки».
--
-- С провайдером Sunor.cc реальная авторизация идёт по общему SUNO_API_KEY, а пул
-- suno_accounts выполняет роль ограничителя конкуренции. Без хотя бы одного
-- аккаунта воркер не может «захватить слот» и заказ падает с «нет свободных
-- аккаунтов». Этот аккаунт снимает требование вручную добавлять аккаунт.
--
-- encrypted_session пустой: репозиторий не пытается расшифровать пустую строку,
-- а для Sunor сессия аккаунта в HTTP-запросе всё равно не используется.
-- Вставляем ТОЛЬКО если пул пуст — чтобы не мешать реальным аккаунтам админа.
INSERT INTO suno_accounts (
    id, email, encrypted_session, status, token_balance, failure_count,
    max_concurrent_tasks, concurrent_tasks, cooldown_until, last_used_at, created_at, updated_at
)
SELECT
    gen_random_uuid(), 'default@sunor.pool', '', 'active', 1000000, 0,
    5, 0, NULL, NULL, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM suno_accounts);
