-- Фиксация согласия на обработку ПДн (152-ФЗ) при создании заказа.

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS consent_given_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS consent_doc_version VARCHAR(32);
