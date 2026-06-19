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
