-- Демо-режим: бесплатный короткий фрагмент до оплаты.
-- Полностью аддитивно и изолированно от платёжного пути: две nullable-колонки,
-- которые пишет ТОЛЬКО демо-поток (UpdateDemo), и не трогает обычный Update заказа.
-- demo_status: none → processing → ready | failed.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS demo_status     VARCHAR(20) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS demo_url        TEXT,
    -- Аккаунт, чей слот захвачен под демо. Хранится в БД (а не только в payload
    -- задачи), чтобы recovery-крон мог освободить слот при краше воркера и не дать
    -- демо навсегда отъесть ёмкость у платных генераций.
    ADD COLUMN IF NOT EXISTS demo_account_id UUID REFERENCES suno_accounts(id);

-- Частичный индекс для фоновой диагностики «застрявших» демо (processing).
CREATE INDEX IF NOT EXISTS orders_demo_processing_idx
    ON orders (updated_at)
    WHERE demo_status = 'processing';
