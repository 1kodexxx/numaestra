-- Детальный прогресс генерации для страницы статуса заказа.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS generation_phase   VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS generation_progress SMALLINT   NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tracks_ready       SMALLINT   NOT NULL DEFAULT 0;

UPDATE orders
SET generation_phase = 'completed',
    generation_progress = 100,
    tracks_ready = (SELECT COUNT(*)::int FROM tracks t WHERE t.order_id = orders.id)
WHERE generation_status = 'completed';

UPDATE orders
SET generation_phase = 'queued',
    generation_progress = 3
WHERE generation_status = 'queued' AND payment_status = 'paid';

UPDATE orders
SET generation_phase = 'generating',
    generation_progress = 35
WHERE generation_status = 'processing';
