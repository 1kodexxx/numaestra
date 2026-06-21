-- Исправляем пути к обложкам: используем SVG-заглушки, встроенные в бинарник.
UPDATE categories SET cover_image_url = '/images/covers/wedding.jpg'   WHERE id = 'wedding'   AND cover_image_url = '/images/covers/wedding.jpg';
UPDATE categories SET cover_image_url = '/images/covers/wedding.svg'   WHERE id = 'wedding';
UPDATE categories SET cover_image_url = '/images/covers/corporate.svg' WHERE id = 'corporate';
UPDATE categories SET cover_image_url = '/images/covers/boss.svg'      WHERE id = 'boss';
UPDATE categories SET cover_image_url = '/images/covers/birthday.svg'  WHERE id = 'birthday';

-- Добавляем поле plan к заказам (standard / premium).
-- DEFAULT 'standard' покрывает все исторические строки.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS plan VARCHAR(20) NOT NULL DEFAULT 'standard';
