import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { adminCategoryApi } from '@entities/admin-category'
import type { AdminCategory, AdminQuestion, AdminOption } from '@entities/admin-category'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, Panel, ErrorBanner, EmptyState, StatusBadge, Field, Select } from '@widgets/admin-layout'

const UI_TYPES: AdminQuestion['ui_type'][] = ['text', 'textarea', 'select', 'tags', 'radio']

export function AdminCategoryEditPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [category, setCategory] = useState<AdminCategory | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [coverImageUrl, setCoverImageUrl] = useState('')
  const [seoTags, setSeoTags] = useState('')
  const [basePromptTemplate, setBasePromptTemplate] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  function load() {
    setLoading(true)
    adminCategoryApi
      .get(id)
      .then((c) => {
        setCategory(c)
        setTitle(c.title); setDescription(c.description); setCoverImageUrl(c.cover_image_url)
        setSeoTags((c.seo_tags ?? []).join(', ')); setBasePromptTemplate(c.base_prompt_template)
        setError(null)
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [id])

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaveError(null); setSaving(true)
    try {
      await adminCategoryApi.update(id, {
        title, description,
        cover_image_url: coverImageUrl,
        seo_tags: seoTags.split(',').map((t) => t.trim()).filter(Boolean),
        base_prompt_template: basePromptTemplate,
      })
      load()
    } catch (err) {
      setSaveError(err instanceof ApiError ? err.message : 'Не удалось сохранить изменения')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>
  if (error) return <ErrorBanner>{error}</ErrorBanner>
  if (!category) return null

  return (
    <div style={{ maxWidth: 820 }}>
      <Link to="/admin/categories" style={{ color: A.txt2, textDecoration: 'none', fontSize: '13px' }}>← К списку категорий</Link>

      <div style={{ display: 'flex', alignItems: 'baseline', gap: '10px', margin: '12px 0 28px' }}>
        <h1 style={{ fontSize: '26px', fontWeight: 800, letterSpacing: '-0.03em' }}>{category.title}</h1>
        <span style={{ fontSize: '13px', color: A.txt3, fontFamily: 'monospace' }}>{category.id}</span>
      </div>

      <Panel style={{ padding: '24px', marginBottom: '28px' }}>
        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
          <TextField label="Заголовок" value={title} onChange={setTitle} required surfaceColor={A.surface} />
          <TextField label="Описание (вступление для квиза)" value={description} onChange={setDescription} multiline rows={2} surfaceColor={A.surface} />
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            <TextField label="URL картинки" value={coverImageUrl} onChange={setCoverImageUrl} surfaceColor={A.surface} />
            <TextField label="Тэги (через запятую)" value={seoTags} onChange={setSeoTags} surfaceColor={A.surface} />
          </div>
          <TextField label="Шаблон промпта для Suno" value={basePromptTemplate} onChange={setBasePromptTemplate} multiline rows={3} required surfaceColor={A.surface} />
          {saveError && <ErrorBanner>{saveError}</ErrorBanner>}
          <Button type="submit" loading={saving} style={{ alignSelf: 'flex-start' }}>Сохранить изменения</Button>
        </form>
      </Panel>

      <QuestionsEditor categoryId={category.id} questions={category.questions ?? []} onChange={load} />

      <div style={{ marginTop: '32px', paddingTop: '24px', borderTop: `1px solid ${A.border}` }}>
        <Button
          variant="outlined"
          onClick={async () => {
            if (!confirm(`Удалить категорию "${category.id}"?`)) return
            await adminCategoryApi.remove(category.id)
            navigate('/admin/categories')
          }}
          style={{ color: '#f87171', borderColor: 'rgba(239,68,68,0.4)' }}
        >
          Удалить категорию
        </Button>
      </div>
    </div>
  )
}

function QuestionsEditor({ categoryId, questions, onChange }: { categoryId: string; questions: AdminQuestion[]; onChange: () => void }) {
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [stepNumber, setStepNumber] = useState(questions.length + 1)
  const [questionText, setQuestionText] = useState('')
  const [uiType, setUiType] = useState<AdminQuestion['ui_type']>('text')
  const [mappingKey, setMappingKey] = useState('')
  const [isRequired, setIsRequired] = useState(true)
  const [optionsText, setOptionsText] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function resetForm() {
    setEditingId(null); setStepNumber(questions.length + 1); setQuestionText('')
    setUiType('text'); setMappingKey(''); setIsRequired(true); setOptionsText(''); setFormError(null)
  }

  function startEdit(q: AdminQuestion) {
    setEditingId(q.id); setStepNumber(q.step_number); setQuestionText(q.question_text)
    setUiType(q.ui_type); setMappingKey(q.mapping_key); setIsRequired(q.is_required)
    setOptionsText((q.options ?? []).map((o) => `${o.label}=${o.value}`).join('\n'))
    setShowForm(true)
  }

  function parseOptions(): AdminOption[] {
    return optionsText.split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
      const [label, value] = line.split('=')
      return { label: (label ?? '').trim(), value: (value ?? label ?? '').trim() }
    })
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null); setSubmitting(true)
    const payload = { step_number: stepNumber, question_text: questionText, ui_type: uiType, mapping_key: mappingKey, is_required: isRequired, options: parseOptions() }
    try {
      if (editingId != null) await adminCategoryApi.updateQuestion(categoryId, editingId, payload)
      else await adminCategoryApi.addQuestion(categoryId, payload)
      setShowForm(false); resetForm(); onChange()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось сохранить вопрос')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(questionId: number) {
    if (!confirm('Удалить вопрос?')) return
    await adminCategoryApi.removeQuestion(categoryId, questionId)
    onChange()
  }

  const needsOptions = uiType === 'select' || uiType === 'tags' || uiType === 'radio'

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
        <h2 style={{ fontSize: '18px', fontWeight: 800, letterSpacing: '-0.02em' }}>Вопросы квиза</h2>
        <Button variant={showForm ? 'outlined' : 'tonal'} size="sm" onClick={() => { resetForm(); setShowForm(v => !v) }}>
          {showForm ? 'Отмена' : '+ Вопрос'}
        </Button>
      </div>

      {showForm && (
        <Panel style={{ padding: '22px', marginBottom: '16px' }}>
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '90px 1fr', gap: '14px' }}>
              <Field label="Шаг">
                <input type="number" value={stepNumber} onChange={(e) => setStepNumber(Number(e.target.value))}
                  style={{ background: A.surface2, border: `1px solid ${A.border}`, borderRadius: '12px', padding: '12px 14px', color: A.txt, fontSize: '14px', fontFamily: 'inherit', outline: 'none', width: '100%' }} />
              </Field>
              <TextField label="Текст вопроса" value={questionText} onChange={setQuestionText} required surfaceColor={A.surface} />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' }}>
              <Field label="Тип поля">
                <Select value={uiType} onChange={(v) => setUiType(v as AdminQuestion['ui_type'])}>
                  {UI_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                </Select>
              </Field>
              <TextField label="Mapping key — плейсхолдер [KEY]" value={mappingKey} onChange={(v) => setMappingKey(v.toUpperCase())} required surfaceColor={A.surface} />
            </div>
            {needsOptions && (
              <TextField
                label='Варианты ответа — «Метка=значение», по одному на строку'
                value={optionsText} onChange={setOptionsText}
                multiline rows={4}
                placeholder="Современный Поп=modern pop"
                surfaceColor={A.surface}
              />
            )}
            <label style={{ display: 'flex', alignItems: 'center', gap: '9px', fontSize: '14px', color: A.txt2, cursor: 'pointer' }}>
              <input type="checkbox" checked={isRequired} onChange={(e) => setIsRequired(e.target.checked)} style={{ width: 16, height: 16, accentColor: A.accent, cursor: 'pointer' }} />
              Обязательный вопрос
            </label>
            {formError && <ErrorBanner>{formError}</ErrorBanner>}
            <Button type="submit" loading={submitting} style={{ alignSelf: 'flex-start' }}>
              {editingId != null ? 'Сохранить вопрос' : 'Добавить вопрос'}
            </Button>
          </form>
        </Panel>
      )}

      {questions.length === 0
        ? <Panel><EmptyState icon="❓" text="Вопросов пока нет." /></Panel>
        : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            {[...questions].sort((a, b) => a.step_number - b.step_number).map((q) => (
              <Panel key={q.id} style={{ padding: '14px 18px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '14px' }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span style={{ fontSize: '12px', fontWeight: 700, color: A.accent }}>#{q.step_number}</span>
                    <span style={{ fontSize: '14px', fontWeight: 600, color: A.txt }}>{q.question_text}</span>
                    {q.is_required && <span style={{ color: '#f87171', fontSize: '13px' }}>*</span>}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '7px' }}>
                    <StatusBadge label={q.ui_type} tone="muted" />
                    <span style={{ fontSize: '12px', color: A.txt3, fontFamily: 'monospace' }}>{q.mapping_key}</span>
                  </div>
                </div>
                <div style={{ display: 'flex', gap: '8px', flexShrink: 0 }}>
                  <Button variant="outlined" size="sm" onClick={() => startEdit(q)}>Изменить</Button>
                  <Button variant="text" size="sm" onClick={() => handleDelete(q.id)} style={{ color: '#f87171' }}>Удалить</Button>
                </div>
              </Panel>
            ))}
          </div>
        )}
    </div>
  )
}
