-- Таблица аккаунтов Suno
CREATE TABLE IF NOT EXISTS suno_accounts (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    encrypted_session TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    token_balance INT NOT NULL,
    failure_count INT NOT NULL,
    cooldown_until TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Шедевральный частичный индекс для FetchAndLockAvailable (SKIP LOCKED)
CREATE INDEX IF NOT EXISTS idx_suno_accounts_available 
ON suno_accounts (last_used_at ASC NULLS FIRST, created_at ASC)
WHERE status = 'active' AND token_balance > 0;

-- Таблица заказов
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    invoice_id BIGINT NOT NULL UNIQUE,
    customer_email VARCHAR(255),
    customer_phone VARCHAR(50),
    brief TEXT NOT NULL,
    amount_kopecks BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL,
    payment_status VARCHAR(50) NOT NULL,
    generation_status VARCHAR(50) NOT NULL,
    assigned_account_id UUID REFERENCES suno_accounts(id) ON DELETE SET NULL,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_orders_customer_email ON orders(customer_email);
CREATE INDEX IF NOT EXISTS idx_orders_invoice_id ON orders(invoice_id);

-- Таблица треков, жестко связанных с заказами (один к многим)
CREATE TABLE IF NOT EXISTS tracks (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    index INT NOT NULL,
    audio_url TEXT NOT NULL,
    duration_sec INT NOT NULL,
    suno_track_id VARCHAR(255) NOT NULL,
    UNIQUE(order_id, index)
);