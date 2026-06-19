-- Очищаем таблицы перед заливкой, чтобы можно было перезапускать этот файл много раз без дублей
TRUNCATE TABLE question_options, questions, categories RESTART IDENTITY CASCADE;

-- ==============================================================================
-- 1. КАТЕГОРИИ (CATEGORIES)
-- ==============================================================================
INSERT INTO categories (id, title, description, cover_image_url, seo_tags, base_prompt_template) VALUES 
(
    'wedding', 
    'Песня на свадьбу', 
    'Создайте идеальный саундтрек для вашего главного дня. Трогательно, нежно или с драйвом!', 
    '/images/covers/wedding.jpg', 
    '{"свадьба", "подарок", "романтика", "первый танец"}', 
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. The song is about: Жениха зовут [GROOM], невесту [BRIDE]. Они познакомились: [MEET_STORY]. Главное обещание друг другу: [PROMISE].'
),
(
    'corporate', 
    'Гимн компании / Корпоратив', 
    'Мощный трек, который сплотит команду и подчеркнет статус вашего бизнеса.', 
    '/images/covers/corporate.jpg', 
    '{"бизнес", "корпоратив", "гимн", "команда"}', 
    'Create a [MOOD] [GENRE] corporate anthem with [VOCAL]. The lyrics must be in Russian language. The song is about a company named [COMPANY_NAME]. Industry: [INDUSTRY]. Core values: [VALUES]. Special shoutouts to: [SHOUTOUTS].'
),
(
    'boss', 
    'Подарок начальнику', 
    'Песня-поздравление или рэп-прожарка для вашего любимого руководителя.', 
    '/images/covers/boss.jpg', 
    '{"начальник", "босс", "юбилей", "коллеги"}', 
    'Create a [MOOD] [GENRE] song with [VOCAL]. Lyrics in Russian. The song is a gift for a boss named [BOSS_NAME]. Their domain/department: [ROLE]. Funny habit or catchphrase: [HABIT]. The team wishes them: [WISHES].'
),
(
    'birthday', 
    'С Днем Рождения', 
    'Персональный хит для именинника, который он запомнит навсегда.', 
    '/images/covers/birthday.jpg', 
    '{"день рождения", "подарок", "праздник", "юбилей"}', 
    'Create a [MOOD] [GENRE] birthday song with [VOCAL]. Lyrics in Russian. Birthday person is [NAME]. Age (optional): [AGE]. Hobbies and funny facts: [HOBBIES]. This gift is from: [FROM_WHO].'
);

-- ==============================================================================
-- 2. ВОПРОСЫ (QUESTIONS)
-- Используем ручное назначение ID (100+, 200+), чтобы легко привязывать Options
-- ==============================================================================

-- ---> СВАДЬБА (ID 100-199)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(101, 'wedding', 1, 'Как зовут жениха?', 'text', 'GROOM', true),
(102, 'wedding', 2, 'Как зовут невесту?', 'text', 'BRIDE', true),
(103, 'wedding', 3, 'Где вы познакомились или как это было?', 'text', 'MEET_STORY', false),
(104, 'wedding', 4, 'Что самое главное вы хотите пообещать друг другу?', 'text', 'PROMISE', true),
(105, 'wedding', 5, 'В каком жанре будет песня?', 'tags', 'GENRE', true),
(106, 'wedding', 6, 'Какое настроение передаем?', 'tags', 'MOOD', true),
(107, 'wedding', 7, 'Какой вокал предпочитаете?', 'radio', 'VOCAL', true);

-- ---> КОРПОРАТИВ (ID 200-299)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(201, 'corporate', 1, 'Как называется ваша компания?', 'text', 'COMPANY_NAME', true),
(202, 'corporate', 2, 'В какой сфере вы работаете (что делаете)?', 'text', 'INDUSTRY', true),
(203, 'corporate', 3, 'Главная ценность или фишка вашей команды?', 'text', 'VALUES', false),
(204, 'corporate', 4, 'Кого из отделов или людей обязательно упомянуть?', 'text', 'SHOUTOUTS', false),
(205, 'corporate', 5, 'В каком стиле напишем гимн?', 'tags', 'GENRE', true),
(206, 'corporate', 6, 'Какая будет атмосфера?', 'tags', 'MOOD', true),
(207, 'corporate', 7, 'Кто исполняет?', 'radio', 'VOCAL', true);

-- ---> БОСС / НАЧАЛЬНИК (ID 300-399)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(301, 'boss', 1, 'Как зовут начальника (имя и отчество)?', 'text', 'BOSS_NAME', true),
(302, 'boss', 2, 'Чем он руководит (название отдела, компании)?', 'text', 'ROLE', true),
(303, 'boss', 3, 'Его любимая фраза или забавная привычка?', 'text', 'HABIT', false),
(304, 'boss', 4, 'Что коллектив хочет ему пожелать?', 'text', 'WISHES', true),
(305, 'boss', 5, 'Музыкальный жанр', 'tags', 'GENRE', true),
(306, 'boss', 6, 'Настроение трека', 'tags', 'MOOD', true),
(307, 'boss', 7, 'Голос', 'radio', 'VOCAL', true);

-- ---> ДЕНЬ РОЖДЕНИЯ (ID 400-499)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(401, 'birthday', 1, 'Имя именинника?', 'text', 'NAME', true),
(402, 'birthday', 2, 'Сколько лет исполняется (можно пропустить)?', 'text', 'AGE', false),
(403, 'birthday', 3, 'Его хобби, увлечения или забавные факты?', 'text', 'HOBBIES', true),
(404, 'birthday', 4, 'От кого этот подарок?', 'text', 'FROM_WHO', true),
(405, 'birthday', 5, 'Любимый жанр именинника', 'tags', 'GENRE', true),
(406, 'birthday', 6, 'Настроение песни', 'tags', 'MOOD', true),
(407, 'birthday', 7, 'Вокал', 'radio', 'VOCAL', true);


-- ==============================================================================
-- 3. ВАРИАНТЫ ОТВЕТОВ ДЛЯ ТЕГОВ И РАДИОКНОПОК (QUESTION OPTIONS)
-- Привязываемся к ID из таблицы questions
-- ==============================================================================

-- Опции для Свадьбы
INSERT INTO question_options (question_id, label, value) VALUES
(105, 'Современный Поп', 'modern pop'),
(105, 'Романтический Акустик', 'acoustic guitar romantic pop'),
(105, 'R&B / Soul', 'smooth r&b soul'),
(105, 'Энергичный Танцевальный', 'edm dance pop'),
(106, 'Трогательное (до слез)', 'emotional, touching, cinematic'),
(106, 'Легкое и светлое', 'light, airy, happy'),
(106, 'Качающее (для танцев)', 'upbeat, energetic, party'),
(107, 'Мужской голос', 'male vocals'),
(107, 'Женский голос', 'female vocals'),
(107, 'Красивый дуэт', 'male and female duet vocals');

-- Опции для Корпоратива
INSERT INTO question_options (question_id, label, value) VALUES
(205, 'Эпичный Рок', 'epic arena rock'),
(205, 'Мощный Хип-Хоп', 'energetic hip-hop, heavy bass'),
(205, 'Синтвейв (Ретро)', '80s synthwave, cyber'),
(205, 'Уверенный Поп', 'corporate upbeat pop'),
(206, 'Мотивирующее', 'motivational, inspiring, epic'),
(206, 'Веселое и командное', 'fun, friendly, upbeat'),
(206, 'Пафосное и статусное', 'triumphant, majestic, serious'),
(207, 'Брутальный мужской', 'powerful male vocals'),
(207, 'Яркий женский', 'strong female vocals'),
(207, 'Хор сотрудников', 'group choir vocals');

-- Опции для Босса
INSERT INTO question_options (question_id, label, value) VALUES
(305, 'Рэп-прожарка', 'comedic rap, trap beat'),
(305, 'Русский Шансон', 'russian chanson, acoustic guitar'),
(305, 'Эстрадный хит', 'classic russian pop'),
(305, 'Джаз / Лаунж', 'smooth jazz lounge'),
(306, 'Юмористическое', 'comedic, funny, sarcastic'),
(306, 'Уважительное', 'respectful, calm, honoring'),
(306, 'Энергичное', 'hype, energetic'),
(307, 'Мужской (низкий тембр)', 'deep male vocals'),
(307, 'Женский (мелодичный)', 'melodic female vocals');

-- Опции для Дня Рождения
INSERT INTO question_options (question_id, label, value) VALUES
(405, 'Pop / Dark Trap', 'dark trap, heavy bass'),
(405, 'Инди-Поп', 'indie pop, happy'),
(405, 'Рок-н-Ролл', 'upbeat rock and roll'),
(405, 'Lo-Fi Chill', 'lo-fi hip hop chill'),
(406, 'Отрывное / Вечеринка', 'party, hype, explosive'),
(406, 'Душевное', 'warm, friendly, acoustic'),
(406, 'Шуточное', 'funny, silly'),
(407, 'Мужской', 'male vocals'),
(407, 'Женский', 'female vocals');

-- Восстанавливаем автоинкремент для вопросов, чтобы при добавлении новых через админку ничего не сломалось
SELECT setval('questions_id_seq', (SELECT MAX(id) FROM questions));