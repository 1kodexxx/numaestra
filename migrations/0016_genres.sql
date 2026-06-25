-- Справочник музыкальных жанров и привязка к категориям квиза.
-- Вопросы с option_source = 'genres' подтягивают варианты из genres + category_genres.

CREATE TABLE genres (
    id          SERIAL PRIMARY KEY,
    slug        VARCHAR(64)  NOT NULL UNIQUE,
    label       VARCHAR(100) NOT NULL,
    suno_value  VARCHAR(200) NOT NULL,
    sort_order  INT          NOT NULL DEFAULT 0,
    is_active   BOOLEAN      NOT NULL DEFAULT true
);

CREATE TABLE category_genres (
    category_id VARCHAR(50) NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    genre_id    INT         NOT NULL REFERENCES genres(id) ON DELETE CASCADE,
    sort_order  INT         NOT NULL DEFAULT 0,
    PRIMARY KEY (category_id, genre_id)
);

CREATE INDEX idx_category_genres_genre ON category_genres(genre_id);

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS option_source VARCHAR(32) NOT NULL DEFAULT 'inline',
    ADD COLUMN IF NOT EXISTS config JSONB NOT NULL DEFAULT '{}';

-- Жанры: единый каталог (расширяемый через админку).
INSERT INTO genres (slug, label, suno_value, sort_order) VALUES
    ('pop', 'Поп', 'modern pop', 10),
    ('pop_ballad', 'Поп-баллада', 'pop ballad', 20),
    ('ballad', 'Баллада', 'emotional ballad', 30),
    ('lyrical_ballad', 'Лирическая баллада', 'lyrical ballad', 40),
    ('romantic_ballad', 'Романтическая баллада', 'romantic ballad', 50),
    ('acoustic_ballad', 'Акустическая баллада', 'romantic acoustic ballad', 60),
    ('rock_ballad', 'Рок-баллада', 'rock ballad', 70),
    ('rock', 'Рок', 'rock', 80),
    ('pop_rock', 'Поп-рок', 'pop rock', 90),
    ('arena_rock', 'Арена-рок', 'epic arena rock', 100),
    ('indie', 'Инди', 'indie', 110),
    ('indie_pop', 'Инди-поп', 'indie pop', 120),
    ('folk', 'Фолк', 'folk', 130),
    ('acoustic', 'Акустика', 'acoustic guitar', 140),
    ('country', 'Кантри', 'modern country', 150),
    ('soul', 'Соул', 'smooth soul', 160),
    ('rnb', 'R&B', 'contemporary rnb', 170),
    ('jazz', 'Джаз', 'smooth jazz', 180),
    ('electronic', 'Электроника', 'electronic dance', 190),
    ('synthpop', 'Синти-поп', 'synth pop', 200),
    ('disco', 'Диско', 'disco funk', 210),
    ('hiphop', 'Хип-хоп', 'hip hop', 220),
    ('rap', 'Рэп', 'rap', 230),
    ('pop_punk', 'Поп-панк', 'pop punk', 240),
    ('punk_rock', 'Панк-рок', 'punk rock', 250),
    ('metal', 'Метал', 'heavy metal', 260),
    ('lullaby', 'Колыбельная', 'soft lullaby', 270),
    ('romance', 'Романс', 'russian romance style', 280),
    ('chanson', 'Шансон', 'russian chanson', 290),
    ('estrada', 'Эстрада', 'classic russian pop', 300),
    ('latin', 'Латино', 'latin pop', 310),
    ('reggae', 'Регги', 'reggae', 320),
    ('blues', 'Блюз', 'blues', 330),
    ('orchestral', 'Оркестровая', 'cinematic orchestral', 340),
    ('gospel', 'Госпел', 'gospel choir', 350);

-- Все категории по умолчанию получают полный каталог жанров.
INSERT INTO category_genres (category_id, genre_id, sort_order)
SELECT c.id, g.id, g.sort_order
FROM categories c
CROSS JOIN genres g
WHERE g.is_active = true
ON CONFLICT DO NOTHING;

-- GENRE-вопросы берут варианты из справочника, а не из question_options.
UPDATE questions
SET option_source = 'genres',
    config = '{"min_select": 1, "max_select": 3}'::jsonb
WHERE mapping_key = 'GENRE';
