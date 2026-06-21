import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { adminCategoryApi } from '@entities/admin-category'
import type { AdminCategory, AdminQuestion, AdminOption } from '@entities/admin-category'
import { Spinner } from '@shared/ui'
import { ApiError } from '@shared/api'

const inputCls = [
  'w-full bg-bg3 border border-border rounded-lg px-3 py-2',
  'text-txt text-sm outline-none transition-colors',
  'focus:border-accent2 placeholder:text-muted',
].join(' ')

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
        setTitle(c.title)
        setDescription(c.description)
        setCoverImageUrl(c.cover_image_url)
        setSeoTags((c.seo_tags ?? []).join(', '))
        setBasePromptTemplate(c.base_prompt_template)
        setError(null)
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [id])

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaveError(null)
    setSaving(true)
    try {
      await adminCategoryApi.update(id, {
        title,
        description,
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

  if (loading) return <div className="py-10"><Spinner /></div>
  if (error) return <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm">{error}</div>
  if (!category) return null

  return (
    <div className="max-w-3xl">
      <Link to="/admin/categories" className="text-muted no-underline text-sm hover:text-txt transition-colors">← К списку категорий</Link>
      <h1 className="text-2xl font-bold mt-2 mb-6">{category.title} <span className="text-muted text-sm font-normal">({category.id})</span></h1>

      <form onSubmit={handleSave} className="bg-bg2 border border-border rounded-xl p-6 mb-8 flex flex-col gap-3">
        <div className="flex flex-col gap-1">
          <label className="text-xs text-muted">Заголовок</label>
          <input className={inputCls} value={title} onChange={(e) => setTitle(e.target.value)} required />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-xs text-muted">Описание (вступительный текст для квиза)</label>
          <textarea className={inputCls} rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted">URL картинки</label>
            <input className={inputCls} value={coverImageUrl} onChange={(e) => setCoverImageUrl(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted">Тэги (через запятую)</label>
            <input className={inputCls} value={seoTags} onChange={(e) => setSeoTags(e.target.value)} />
          </div>
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-xs text-muted">Шаблон промпта для Suno</label>
          <textarea className={inputCls} rows={3} value={basePromptTemplate} onChange={(e) => setBasePromptTemplate(e.target.value)} required />
        </div>
        {saveError && <div className="bg-error/10 border border-error/30 rounded-lg px-3 py-2 text-error text-sm">{saveError}</div>}
        <button
          type="submit"
          disabled={saving}
          className="self-start px-4 py-2 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] disabled:opacity-60 transition-all"
        >
          {saving ? 'Сохраняем...' : 'Сохранить'}
        </button>
      </form>

      <QuestionsEditor categoryId={category.id} questions={category.questions ?? []} onChange={load} />

      <button
        className="mt-8 px-4 py-2 rounded-lg text-sm font-medium bg-transparent border border-border text-muted cursor-pointer hover:text-error hover:border-error transition-colors"
        onClick={async () => {
          if (!confirm(`Удалить категорию "${category.id}"?`)) return
          await adminCategoryApi.remove(category.id)
          navigate('/admin/categories')
        }}
      >
        Удалить категорию
      </button>
    </div>
  )
}

function QuestionsEditor({
  categoryId, questions, onChange,
}: { categoryId: string; questions: AdminQuestion[]; onChange: () => void }) {
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [stepNumber, setStepNumber] = useState(questions.length + 1)
  const [questionText, setQuestionText] = useState('')
  const [uiType, setUiType] = useState<AdminQuestion['ui_type']>('text')
  const [mappingKey, setMappingKey] = useState('')
  const [isRequired, setIsRequired] = useState(true)
  const [optionsText, setOptionsText] = useState('') // "Метка=значение" по одной на строку
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function resetForm() {
    setEditingId(null)
    setStepNumber(questions.length + 1)
    setQuestionText('')
    setUiType('text')
    setMappingKey('')
    setIsRequired(true)
    setOptionsText('')
    setFormError(null)
  }

  function startEdit(q: AdminQuestion) {
    setEditingId(q.id)
    setStepNumber(q.step_number)
    setQuestionText(q.question_text)
    setUiType(q.ui_type)
    setMappingKey(q.mapping_key)
    setIsRequired(q.is_required)
    setOptionsText((q.options ?? []).map((o) => `${o.label}=${o.value}`).join('\n'))
    setShowForm(true)
  }

  function parseOptions(): AdminOption[] {
    return optionsText
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const [label, value] = line.split('=')
        return { label: (label ?? '').trim(), value: (value ?? label ?? '').trim() }
      })
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)
    setSubmitting(true)
    const payload = {
      step_number: stepNumber,
      question_text: questionText,
      ui_type: uiType,
      mapping_key: mappingKey,
      is_required: isRequired,
      options: parseOptions(),
    }
    try {
      if (editingId != null) {
        await adminCategoryApi.updateQuestion(categoryId, editingId, payload)
      } else {
        await adminCategoryApi.addQuestion(categoryId, payload)
      }
      setShowForm(false)
      resetForm()
      onChange()
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
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-lg font-bold">Вопросы квиза</h2>
        <button
          className="px-3 py-1.5 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] transition-all"
          onClick={() => { resetForm(); setShowForm((v) => !v) }}
        >
          {showForm ? 'Отмена' : '+ Вопрос'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-bg2 border border-border rounded-xl p-5 mb-4 flex flex-col gap-3">
          <div className="grid grid-cols-[80px_1fr] gap-3">
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">Шаг</label>
              <input type="number" className={inputCls} value={stepNumber} onChange={(e) => setStepNumber(Number(e.target.value))} />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">Текст вопроса</label>
              <input className={inputCls} value={questionText} onChange={(e) => setQuestionText(e.target.value)} required />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">Тип поля</label>
              <select className={inputCls} value={uiType} onChange={(e) => setUiType(e.target.value as AdminQuestion['ui_type'])}>
                {UI_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">Mapping key (плейсхолдер [KEY] в шаблоне)</label>
              <input className={inputCls} value={mappingKey} onChange={(e) => setMappingKey(e.target.value.toUpperCase())} required />
            </div>
          </div>
          {needsOptions && (
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">Варианты ответа — по одному на строку, формат "Метка=значение"</label>
              <textarea className={inputCls} rows={4} value={optionsText} onChange={(e) => setOptionsText(e.target.value)}
                placeholder="Современный Поп=modern pop" />
            </div>
          )}
          <label className="flex items-center gap-2 text-sm text-muted">
            <input type="checkbox" checked={isRequired} onChange={(e) => setIsRequired(e.target.checked)} />
            Обязательный вопрос
          </label>
          {formError && <div className="bg-error/10 border border-error/30 rounded-lg px-3 py-2 text-error text-sm">{formError}</div>}
          <button
            type="submit"
            disabled={submitting}
            className="self-start px-4 py-2 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] disabled:opacity-60 transition-all"
          >
            {submitting ? 'Сохраняем...' : editingId != null ? 'Сохранить вопрос' : 'Добавить вопрос'}
          </button>
        </form>
      )}

      <div className="flex flex-col gap-2">
        {[...questions].sort((a, b) => a.step_number - b.step_number).map((q) => (
          <div key={q.id} className="bg-bg2 border border-border rounded-lg px-4 py-3 flex items-center justify-between gap-4">
            <div>
              <div className="text-sm font-medium">
                <span className="text-muted">#{q.step_number}</span> {q.question_text}
                {q.is_required && <span className="text-error ml-1">*</span>}
              </div>
              <div className="text-xs text-muted mt-0.5">{q.ui_type} · mapping_key: {q.mapping_key}</div>
            </div>
            <div className="flex gap-2 shrink-0">
              <button className="px-3 py-1.5 rounded-lg text-xs font-medium bg-transparent border border-accent2 text-accent cursor-pointer hover:bg-accent/10 transition-colors" onClick={() => startEdit(q)}>
                Изменить
              </button>
              <button className="px-3 py-1.5 rounded-lg text-xs font-medium bg-transparent border border-border text-muted cursor-pointer hover:text-error hover:border-error transition-colors" onClick={() => handleDelete(q.id)}>
                Удалить
              </button>
            </div>
          </div>
        ))}
        {questions.length === 0 && <div className="text-muted text-sm">Вопросов пока нет.</div>}
      </div>
    </div>
  )
}
