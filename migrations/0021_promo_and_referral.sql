-- Промокоды: скидка в процентах или фиксированной сумме (в рублях).
CREATE TABLE IF NOT EXISTS promo_codes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code           VARCHAR(50)  UNIQUE NOT NULL,
    discount_type  VARCHAR(20)  NOT NULL CHECK (discount_type IN ('percent', 'fixed_rub')),
    discount_value INT          NOT NULL CHECK (discount_value > 0),
    max_uses       INT,                          -- NULL = без лимита
    current_uses   INT          NOT NULL DEFAULT 0,
    valid_from     TIMESTAMPTZ,
    valid_until    TIMESTAMPTZ,
    is_active      BOOLEAN      NOT NULL DEFAULT true,
    description    TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS promo_codes_code_idx ON promo_codes (code);

-- Добавляем поля промокода и реферала к заказам.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS promo_code_id           UUID REFERENCES promo_codes(id),
    ADD COLUMN IF NOT EXISTS original_amount_kopecks BIGINT,
    ADD COLUMN IF NOT EXISTS discount_kopecks        BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS referral_code           VARCHAR(50);
