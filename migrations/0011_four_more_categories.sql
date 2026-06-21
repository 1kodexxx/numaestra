-- Ещё 4 категории "на все случаи жизни" в дополнение к существующим.
-- Применяется ровно один раз (см. pkg/migrate); таблицы НЕ truncate-ятся.
-- cover_image_url = '' (не NULL): репозиторий сканирует поле в *string без COALESCE.

-- ==============================================================================
-- 1. КАТЕГОРИИ
-- ==============================================================================
INSERT INTO categories (id, title, description, cover_image_url, seo_tags, base_prompt_template) VALUES
(
    'valentine',
    'День святого Валентина',
    'Романтический трек-валентинка для любимого человека к 14 февраля.',
    '',
    '{"14 февраля", "валентинка", "романтика", "любовь"}',
    'Create a [MOOD] [GENRE] love song with [VOCAL]. The lyrics must be in Russian language. A Valentine song for [PARTNER] from [AUTHOR]. Why you love them: [WHY]. A shared romantic memory: [MEMORY].'
),
(
    'march8',
    '8 Марта',
    'Праздничное музыкальное поздравление для мамы, жены, подруги или коллег.',
    '',
    '{"8 марта", "международный женский день", "поздравление", "подарок"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. An International Women''s Day (March 8) greeting for [RECIPIENT]. Who she is to the author: [RELATION]. Wishes for her: [WISHES]. A compliment that describes her: [COMPLIMENT].'
),
(
    'promotion',
    'Повышение и новая работа',
    'Заряжающая песня-поздравление с карьерным успехом или новым местом работы.',
    '',
    '{"повышение", "новая работа", "карьера", "поздравление"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Congratulating [NAME] on a career milestone: [MILESTONE]. Their field or role: [ROLE]. Wishes for the new chapter: [WISHES].'
),
(
    'kids',
    'Детская песня',
    'Весёлая песенка специально про вашего ребёнка — для праздника или просто так.',
    '',
    '{"детская песня", "ребёнок", "детский праздник", "подарок"}',
    'Create a [MOOD] [GENRE] children song with [VOCAL]. The lyrics must be in Russian language, simple and cheerful for kids. About a child named [CHILD_NAME], age [AGE]. Favourite things and heroes: [FAVOURITES]. From whom: [FROM_WHO].'
);

-- ==============================================================================
-- 2. ВОПРОСЫ (QUESTIONS) — блоки ID 2700+, 2800+, 2900+, 3000+
-- ==============================================================================

-- ---> ДЕНЬ СВЯТОГО ВАЛЕНТИНА (ID 2700-2799)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2701, 'valentine', 1, 'Как зовут вашу половинку?', 'text', 'PARTNER', true),
(2702, 'valentine', 2, 'Как зовут вас?', 'text', 'AUTHOR', true),
(2703, 'valentine', 3, 'За что вы любите этого человека?', 'text', 'WHY', true),
(2704, 'valentine', 4, 'Романтическое воспоминание о вас двоих?', 'text', 'MEMORY', false),
(2705, 'valentine', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2706, 'valentine', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2707, 'valentine', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> 8 МАРТА (ID 2800-2899)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2801, 'march8', 1, 'Кого поздравляем (имя)?', 'text', 'RECIPIENT', true),
(2802, 'march8', 2, 'Кто она для вас?', 'text', 'RELATION', true),
(2803, 'march8', 3, 'Что хотите пожелать?', 'text', 'WISHES', true),
(2804, 'march8', 4, 'Комплимент, который её описывает?', 'text', 'COMPLIMENT', false),
(2805, 'march8', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2806, 'march8', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2807, 'march8', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПОВЫШЕНИЕ И НОВАЯ РАБОТА (ID 2900-2999)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2901, 'promotion', 1, 'Кого поздравляем (имя)?', 'text', 'NAME', true),
(2902, 'promotion', 2, 'С чем поздравляем (повышение, новая работа)?', 'text', 'MILESTONE', true),
(2903, 'promotion', 3, 'Сфера или должность?', 'text', 'ROLE', true),
(2904, 'promotion', 4, 'Что пожелать на новом этапе?', 'text', 'WISHES', true),
(2905, 'promotion', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2906, 'promotion', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2907, 'promotion', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ДЕТСКАЯ ПЕСНЯ (ID 3000-3099)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(3001, 'kids', 1, 'Имя ребёнка?', 'text', 'CHILD_NAME', true),
(3002, 'kids', 2, 'Сколько лет (можно пропустить)?', 'text', 'AGE', false),
(3003, 'kids', 3, 'Любимые занятия, игрушки, герои?', 'text', 'FAVOURITES', true),
(3004, 'kids', 4, 'От кого песня?', 'text', 'FROM_WHO', true),
(3005, 'kids', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(3006, 'kids', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(3007, 'kids', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ==============================================================================
-- 3. ВАРИАНТЫ ОТВЕТОВ (QUESTION OPTIONS)
-- ==============================================================================

-- День святого Валентина
INSERT INTO question_options (question_id, label, value) VALUES
(2705, 'Романтический поп', 'romantic pop'),
(2705, 'R&B / Соул', 'smooth r&b soul'),
(2705, 'Акустическая баллада', 'acoustic ballad'),
(2706, 'Страстное', 'passionate, intense'),
(2706, 'Нежное', 'tender, gentle'),
(2706, 'Мечтательное', 'dreamy, romantic'),
(2707, 'Мужской голос', 'male vocals'),
(2707, 'Женский голос', 'female vocals'),
(2707, 'Дуэт', 'male and female duet vocals');

-- 8 Марта
INSERT INTO question_options (question_id, label, value) VALUES
(2805, 'Поп', 'pop'),
(2805, 'Лирическая баллада', 'lyrical ballad'),
(2805, 'Романс', 'russian romance style'),
(2806, 'Тёплое', 'warm, loving'),
(2806, 'Праздничное', 'festive, celebratory'),
(2806, 'Нежное', 'tender, gentle'),
(2807, 'Мужской голос', 'male vocals'),
(2807, 'Женский голос', 'female vocals'),
(2807, 'Хор', 'group choir vocals');

-- Повышение и новая работа
INSERT INTO question_options (question_id, label, value) VALUES
(2905, 'Уверенный поп', 'corporate upbeat pop'),
(2905, 'Поп-рок', 'pop rock'),
(2905, 'Хип-хоп', 'energetic hip-hop, heavy bass'),
(2906, 'Триумфальное', 'triumphant, epic'),
(2906, 'Весёлое', 'fun, upbeat'),
(2906, 'Мотивирующее', 'motivational, inspiring'),
(2907, 'Мужской голос', 'male vocals'),
(2907, 'Женский голос', 'female vocals'),
(2907, 'Хор', 'group choir vocals');

-- Детская песня
INSERT INTO question_options (question_id, label, value) VALUES
(3005, 'Весёлый поп', 'fun upbeat children pop'),
(3005, 'Мультяшный', 'cartoon style, playful'),
(3005, 'Танцевальный', 'kids dance pop'),
(3006, 'Весёлое', 'fun, silly, playful'),
(3006, 'Доброе', 'sweet, gentle'),
(3006, 'Заводное', 'energetic, bouncy'),
(3007, 'Детский хор', 'children choir vocals'),
(3007, 'Женский голос', 'female vocals'),
(3007, 'Мужской голос', 'male vocals');

-- Восстанавливаем автоинкремент для вопросов
SELECT setval('questions_id_seq', (SELECT MAX(id) FROM questions));
