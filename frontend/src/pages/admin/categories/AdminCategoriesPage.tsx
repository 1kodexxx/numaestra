import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { adminCategoryApi } from '@entities/admin-category'
import type { AdminCategory } from '@entities/admin-category'
import { Spinner } from '@shared/ui'
import { ApiError } from '@shared/api'

const inputCls = [
  'w-full bg-bg3 border border-border rounded-lg px-3 py-2',
  'text-txt text-sm outline-none transition-colors',
  'focus:border-accent2 placeholder:text-muted',
].join(' ')

export function AdminCategoriesPage() {
  const [categories, setCategories] = useState<AdminCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [id, setId] = useState('')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [coverImageUrl, setCoverImageUrl] = useState('')
  const [seoTags, setSeoTags] = useState('')
  const [basePromptTemplate, setBasePromptTemplate] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function load() {
    setLoading(true)
    adminCategoryApi
      .list()
      .then((data) => { setCategories(data); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)
    setSubmitting(true)
    try {
      await adminCategoryApi.create({
        id,
        title,
        description,
        cover_image_url: coverImageUrl,
        seo_tags: seoTags.split(',').map((t) => t.trim()).filter(Boolean),
        base_prompt_template: basePromptTemplate,
      })
      setId(''); setTitle(''); setDescription(''); setCoverImageUrl(''); setSeoTags(''); setBasePromptTemplate('')
      setShowForm(false)
      load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось создать категорию')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(categoryId: string) {
    if (!confirm(`Удалить категорию "${categoryId}"? Вопросы категории удалятся вместе с ней.`)) return
    try {
      await adminCategoryApi.remove(categoryId)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить категорию')
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Категории</h1>
        <button
          className="px-4 py-2 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] transition-all"
          onClick={() => setShowForm((v) => !v)}
        >
          {showForm ? 'Отмена' : '+ Новая категория'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="bg-bg2 border border-border rounded-xl p-6 mb-6 flex flex-col gap-3 max-w-2xl">
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">ID (слаг, например "general")</label>
              <input className={inputCls} value={id} onChange={(e) => setId(e.target.value)} required />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">Заголовок</label>
              <input className={inputCls} value={title} onChange={(e) => setTitle(e.target.value)} required />
            </div>
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted">Описание (вступительный текст для квиза)</label>
            <textarea className={inputCls} rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">URL картинки</label>
              <input className={inputCls} value={coverImageUrl} onChange={(e) => setCoverImageUrl(e.target.value)} placeholder="/images/covers/example.svg" />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs text-muted">Тэги (через запятую)</label>
              <input className={inputCls} value={seoTags} onChange={(e) => setSeoTags(e.target.value)} placeholder="свадьба, подарок" />
            </div>
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted">
              Шаблон промпта для Suno (плейсхолдеры вида [KEY] заменяются ответами на вопросы)
            </label>
            <textarea
              className={inputCls}
              rows={3}
              value={basePromptTemplate}
              onChange={(e) => setBasePromptTemplate(e.target.value)}
              placeholder="Create a [MOOD] [GENRE] song about [NAME]..."
              required
            />
          </div>
          {formError && <div className="bg-error/10 border border-error/30 rounded-lg px-3 py-2 text-error text-sm">{formError}</div>}
          <button
            type="submit"
            disabled={submitting}
            className="self-start px-4 py-2 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] disabled:opacity-60 transition-all"
          >
            {submitting ? 'Создаём...' : 'Создать'}
          </button>
        </form>
      )}

      {loading && <div className="py-10"><Spinner /></div>}
      {error && <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm mb-4">{error}</div>}

      {!loading && (
        <div className="flex flex-col gap-2">
          {categories.map((c) => (
            <div key={c.id} className="bg-bg2 border border-border rounded-lg px-5 py-4 flex items-center justify-between gap-4">
              <div>
                <div className="font-semibold">{c.title} <span className="text-muted text-xs font-normal">({c.id})</span></div>
                <div className="text-sm text-muted mt-0.5">{c.description}</div>
                <div className="flex flex-wrap gap-1.5 mt-2">
                  {(c.seo_tags ?? []).map((t) => (
                    <span key={t} className="bg-accent/15 border border-accent/30 text-accent px-2 py-0.5 rounded-full text-xs">{t}</span>
                  ))}
                </div>
              </div>
              <div className="flex gap-2 shrink-0">
                <Link
                  to={`/admin/categories/${c.id}`}
                  className="px-3 py-2 rounded-lg text-sm font-medium bg-transparent border border-accent2 text-accent no-underline hover:bg-accent/10 transition-colors"
                >
                  Редактировать
                </Link>
                <button
                  className="px-3 py-2 rounded-lg text-sm font-medium bg-transparent border border-border text-muted cursor-pointer hover:text-error hover:border-error transition-colors"
                  onClick={() => handleDelete(c.id)}
                >
                  Удалить
                </button>
              </div>
            </div>
          ))}
          {categories.length === 0 && <div className="text-muted text-sm">Категорий пока нет.</div>}
        </div>
      )}
    </div>
  )
}
