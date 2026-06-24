-- Отзывы о приложении. Оставляются публично, без регистрации; модерируются
-- администратором (ответ, скрытие, удаление). id генерирует приложение (uuid.New).
CREATE TABLE IF NOT EXISTS reviews (
    id             UUID PRIMARY KEY,
    author_name    TEXT NOT NULL,
    rating         SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body           TEXT NOT NULL,
    admin_reply    TEXT NOT NULL DEFAULT '',
    admin_reply_at TIMESTAMPTZ,
    is_published   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Публичный список отдаёт только опубликованные, в порядке свежести.
CREATE INDEX IF NOT EXISTS idx_reviews_published_created ON reviews (is_published, created_at DESC);
