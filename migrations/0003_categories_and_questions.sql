-- Таблица категорий (для карточек на главной)
CREATE TABLE categories (
    id VARCHAR(50) PRIMARY KEY,
    title VARCHAR(100) NOT NULL,
    description TEXT,
    cover_image_url VARCHAR(255),
    seo_tags TEXT[],
    base_prompt_template TEXT NOT NULL -- Скрытый от юзера шаблон. Пример: "Create a [MOOD] [GENRE] song about [TOPIC]."
);

-- Таблица вопросов (шаги для квиза или подсказки)
CREATE TABLE questions (
    id SERIAL PRIMARY KEY,
    category_id VARCHAR(50) REFERENCES categories(id) ON DELETE CASCADE,
    step_number INT NOT NULL,
    question_text VARCHAR(255) NOT NULL,
    ui_type VARCHAR(50) NOT NULL, -- 'text', 'tags', 'radio'
    mapping_key VARCHAR(50) NOT NULL, -- Ключ переменной (например, 'GENRE', 'TOPIC')
    is_required BOOLEAN DEFAULT true
);

-- Варианты ответов для карточек-подсказок (если ui_type = 'tags' или 'radio')
CREATE TABLE question_options (
    id SERIAL PRIMARY KEY,
    question_id INT REFERENCES questions(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL, -- Что видит юзер ("Dark Trap")
    value VARCHAR(100) NOT NULL  -- Что идет в промпт ("dark trap")
);