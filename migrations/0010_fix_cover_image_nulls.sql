-- 0009_more_categories.sql изначально записывал NULL в cover_image_url для новых
-- категорий, а CategoryRepository.List() сканирует это поле в *string без COALESCE —
-- запрос падал с "cannot scan NULL into *string". Сама 0009 уже исправлена на
-- пустую строку для свежих баз, но эта миграция чинит базы, где 0009 уже применилась.
UPDATE categories SET cover_image_url = '' WHERE cover_image_url IS NULL;
