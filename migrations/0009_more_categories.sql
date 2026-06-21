-- Расширяем каталог категорий: добавляем 22 новые категории "на все случаи жизни"
-- в дополнение к 4 существующим (wedding, corporate, boss, birthday).
-- В отличие от 0004_seed_categories.sql эта миграция НЕ truncate-ит таблицы —
-- она применяется ровно один раз (см. pkg/migrate) и не должна стирать
-- уже существующие категории/заказы.

-- ==============================================================================
-- 1. КАТЕГОРИИ
-- ==============================================================================
INSERT INTO categories (id, title, description, cover_image_url, seo_tags, base_prompt_template) VALUES
(
    'anniversary',
    'Годовщина свадьбы',
    'Песня про путь, который вы прошли вместе — от первой встречи до сегодняшнего дня.',
    '',
    '{"годовщина", "юбилей свадьбы", "отношения", "любовь"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Celebrating [YEARS] years together for [PARTNER_A] and [PARTNER_B]. How they met: [MEET_STORY].'
),
(
    'proposal',
    'Предложение руки и сердца',
    'Романтичный трек для момента, когда вы скажете «Будь моей/моим навсегда».',
    '',
    '{"предложение", "помолвка", "романтика", "любовь"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. [PROPOSER] is about to propose to [PARTNER]. How they met: [MEET_STORY]. The place chosen for the proposal: [PLACE].'
),
(
    'baby',
    'Рождение ребёнка',
    'Колыбельная или нежная песня в честь нового члена семьи.',
    '',
    '{"рождение ребёнка", "малыш", "колыбельная", "семья"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. The song welcomes a newborn named [BABY_NAME], born on [DATE]. Parents: [PARENTS]. Wishes for the baby future: [WISHES].'
),
(
    'mother',
    'Подарок маме',
    'Песня с благодарностью самому важному человеку в жизни.',
    '',
    '{"мама", "8 марта", "день матери", "благодарность"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Dedicated to a mother named [NAME]. What she means to her children: [MEANING]. A memory that describes her best: [MEMORY]. Occasion: [OCCASION].'
),
(
    'father',
    'Подарок папе',
    'Песня-благодарность отцу — для 23 февраля, дня отца или просто от всего сердца.',
    '',
    '{"папа", "23 февраля", "день отца", "благодарность"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Dedicated to a father named [NAME]. His role and character: [CHARACTER]. A lesson or memory from him: [MEMORY]. Occasion: [OCCASION].'
),
(
    'friendship',
    'Лучшему другу',
    'Песня-признание для друга или подруги, которые всегда рядом.',
    '',
    '{"дружба", "лучший друг", "подруга", "подарок"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. About friendship between [NAME1] and [NAME2]. How they met: [MEET_STORY]. Inside jokes or shared memories: [MEMORIES].'
),
(
    'roast',
    'Дружеский роуст',
    'Весёлая ироничная песня про «недостатки» друга — с любовью и юмором.',
    '',
    '{"роуст", "прожарка", "юмор", "день рождения"}',
    'Create a [MOOD] [GENRE] comedic roast song with [VOCAL]. The lyrics must be in Russian language, funny and sarcastic but friendly. The target is [NAME]. Funny habits or flaws to roast: [FLAWS]. Occasion: [OCCASION].'
),
(
    'love',
    'Признание в любви',
    'Самая важная песня — о том, что вы чувствуете к любимому человеку.',
    '',
    '{"любовь", "признание", "романтика", "для второй половинки"}',
    'Create a [MOOD] [GENRE] love song with [VOCAL]. The lyrics must be in Russian language. For [PARTNER] from [AUTHOR]. Why this person is special: [WHY]. A memory together: [MEMORY].'
),
(
    'breakup',
    'Песня после расставания',
    'Терапевтическая или ироничная песня, чтобы пережить расставание и двигаться дальше.',
    '',
    '{"расставание", "развод", "эмоции", "поддержка"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. About moving on after a breakup. What happened (briefly): [STORY]. Feelings now: [FEELINGS]. What the future looks like: [FUTURE].'
),
(
    'farewell',
    'Прощание с коллегой',
    'Тёплые слова коллеге, который уходит из компании — со сцены, с почётом.',
    '',
    '{"прощание", "увольнение", "переезд", "коллеги"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Farewell song for a colleague named [NAME], who is leaving because of [REASON]. Best memories with the team: [MEMORIES]. Wishes for the future: [WISHES].'
),
(
    'retirement',
    'Выход на пенсию',
    'Праздничная песня для того, кто заслужил отдых после долгих лет работы.',
    '',
    '{"пенсия", "выход на пенсию", "коллеги", "благодарность"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Celebrating the retirement of [NAME] after [YEARS] years of work as [ROLE]. Wishes for retirement: [WISHES].'
),
(
    'sports',
    'Спортивная мотивация',
    'Заряжающий трек для команды, забега или личной победы.',
    '',
    '{"спорт", "мотивация", "команда", "марафон"}',
    'Create a [MOOD] [GENRE] motivational song with [VOCAL]. The lyrics must be in Russian language. For [NAME_OR_TEAM] preparing for [EVENT]. The goal: [GOAL]. What keeps them going: [DRIVE].'
),
(
    'travel',
    'Песня о путешествии',
    'Саундтрек для поездки, которая запомнится на всю жизнь.',
    '',
    '{"путешествие", "поездка", "отпуск", "приключение"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. About a trip to [DESTINATION] with [COMPANIONS]. Best moment of the trip: [HIGHLIGHT]. The feeling this journey gave them: [FEELING].'
),
(
    'teacher',
    'Подарок учителю',
    'Песня-благодарность учителю или преподавателю от учеников.',
    '',
    '{"учитель", "преподаватель", "день учителя", "благодарность"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Dedicated to a teacher named [NAME], who teaches [SUBJECT]. What makes them special: [QUALITY]. A memorable lesson or moment: [MEMORY].'
),
(
    'doctor',
    'Подарок врачу',
    'Песня благодарности врачу или медицинскому работнику.',
    '',
    '{"врач", "медик", "день медика", "благодарность"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Dedicated to a doctor or nurse named [NAME], specialty: [SPECIALTY]. Why patients are grateful: [GRATITUDE]. A story that shows their care: [STORY].'
),
(
    'military',
    'Защитнику Отечества',
    'Песня для военного, ветерана или просто настоящего защитника — к 23 февраля и не только.',
    '',
    '{"23 февраля", "военный", "защитник", "армия"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Dedicated to [NAME], serving as [ROLE]. Qualities that make them a true protector: [QUALITIES]. Message from family or friends: [MESSAGE].'
),
(
    'housewarming',
    'Новоселье',
    'Праздничная песня для тех, кто переезжает в новый дом.',
    '',
    '{"новоселье", "новый дом", "переезд", "праздник"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Celebrating the new home of [NAME_OR_FAMILY] at [ADDRESS_OR_CITY]. What this new chapter means to them: [MEANING]. Wishes for the new home: [WISHES].'
),
(
    'pet',
    'Песня про питомца',
    'Забавная или нежная песня в честь любимого питомца.',
    '',
    '{"питомец", "собака", "кот", "домашнее животное"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. About a pet named [PET_NAME], a [SPECIES_BREED]. Funny habits: [HABITS]. Why they are loved so much: [WHY_LOVED].'
),
(
    'apology',
    'Песня-извинение',
    'Когда слов мало — песня поможет искренне попросить прощения.',
    '',
    '{"извинение", "прощение", "примирение"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. An apology to [NAME] for [REASON]. What the author wants them to understand: [MESSAGE].'
),
(
    'memory',
    'В память о близком человеке',
    'Тёплая, бережная песня-посвящение в память о дорогом человеке.',
    '',
    '{"память", "посвящение", "в честь", "ушедшим близким"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language, gentle and respectful. In loving memory of [NAME]. Who they were to the author: [RELATION]. A cherished memory: [MEMORY]. What the author wants to say to them: [MESSAGE].'
),
(
    'newyear',
    'Новогодняя песня',
    'Праздничный трек к Новому году или Рождеству для семьи, друзей или компании.',
    '',
    '{"новый год", "рождество", "праздник", "семья"}',
    'Create a [MOOD] [GENRE] New Year song with [VOCAL]. The lyrics must be in Russian language. For [NAME_OR_FAMILY]. The best moment of this year: [HIGHLIGHT]. Wishes for the new year: [WISHES].'
),
(
    'graduation',
    'Выпускной',
    'Песня для выпускников школы, университета — начало новой главы жизни.',
    '',
    '{"выпускной", "школа", "университет", "студенты"}',
    'Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. For the graduating class of [SCHOOL_OR_GROUP], year [YEAR]. Best memories of these years: [MEMORIES]. Hopes for the future: [HOPES].'
);

-- ==============================================================================
-- 2. ВОПРОСЫ (QUESTIONS)
-- Используем ручное назначение ID (500+, 600+, ...), чтобы легко привязывать Options
-- ==============================================================================

-- ---> ГОДОВЩИНА СВАДЬБЫ (ID 500-599)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(501, 'anniversary', 1, 'Как зовут одного из партнёров?', 'text', 'PARTNER_A', true),
(502, 'anniversary', 2, 'Как зовут второго партнёра?', 'text', 'PARTNER_B', true),
(503, 'anniversary', 3, 'Сколько лет вы вместе?', 'text', 'YEARS', true),
(504, 'anniversary', 4, 'Как вы познакомились?', 'text', 'MEET_STORY', false),
(505, 'anniversary', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(506, 'anniversary', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(507, 'anniversary', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПРЕДЛОЖЕНИЕ РУКИ И СЕРДЦА (ID 600-699)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(601, 'proposal', 1, 'Как зовут вас?', 'text', 'PROPOSER', true),
(602, 'proposal', 2, 'Как зовут вашу половинку?', 'text', 'PARTNER', true),
(603, 'proposal', 3, 'Как вы познакомились?', 'text', 'MEET_STORY', false),
(604, 'proposal', 4, 'Где и как вы планируете сделать предложение?', 'text', 'PLACE', true),
(605, 'proposal', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(606, 'proposal', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(607, 'proposal', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> РОЖДЕНИЕ РЕБЁНКА (ID 700-799)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(701, 'baby', 1, 'Имя малыша?', 'text', 'BABY_NAME', true),
(702, 'baby', 2, 'Дата рождения (можно пропустить)?', 'text', 'DATE', false),
(703, 'baby', 3, 'Имена родителей?', 'text', 'PARENTS', true),
(704, 'baby', 4, 'Что вы желаете малышу?', 'text', 'WISHES', true),
(705, 'baby', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(706, 'baby', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(707, 'baby', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПОДАРОК МАМЕ (ID 800-899)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(801, 'mother', 1, 'Как зовут маму?', 'text', 'NAME', true),
(802, 'mother', 2, 'Что она значит для своих детей?', 'text', 'MEANING', true),
(803, 'mother', 3, 'Воспоминание, которое описывает её лучше всего?', 'text', 'MEMORY', false),
(804, 'mother', 4, 'По какому поводу песня?', 'text', 'OCCASION', true),
(805, 'mother', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(806, 'mother', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(807, 'mother', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПОДАРОК ПАПЕ (ID 900-999)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(901, 'father', 1, 'Как зовут папу?', 'text', 'NAME', true),
(902, 'father', 2, 'Какой он по характеру?', 'text', 'CHARACTER', true),
(903, 'father', 3, 'Урок или воспоминание от него?', 'text', 'MEMORY', false),
(904, 'father', 4, 'По какому поводу песня?', 'text', 'OCCASION', true),
(905, 'father', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(906, 'father', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(907, 'father', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ЛУЧШЕМУ ДРУГУ (ID 1000-1099)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1001, 'friendship', 1, 'Как зовут вас?', 'text', 'NAME1', true),
(1002, 'friendship', 2, 'Как зовут вашего друга/подругу?', 'text', 'NAME2', true),
(1003, 'friendship', 3, 'Как вы познакомились?', 'text', 'MEET_STORY', false),
(1004, 'friendship', 4, 'Общие шутки или воспоминания?', 'text', 'MEMORIES', true),
(1005, 'friendship', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1006, 'friendship', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1007, 'friendship', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ДРУЖЕСКИЙ РОУСТ (ID 1100-1199)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1101, 'roast', 1, 'Кого роустим?', 'text', 'NAME', true),
(1102, 'roast', 2, 'Забавные привычки или «недостатки»?', 'text', 'FLAWS', true),
(1103, 'roast', 3, 'По какому поводу?', 'text', 'OCCASION', false),
(1104, 'roast', 4, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1105, 'roast', 5, 'Какое настроение?', 'tags', 'MOOD', true),
(1106, 'roast', 6, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПРИЗНАНИЕ В ЛЮБВИ (ID 1200-1299)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1201, 'love', 1, 'Как зовут вас?', 'text', 'AUTHOR', true),
(1202, 'love', 2, 'Как зовут любимого человека?', 'text', 'PARTNER', true),
(1203, 'love', 3, 'Почему этот человек особенный?', 'text', 'WHY', true),
(1204, 'love', 4, 'Воспоминание о вас двоих?', 'text', 'MEMORY', false),
(1205, 'love', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1206, 'love', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1207, 'love', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПЕСНЯ ПОСЛЕ РАССТАВАНИЯ (ID 1300-1399)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1301, 'breakup', 1, 'Что произошло (вкратце)?', 'text', 'STORY', true),
(1302, 'breakup', 2, 'Какие чувства сейчас?', 'text', 'FEELINGS', true),
(1303, 'breakup', 3, 'Как видится будущее?', 'text', 'FUTURE', false),
(1304, 'breakup', 4, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1305, 'breakup', 5, 'Какое настроение?', 'tags', 'MOOD', true),
(1306, 'breakup', 6, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПРОЩАНИЕ С КОЛЛЕГОЙ (ID 1400-1499)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1401, 'farewell', 1, 'Как зовут коллегу?', 'text', 'NAME', true),
(1402, 'farewell', 2, 'Причина ухода (можно пропустить)?', 'text', 'REASON', false),
(1403, 'farewell', 3, 'Лучшие воспоминания с командой?', 'text', 'MEMORIES', true),
(1404, 'farewell', 4, 'Что желает коллектив?', 'text', 'WISHES', true),
(1405, 'farewell', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1406, 'farewell', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1407, 'farewell', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ВЫХОД НА ПЕНСИЮ (ID 1500-1599)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1501, 'retirement', 1, 'Как зовут именинника торжества?', 'text', 'NAME', true),
(1502, 'retirement', 2, 'Сколько лет он(а) работал(а)?', 'text', 'YEARS', true),
(1503, 'retirement', 3, 'Кем работал(а)?', 'text', 'ROLE', true),
(1504, 'retirement', 4, 'Что желает коллектив?', 'text', 'WISHES', true),
(1505, 'retirement', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1506, 'retirement', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1507, 'retirement', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> СПОРТИВНАЯ МОТИВАЦИЯ (ID 1600-1699)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1601, 'sports', 1, 'Имя спортсмена или название команды?', 'text', 'NAME_OR_TEAM', true),
(1602, 'sports', 2, 'К какому событию готовитесь?', 'text', 'EVENT', true),
(1603, 'sports', 3, 'Какая цель?', 'text', 'GOAL', true),
(1604, 'sports', 4, 'Что придаёт сил?', 'text', 'DRIVE', false),
(1605, 'sports', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1606, 'sports', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1607, 'sports', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПЕСНЯ О ПУТЕШЕСТВИИ (ID 1700-1799)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1701, 'travel', 1, 'Куда была поездка?', 'text', 'DESTINATION', true),
(1702, 'travel', 2, 'С кем путешествовали (можно пропустить)?', 'text', 'COMPANIONS', false),
(1703, 'travel', 3, 'Лучший момент поездки?', 'text', 'HIGHLIGHT', true),
(1704, 'travel', 4, 'Какие чувства подарило это путешествие?', 'text', 'FEELING', false),
(1705, 'travel', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1706, 'travel', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1707, 'travel', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПОДАРОК УЧИТЕЛЮ (ID 1800-1899)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1801, 'teacher', 1, 'Как зовут учителя?', 'text', 'NAME', true),
(1802, 'teacher', 2, 'Какой предмет преподаёт?', 'text', 'SUBJECT', true),
(1803, 'teacher', 3, 'Что делает его(её) особенным(ой)?', 'text', 'QUALITY', true),
(1804, 'teacher', 4, 'Запоминающийся урок или момент?', 'text', 'MEMORY', false),
(1805, 'teacher', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1806, 'teacher', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1807, 'teacher', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПОДАРОК ВРАЧУ (ID 1900-1999)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(1901, 'doctor', 1, 'Как зовут врача/медика?', 'text', 'NAME', true),
(1902, 'doctor', 2, 'Специализация (можно пропустить)?', 'text', 'SPECIALTY', false),
(1903, 'doctor', 3, 'За что пациенты благодарны?', 'text', 'GRATITUDE', true),
(1904, 'doctor', 4, 'История, которая показывает его(её) заботу?', 'text', 'STORY', false),
(1905, 'doctor', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(1906, 'doctor', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(1907, 'doctor', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ЗАЩИТНИКУ ОТЕЧЕСТВА (ID 2000-2099)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2001, 'military', 1, 'Как зовут защитника?', 'text', 'NAME', true),
(2002, 'military', 2, 'Кем он служит (можно пропустить)?', 'text', 'ROLE', false),
(2003, 'military', 3, 'Какие качества делают его настоящим защитником?', 'text', 'QUALITIES', true),
(2004, 'military', 4, 'Послание от семьи или друзей?', 'text', 'MESSAGE', true),
(2005, 'military', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2006, 'military', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2007, 'military', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> НОВОСЕЛЬЕ (ID 2100-2199)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2101, 'housewarming', 1, 'Имя или фамилия семьи?', 'text', 'NAME_OR_FAMILY', true),
(2102, 'housewarming', 2, 'Адрес или город нового дома (можно пропустить)?', 'text', 'ADDRESS_OR_CITY', false),
(2103, 'housewarming', 3, 'Что значит этот переезд для них?', 'text', 'MEANING', false),
(2104, 'housewarming', 4, 'Пожелания для нового дома?', 'text', 'WISHES', true),
(2105, 'housewarming', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2106, 'housewarming', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2107, 'housewarming', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПЕСНЯ ПРО ПИТОМЦА (ID 2200-2299)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2201, 'pet', 1, 'Имя питомца?', 'text', 'PET_NAME', true),
(2202, 'pet', 2, 'Вид/порода?', 'text', 'SPECIES_BREED', true),
(2203, 'pet', 3, 'Забавные привычки?', 'text', 'HABITS', true),
(2204, 'pet', 4, 'За что его(её) так любят?', 'text', 'WHY_LOVED', false),
(2205, 'pet', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2206, 'pet', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2207, 'pet', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ПЕСНЯ-ИЗВИНЕНИЕ (ID 2300-2399)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2301, 'apology', 1, 'Перед кем извиняетесь?', 'text', 'NAME', true),
(2302, 'apology', 2, 'За что извиняетесь?', 'text', 'REASON', true),
(2303, 'apology', 3, 'Что хотите, чтобы человек понял?', 'text', 'MESSAGE', true),
(2304, 'apology', 4, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2305, 'apology', 5, 'Какое настроение?', 'tags', 'MOOD', true),
(2306, 'apology', 6, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> В ПАМЯТЬ О БЛИЗКОМ ЧЕЛОВЕКЕ (ID 2400-2499)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2401, 'memory', 1, 'Имя дорогого человека?', 'text', 'NAME', true),
(2402, 'memory', 2, 'Кем он(а) был(а) для вас?', 'text', 'RELATION', true),
(2403, 'memory', 3, 'Дорогое воспоминание?', 'text', 'MEMORY', false),
(2404, 'memory', 4, 'Что хотите ему(ей) сказать?', 'text', 'MESSAGE', true),
(2405, 'memory', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2406, 'memory', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2407, 'memory', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> НОВОГОДНЯЯ ПЕСНЯ (ID 2500-2599)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2501, 'newyear', 1, 'Для кого песня (имя или семья)?', 'text', 'NAME_OR_FAMILY', true),
(2502, 'newyear', 2, 'Лучший момент этого года (можно пропустить)?', 'text', 'HIGHLIGHT', false),
(2503, 'newyear', 3, 'Пожелания на новый год?', 'text', 'WISHES', true),
(2504, 'newyear', 4, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2505, 'newyear', 5, 'Какое настроение?', 'tags', 'MOOD', true),
(2506, 'newyear', 6, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ---> ВЫПУСКНОЙ (ID 2600-2699)
INSERT INTO questions (id, category_id, step_number, question_text, ui_type, mapping_key, is_required) VALUES
(2601, 'graduation', 1, 'Название школы/класса/группы?', 'text', 'SCHOOL_OR_GROUP', true),
(2602, 'graduation', 2, 'Год выпуска (можно пропустить)?', 'text', 'YEAR', false),
(2603, 'graduation', 3, 'Лучшие воспоминания этих лет?', 'text', 'MEMORIES', true),
(2604, 'graduation', 4, 'Надежды на будущее?', 'text', 'HOPES', true),
(2605, 'graduation', 5, 'В каком жанре песня?', 'tags', 'GENRE', true),
(2606, 'graduation', 6, 'Какое настроение?', 'tags', 'MOOD', true),
(2607, 'graduation', 7, 'Какой вокал?', 'radio', 'VOCAL', true);

-- ==============================================================================
-- 3. ВАРИАНТЫ ОТВЕТОВ ДЛЯ ТЕГОВ И РАДИОКНОПОК (QUESTION OPTIONS)
-- ==============================================================================

-- Годовщина свадьбы
INSERT INTO question_options (question_id, label, value) VALUES
(505, 'Романтическая баллада', 'romantic acoustic ballad'),
(505, 'Современный поп', 'modern pop'),
(505, 'Джаз', 'smooth jazz'),
(506, 'Трогательное', 'emotional, touching, cinematic'),
(506, 'Тёплое и светлое', 'warm, light, happy'),
(506, 'Торжественное', 'celebratory, majestic'),
(507, 'Мужской голос', 'male vocals'),
(507, 'Женский голос', 'female vocals'),
(507, 'Дуэт', 'male and female duet vocals');

-- Предложение руки и сердца
INSERT INTO question_options (question_id, label, value) VALUES
(605, 'Романтическая баллада', 'romantic ballad'),
(605, 'Акустика', 'acoustic guitar'),
(605, 'Соул', 'smooth soul'),
(606, 'Волнительное', 'nervous, anticipating, emotional'),
(606, 'Нежное', 'tender, gentle'),
(606, 'Торжественное', 'triumphant, celebratory'),
(607, 'Мужской голос', 'male vocals'),
(607, 'Женский голос', 'female vocals'),
(607, 'Дуэт', 'male and female duet vocals');

-- Рождение ребёнка
INSERT INTO question_options (question_id, label, value) VALUES
(705, 'Колыбельная', 'soft lullaby'),
(705, 'Нежный поп', 'soft pop'),
(705, 'Акустика', 'acoustic'),
(706, 'Нежное и спокойное', 'tender, calm, soft'),
(706, 'Радостное', 'joyful, happy'),
(706, 'Трогательное', 'emotional, touching'),
(707, 'Женский (колыбельный)', 'female lullaby vocals'),
(707, 'Мужской голос', 'male vocals'),
(707, 'Дуэт', 'male and female duet vocals');

-- Подарок маме
INSERT INTO question_options (question_id, label, value) VALUES
(805, 'Лирическая баллада', 'lyrical ballad'),
(805, 'Поп', 'pop'),
(805, 'Романс', 'russian romance style'),
(806, 'Трогательное до слёз', 'emotional, touching, cinematic'),
(806, 'Тёплое', 'warm, loving'),
(806, 'Светлое', 'light, happy'),
(807, 'Женский голос', 'female vocals'),
(807, 'Мужской голос', 'male vocals'),
(807, 'Хор/дуэт', 'group choir vocals');

-- Подарок папе
INSERT INTO question_options (question_id, label, value) VALUES
(905, 'Рок-баллада', 'rock ballad'),
(905, 'Эстрадная песня', 'classic russian pop'),
(905, 'Акустика', 'acoustic'),
(906, 'Уважительное', 'respectful, calm, honoring'),
(906, 'Тёплое', 'warm, loving'),
(906, 'Гордое', 'proud, strong'),
(907, 'Мужской голос', 'male vocals'),
(907, 'Женский голос', 'female vocals'),
(907, 'Дуэт', 'male and female duet vocals');

-- Лучшему другу
INSERT INTO question_options (question_id, label, value) VALUES
(1005, 'Поп-рок', 'pop rock'),
(1005, 'Инди', 'indie'),
(1005, 'Акустика', 'acoustic'),
(1006, 'Веселое', 'fun, upbeat'),
(1006, 'Душевное', 'warm, sincere'),
(1006, 'Ностальгическое', 'nostalgic, sentimental'),
(1007, 'Мужской голос', 'male vocals'),
(1007, 'Женский голос', 'female vocals'),
(1007, 'Дуэт', 'male and female duet vocals');

-- Дружеский роуст
INSERT INTO question_options (question_id, label, value) VALUES
(1104, 'Рэп-баттл', 'comedic rap battle, trap beat'),
(1104, 'Шансон', 'russian chanson, acoustic guitar'),
(1104, 'Поп-панк', 'pop punk'),
(1105, 'Дерзкое и смешное', 'cheeky, funny, sarcastic'),
(1105, 'Безобидное подкалывание', 'playful, lighthearted teasing'),
(1105, 'Чёрный юмор', 'dark comedy, sarcastic'),
(1106, 'Мужской (рэп)', 'male rap vocals'),
(1106, 'Женский голос', 'female vocals'),
(1106, 'Дуэт', 'male and female duet vocals');

-- Признание в любви
INSERT INTO question_options (question_id, label, value) VALUES
(1205, 'R&B / Соул', 'smooth r&b soul'),
(1205, 'Романтический поп', 'romantic pop'),
(1205, 'Акустическая баллада', 'acoustic ballad'),
(1206, 'Страстное', 'passionate, intense'),
(1206, 'Нежное', 'tender, gentle'),
(1206, 'Мечтательное', 'dreamy, romantic'),
(1207, 'Мужской голос', 'male vocals'),
(1207, 'Женский голос', 'female vocals'),
(1207, 'Дуэт', 'male and female duet vocals');

-- Песня после расставания
INSERT INTO question_options (question_id, label, value) VALUES
(1304, 'Меланхоличный инди', 'melancholic indie'),
(1304, 'Поп-баллада', 'pop ballad'),
(1304, 'R&B', 'r&b'),
(1305, 'Грустное, но светлое', 'sad but hopeful'),
(1305, 'Освобождающее', 'liberating, empowering'),
(1305, 'Ироничное', 'ironic, sarcastic'),
(1306, 'Женский голос', 'female vocals'),
(1306, 'Мужской голос', 'male vocals'),
(1306, 'Дуэт', 'male and female duet vocals');

-- Прощание с коллегой
INSERT INTO question_options (question_id, label, value) VALUES
(1405, 'Поп', 'pop'),
(1405, 'Рок', 'rock'),
(1405, 'Акустика', 'acoustic'),
(1406, 'Тёплое и благодарное', 'warm, grateful'),
(1406, 'Веселое', 'fun, upbeat'),
(1406, 'Ностальгическое', 'nostalgic'),
(1407, 'Хор (группа)', 'group choir vocals'),
(1407, 'Мужской голос', 'male vocals'),
(1407, 'Женский голос', 'female vocals');

-- Выход на пенсию
INSERT INTO question_options (question_id, label, value) VALUES
(1505, 'Эстрадная песня', 'classic russian pop'),
(1505, 'Джаз', 'smooth jazz'),
(1505, 'Поп', 'pop'),
(1506, 'Торжественное', 'celebratory, majestic'),
(1506, 'Тёплое', 'warm, loving'),
(1506, 'Весёлое', 'fun, upbeat'),
(1507, 'Мужской голос', 'male vocals'),
(1507, 'Женский голос', 'female vocals'),
(1507, 'Хор (группа)', 'group choir vocals');

-- Спортивная мотивация
INSERT INTO question_options (question_id, label, value) VALUES
(1605, 'Эпичный рок', 'epic arena rock'),
(1605, 'Энергичный хип-хоп', 'energetic hip-hop, heavy bass'),
(1605, 'EDM', 'edm dance'),
(1606, 'Заряжающее', 'energetic, hype'),
(1606, 'Бойцовское', 'fierce, fighting spirit'),
(1606, 'Триумфальное', 'triumphant, epic'),
(1607, 'Мужской голос', 'male vocals'),
(1607, 'Женский голос', 'female vocals'),
(1607, 'Командный речитатив', 'group chant vocals');

-- Песня о путешествии
INSERT INTO question_options (question_id, label, value) VALUES
(1705, 'Инди-фолк', 'indie folk'),
(1705, 'Регги', 'reggae'),
(1705, 'Поп-рок', 'pop rock'),
(1706, 'Свободное и лёгкое', 'free, light, breezy'),
(1706, 'Романтическое', 'romantic'),
(1706, 'Приключенческое', 'adventurous, exciting'),
(1707, 'Мужской голос', 'male vocals'),
(1707, 'Женский голос', 'female vocals'),
(1707, 'Дуэт', 'male and female duet vocals');

-- Подарок учителю
INSERT INTO question_options (question_id, label, value) VALUES
(1805, 'Лирическая песня', 'lyrical song'),
(1805, 'Поп', 'pop'),
(1805, 'Хор', 'choir'),
(1806, 'Тёплое', 'warm, loving'),
(1806, 'Уважительное', 'respectful, honoring'),
(1806, 'Радостное', 'joyful, happy'),
(1807, 'Хор (группа учеников)', 'group choir vocals'),
(1807, 'Женский голос', 'female vocals'),
(1807, 'Мужской голос', 'male vocals');

-- Подарок врачу
INSERT INTO question_options (question_id, label, value) VALUES
(1905, 'Лирическая баллада', 'lyrical ballad'),
(1905, 'Поп', 'pop'),
(1905, 'Акустика', 'acoustic'),
(1906, 'Трогательное', 'emotional, touching'),
(1906, 'Благодарное', 'grateful, warm'),
(1906, 'Светлое', 'light, hopeful'),
(1907, 'Женский голос', 'female vocals'),
(1907, 'Мужской голос', 'male vocals'),
(1907, 'Хор (группа)', 'group choir vocals');

-- Защитнику Отечества
INSERT INTO question_options (question_id, label, value) VALUES
(2005, 'Патриотический рок', 'patriotic rock'),
(2005, 'Эстрадная песня', 'classic russian pop'),
(2005, 'Марш', 'military march'),
(2006, 'Гордое', 'proud, strong'),
(2006, 'Торжественное', 'celebratory, majestic'),
(2006, 'Тёплое', 'warm, loving'),
(2007, 'Мужской голос', 'male vocals'),
(2007, 'Женский голос', 'female vocals'),
(2007, 'Хор (группа)', 'group choir vocals');

-- Новоселье
INSERT INTO question_options (question_id, label, value) VALUES
(2105, 'Поп', 'pop'),
(2105, 'Акустика', 'acoustic'),
(2105, 'Фанк', 'funk'),
(2106, 'Радостное', 'joyful, happy'),
(2106, 'Тёплое', 'warm, cozy'),
(2106, 'Праздничное', 'festive, celebratory'),
(2107, 'Мужской голос', 'male vocals'),
(2107, 'Женский голос', 'female vocals'),
(2107, 'Дуэт', 'male and female duet vocals');

-- Песня про питомца
INSERT INTO question_options (question_id, label, value) VALUES
(2205, 'Весёлый поп', 'fun upbeat pop'),
(2205, 'Фанк', 'funk'),
(2205, 'Регги', 'reggae'),
(2206, 'Смешное и доброе', 'funny, playful, sweet'),
(2206, 'Нежное', 'tender, gentle'),
(2206, 'Энергичное', 'energetic, upbeat'),
(2207, 'Мужской голос', 'male vocals'),
(2207, 'Женский голос', 'female vocals'),
(2207, 'Дуэт', 'male and female duet vocals');

-- Песня-извинение
INSERT INTO question_options (question_id, label, value) VALUES
(2304, 'Соул', 'smooth soul'),
(2304, 'Акустическая баллада', 'acoustic ballad'),
(2304, 'Поп', 'pop'),
(2305, 'Искреннее', 'sincere, heartfelt'),
(2305, 'Грустное, но с надеждой', 'sad but hopeful'),
(2305, 'Тёплое', 'warm, gentle'),
(2306, 'Мужской голос', 'male vocals'),
(2306, 'Женский голос', 'female vocals'),
(2306, 'Дуэт', 'male and female duet vocals');

-- В память о близком человеке
INSERT INTO question_options (question_id, label, value) VALUES
(2405, 'Кинематографическая баллада', 'cinematic ballad'),
(2405, 'Акустика', 'acoustic'),
(2405, 'Хор', 'choir'),
(2406, 'Светлая печаль', 'bittersweet, gentle sadness'),
(2406, 'Тёплое', 'warm, loving'),
(2406, 'Умиротворённое', 'peaceful, calm'),
(2407, 'Женский голос', 'female vocals'),
(2407, 'Мужской голос', 'male vocals'),
(2407, 'Хор (группа)', 'group choir vocals');

-- Новогодняя песня
INSERT INTO question_options (question_id, label, value) VALUES
(2504, 'Праздничный поп', 'festive pop'),
(2504, 'Джаз', 'christmas jazz'),
(2504, 'Народный мотив', 'folk christmas style'),
(2505, 'Волшебное', 'magical, wondrous'),
(2505, 'Весёлое', 'fun, joyful'),
(2505, 'Тёплое и семейное', 'warm, family, cozy'),
(2506, 'Хор (группа)', 'group choir vocals'),
(2506, 'Мужской голос', 'male vocals'),
(2506, 'Женский голос', 'female vocals');

-- Выпускной
INSERT INTO question_options (question_id, label, value) VALUES
(2605, 'Поп-рок', 'pop rock'),
(2605, 'Инди', 'indie'),
(2605, 'Эпичная баллада', 'epic ballad'),
(2606, 'Ностальгическое', 'nostalgic, sentimental'),
(2606, 'Вдохновляющее', 'inspiring, uplifting'),
(2606, 'Радостное', 'joyful, happy'),
(2607, 'Хор (группа)', 'group choir vocals'),
(2607, 'Мужской голос', 'male vocals'),
(2607, 'Женский голос', 'female vocals');

-- Восстанавливаем автоинкремент для вопросов, чтобы при добавлении новых через админку ничего не сломалось
SELECT setval('questions_id_seq', (SELECT MAX(id) FROM questions));
