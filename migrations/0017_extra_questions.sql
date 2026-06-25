-- Дополнительный гибкий вопрос + темп для всех категорий квиза.

INSERT INTO questions (category_id, step_number, question_text, ui_type, mapping_key, is_required, option_source, config)
SELECT
    c.id,
    COALESCE((SELECT MAX(q.step_number) FROM questions q WHERE q.category_id = c.id), 0) + 1,
    'Темп песни',
    'radio',
    'TEMPO',
    false,
    'inline',
    '{}'::jsonb
FROM categories c
WHERE NOT EXISTS (
    SELECT 1 FROM questions q WHERE q.category_id = c.id AND q.mapping_key = 'TEMPO'
);

INSERT INTO question_options (question_id, label, value)
SELECT q.id, o.label, o.value
FROM questions q
CROSS JOIN (VALUES
    ('Медленный', 'slow tempo'),
    ('Средний', 'medium tempo'),
    ('Быстрый', 'upbeat fast tempo')
) AS o(label, value)
WHERE q.mapping_key = 'TEMPO'
  AND NOT EXISTS (SELECT 1 FROM question_options x WHERE x.question_id = q.id);

INSERT INTO questions (category_id, step_number, question_text, ui_type, mapping_key, is_required, option_source, config)
SELECT
    c.id,
    COALESCE((SELECT MAX(q.step_number) FROM questions q WHERE q.category_id = c.id), 0) + 1,
    'Что ещё важно упомянуть в песне?',
    'textarea',
    'EXTRA',
    false,
    'inline',
    '{"placeholder": "Имена, шутки, фразы, которые обязательно должны прозвучать…"}'::jsonb
FROM categories c
WHERE NOT EXISTS (
    SELECT 1 FROM questions q WHERE q.category_id = c.id AND q.mapping_key = 'EXTRA'
);

UPDATE categories
SET base_prompt_template = base_prompt_template
    || ' Tempo feel: [TEMPO]. Optional extra details for lyrics: [EXTRA].'
WHERE base_prompt_template NOT LIKE '%[EXTRA]%';
