-- Добавляем вопрос «Что ты хочешь, чтобы этот человек почувствовал?» (FEELING)
-- в топ-5 категорий по частоте заказов: birthday, wedding, mother, love, friendship.
--
-- Логика безопасная:
--   • FEELING — необязательный вопрос; если клиент не заполнил, StripUnfilledPlaceholders
--     тихо уберёт предложение из промпта — ничего не сломается.
--   • Вставляется как шаг 5; GENRE/MOOD/VOCAL/TEMPO/EXTRA сдвигаются на +1.
--   • В шаблоне [FEELING] добавляется ПЕРЕД «Tempo feel:», чтобы попасть в тело
--     описания к Suno, а не быть вырезанным cleanSubstitutedTemplate.
--   • Ключевые слова VOCAL/GENRE/MOOD/TEMPO/EXTRA остаются TemplateSkipKeys —
--     Go-код не меняется.

-- 1. Сдвигаем стиль-вопросы на +1, чтобы освободить step 5 для FEELING
UPDATE questions
SET step_number = step_number + 1
WHERE category_id IN ('birthday', 'wedding', 'mother', 'love', 'friendship')
  AND mapping_key IN ('GENRE', 'MOOD', 'VOCAL', 'TEMPO', 'EXTRA');

-- 2. Вставляем новый вопрос FEELING шагом 5 для каждой из 5 категорий
INSERT INTO questions (category_id, step_number, question_text, ui_type, mapping_key, is_required)
VALUES
    ('birthday',   5, 'Что ты хочешь, чтобы именинник почувствовал, услышав эту песню? (можно пропустить)', 'text', 'FEELING', false),
    ('wedding',    5, 'Что должны почувствовать молодожёны, услышав эту песню? (можно пропустить)',           'text', 'FEELING', false),
    ('mother',     5, 'Что ты хочешь, чтобы мама почувствовала, услышав эту песню? (можно пропустить)',      'text', 'FEELING', false),
    ('love',       5, 'Что ты хочешь, чтобы любимый человек почувствовал? (можно пропустить)',               'text', 'FEELING', false),
    ('friendship', 5, 'Что ты хочешь, чтобы друг почувствовал, услышав эту песню? (можно пропустить)',       'text', 'FEELING', false);

-- 3. Обновляем шаблоны: добавляем [FEELING] перед «Tempo feel:»
--    Если клиент заполнил вопрос — строчка попадёт в тело описания к Suno.
--    Если пропустил — StripUnfilledPlaceholders уберёт предложение целиком.

UPDATE categories
SET base_prompt_template = replace(
    base_prompt_template,
    ' Tempo feel: [TEMPO].',
    ' The person giving this gift wants the listener to feel: [FEELING]. Tempo feel: [TEMPO].'
)
WHERE id = 'birthday';

UPDATE categories
SET base_prompt_template = replace(
    base_prompt_template,
    ' Tempo feel: [TEMPO].',
    ' What the couple should feel hearing this song: [FEELING]. Tempo feel: [TEMPO].'
)
WHERE id = 'wedding';

UPDATE categories
SET base_prompt_template = replace(
    base_prompt_template,
    ' Tempo feel: [TEMPO].',
    ' The person giving this gift wants mom to feel: [FEELING]. Tempo feel: [TEMPO].'
)
WHERE id = 'mother';

UPDATE categories
SET base_prompt_template = replace(
    base_prompt_template,
    ' Tempo feel: [TEMPO].',
    ' The author wants [PARTNER] to feel: [FEELING]. Tempo feel: [TEMPO].'
)
WHERE id = 'love';

UPDATE categories
SET base_prompt_template = replace(
    base_prompt_template,
    ' Tempo feel: [TEMPO].',
    ' The gift-giver wants their friend to feel: [FEELING]. Tempo feel: [TEMPO].'
)
WHERE id = 'friendship';

-- 4. Сбрасываем автоинкремент вопросов
SELECT setval('questions_id_seq', (SELECT MAX(id) FROM questions));
