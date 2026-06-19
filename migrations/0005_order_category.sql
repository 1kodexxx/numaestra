-- Привязка заказа к категории квиза и кеш готового Suno-промпта.
-- category_id — опциональная ссылка: заказы, созданные до внедрения квиза
-- (или через "свободный" бриф без категории), имеют NULL.
-- suno_prompt — итоговый промпт после подстановки ответов квиза в шаблон.
-- Если NULL, воркер использует сырой brief как прежде (обратная совместимость).

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS category_id VARCHAR(50) REFERENCES categories(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS suno_prompt TEXT;
