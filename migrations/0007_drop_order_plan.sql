-- Продукт упрощён до фиксированной цены: 4 версии песни за один платёж,
-- без тарифов/подписок (см. 0006_cover_images_and_plan.sql, где колонка была добавлена).
ALTER TABLE orders DROP COLUMN IF EXISTS plan;
