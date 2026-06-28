-- Разовая миграция публичных ссылок на CDN (Yandex CDN перед Object Storage).
--
-- НЕ авто-миграция: домены зависят от окружения, поэтому запускается ВРУЧНУЮ один
-- раз после подключения CDN. Переписывает домен в уже сохранённых ссылках, чтобы
-- старые треки/демо тоже отдавались через CDN. Безопасно: оба домена (S3 и CDN)
-- обслуживают одни и те же объекты — даже до миграции ничего не ломается.
--
-- Подставьте свои значения:
--   :old_base — текущая база ссылок в БД: {S3_ENDPOINT}/{S3_BUCKET}
--               напр. https://storage.yandexcloud.net/numaestra-tracks
--   :new_base — значение S3_PUBLIC_BASE_URL (CDN), напр. https://cdn.numaestra.ru
--
-- Запуск (значения передаются как psql-переменные, без завершающего слэша):
--   psql "$DATABASE_URL" \
--     -v old_base='https://storage.yandexcloud.net/numaestra-tracks' \
--     -v new_base='https://cdn.numaestra.ru' \
--     -f deploy/cdn-migrate-urls.sql
--
-- Идемпотентно: повторный запуск ничего не трогает (фильтр LIKE по old_base).
-- Перед запуском сделайте бэкап (deploy/backup-postgres.sh).

BEGIN;

-- Готовые треки (отдельная таблица).
UPDATE tracks
SET audio_url = replace(audio_url, :'old_base', :'new_base')
WHERE audio_url LIKE :'old_base' || '%';

-- Витринное превью демо.
UPDATE orders
SET demo_url = replace(demo_url, :'old_base', :'new_base')
WHERE demo_url LIKE :'old_base' || '%';

-- Полные демо-клипы лежат в JSONB-массиве. Переписываем домен в текстовом
-- представлении и кастуем обратно — домен встречается только в audio_url-строках.
UPDATE orders
SET demo_clips = replace(demo_clips::text, :'old_base', :'new_base')::jsonb
WHERE demo_clips IS NOT NULL
  AND demo_clips::text LIKE '%' || :'old_base' || '%';

-- Примеры готовых работ (аудио и обложки, загруженные в тот же бакет).
UPDATE examples
SET audio_url = replace(audio_url, :'old_base', :'new_base')
WHERE audio_url LIKE :'old_base' || '%';

UPDATE examples
SET cover_url = replace(cover_url, :'old_base', :'new_base')
WHERE cover_url LIKE :'old_base' || '%';

COMMIT;
