-- Примеры готовых работ ("Послушать примеры" на главной). Раньше были захардкожены
-- во фронтенде (shared/data/examples.ts) — теперь управляются через админку.
CREATE TABLE IF NOT EXISTS examples (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    mood        TEXT NOT NULL DEFAULT '',
    audio_url   TEXT NOT NULL DEFAULT '',
    cover_url   TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_examples_active_sort ON examples (is_active, sort_order, id);

-- Сидим текущие 8 примеров, чтобы после миграции главная страница не опустела.
-- ON CONFLICT DO NOTHING: повторный прогон не перетирает правки администратора.
INSERT INTO examples (id, title, category, description, mood, audio_url, cover_url, sort_order) VALUES
('coral-wedding',     'Коралловая свадьба',        'Годовщина', 'Тёплая песня к юбилею свадьбы — о прожитых вместе годах, верности и любви, которая только крепнет со временем.', 'Торжество', '/examples/wedding.mp3',            '/examples/wedding.webp',            1),
('volodya-praskovya', 'Володя и Прасковья',        'Любовь',    'Лиричная история пары — Володи и Прасковьи. Имена и судьбы героев вплетены прямо в текст песни.',               'Романтика', '/examples/volodya-praskovya.mp3',  '/examples/volodya-praskovya.webp',  2),
('family-circle',     'Звенит семейный круг',      'Семья',     'Душевный гимн большой семьи — о тепле родного дома, поколениях и связи, что объединяет всех за одним столом.',  'Тепло',     '/examples/family-circle.mp3',      '/examples/family-circle.webp',      3),
('jubilee-smirnov',   'На юбилей Жени Смирнову',    'Юбилей',    'Персональное поздравление к юбилею — с именем виновника торжества и тёплыми словами от близких.',               'Праздник',  '/examples/jubilee-smirnov.mp3',    '/examples/jubilee-smirnov.webp',    4),
('jubilee-zhenya',    'С юбилеем, Женя',           'Юбилей',    'Праздничная песня-поздравление: искренние пожелания и добрый юмор для дорогого человека в его особенный день.', 'Праздник',  '/examples/jubilee-zhenya.mp3',     '/examples/jubilee-zhenya.webp',     5),
('jubilee',           'Юбилей',                    'Юбилей',    'Торжественный трек к круглой дате — о пройденном пути, достижениях и наступающем новом этапе жизни.',           'Торжество', '/examples/jubilee.mp3',            '/examples/jubilee.webp',            6),
('tatyana',           'Колесниковой Татьяне',      'Подарок',   'Именная песня в подарок Татьяне — нежное посвящение, созданное специально для одного-единственного человека.',  'Тепло',     '/examples/tatyana.mp3',            '/examples/tatyana.webp',            7),
('big-house',         'Наш большой дом',           'Новоселье', 'Светлая песня о родном доме и уюте — отличный подарок на новоселье и для всей семьи.',                          'Тепло',     '/examples/big-house.mp3',          '/examples/big-house.webp',          8)
ON CONFLICT (id) DO NOTHING;
