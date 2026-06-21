-- Обратная связь администратора по заказу (письмо клиенту + история в БД).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS admin_feedback TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS admin_feedback_at TIMESTAMPTZ;
