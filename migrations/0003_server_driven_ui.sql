-- Таблица категорий
CREATE TABLE categories (
    id VARCHAR(50) PRIMARY KEY,
    title VARCHAR(100) NOT NULL,
    description TEXT,
    cover_image_url VARCHAR(255),
    seo_tags TEXT[],
    base_prompt_template TEXT NOT NULL
);

-- Таблица вопросов для квиза
CREATE TABLE questions (
    id SERIAL PRIMARY KEY,
    category_id VARCHAR(50) REFERENCES categories(id) ON DELETE CASCADE,
    step_number INT NOT NULL,
    question_text VARCHAR(255) NOT NULL,
    ui_type VARCHAR(50) NOT NULL,
    mapping_key VARCHAR(50) NOT NULL,
    is_required BOOLEAN DEFAULT true
);

-- Таблица вариантов ответов (для кнопок/тегов)
CREATE TABLE question_options (
    id SERIAL PRIMARY KEY,
    question_id INT REFERENCES questions(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL,
    value VARCHAR(100) NOT NULL
);

-- ==========================================
-- ТЕСТОВЫЕ ДАННЫЕ (SEED)
-- ==========================================
INSERT INTO categories (id, title, description, cover_image_url, seo_tags, base_prompt_template) 
VALUES (
    'wedding', 
    'Песня на свадьбу', 
    'Создайте идеальный саундтрек для вашего главного дня.', 
    '/images/wedding-cover.jpg', 
    '{"свадьба", "подарок", "романтика"}', 
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. The song is about: Жениха зовут [GROOM], невесту [BRIDE]. Они познакомились: [MEET_STORY]. Главное обещание: [PROMISE].'
);

INSERT INTO questions (category_id, step_number, question_text, ui_type, mapping_key) VALUES
('wedding', 1, 'Как зовут жениха?', 'text', 'GROOM'),
('wedding', 2, 'Как зовут невесту?', 'text', 'BRIDE'),
('wedding', 3, 'Где вы познакомились?', 'text', 'MEET_STORY'),
('wedding', 4, 'Что самое главное вы хотите пообещать друг другу?', 'text', 'PROMISE'),
('wedding', 5, 'В каком жанре будет песня?', 'tags', 'GENRE'),
('wedding', 6, 'Какое настроение передаем?', 'tags', 'MOOD'),
('wedding', 7, 'Какой вокал предпочитаете?', 'radio', 'VOCAL');

-- Добавляем варианты ответов только для вопросов с тегами (id 5, 6, 7)
-- Внимание: ID могут отличаться, если вы добавляли что-то до этого. Предполагаем, что жанр это id=5
INSERT INTO question_options (question_id, label, value) VALUES
(5, 'Поп-музыка', 'pop'),
(5, 'Современный Рэп / Trap', 'dark trap'),
(5, 'Романтический Акустик', 'acoustic pop'),
(6, 'Трогательное', 'emotional, touching'),
(6, 'Веселое и танцевальное', 'upbeat, dance'),
(7, 'Мужской', 'male vocals'),
(7, 'Женский', 'female vocals'),
(7, 'Дуэт', 'male and female duet');