-- Публичная share-ссылка (/orders/{id}/share): владелец может отозвать доступ.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS share_revoked_at TIMESTAMPTZ;
