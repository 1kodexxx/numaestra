import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { adminCategoryApi } from '@entities/admin-category'
import type { AdminCategory } from '@entities/admin-category'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, PageHeader, Panel, Grid2, ErrorBanner, EmptyState, StatusBadge } from '@widgets/admin-layout'

export function AdminCategoriesPage() {
  const navigate = useNavigate()
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
        id, title, description,
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
    <div className="admin-page">
      <PageHeader
        title="Категории"
        subtitle={`${categories.length} категорий · карточки и квизы для клиентов`}
        action={<Button variant={showForm ? 'outlined' : 'filled'} onClick={() => setShowForm(v => !v)}>{showForm ? 'Отмена' : '+ Новая категория'}</Button>}
      />

      {showForm && (
        <Panel style={{ padding: '24px', marginBottom: '24px' }}>
          <form onSubmit={handleCreate} className="admin-form-stack">
            <Grid2>
              <TextField label="ID (слаг, напр. wedding)" value={id} onChange={setId} required surfaceColor={A.surface} />
              <TextField label="Заголовок" value={title} onChange={setTitle} required surfaceColor={A.surface} />
            </Grid2>
            <TextField label="Описание (вступление для квиза)" value={description} onChange={setDescription} multiline rows={2} surfaceColor={A.surface} />
            <Grid2>
              <TextField label="URL картинки" value={coverImageUrl} onChange={setCoverImageUrl} placeholder="/images/covers/example.svg" surfaceColor={A.surface} />
              <TextField label="Тэги (через запятую)" value={seoTags} onChange={setSeoTags} placeholder="свадьба, подарок" surfaceColor={A.surface} />
            </Grid2>
            <TextField
              label="Шаблон промпта для Suno"
              value={basePromptTemplate}
              onChange={setBasePromptTemplate}
              multiline rows={3} required
              placeholder="Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Story facts: [NAME] … [EXTRA] подставятся автоматически; стиль уйдёт в Suno tags."
              supportingText="Плейсхолдеры [KEY] заменяются ответами на вопросы квиза."
              surfaceColor={A.surface}
            />
            {formError && <ErrorBanner>{formError}</ErrorBanner>}
            <Button type="submit" loading={submitting} style={{ alignSelf: 'flex-start' }}>Создать категорию</Button>
          </form>
        </Panel>
      )}

      {loading && <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>}
      {error && <ErrorBanner>{error}</ErrorBanner>}

      {!loading && categories.length === 0 && <Panel><EmptyState icon="🗂️" text="Категорий пока нет — создайте первую." /></Panel>}

      {!loading && categories.length > 0 && (
        <div className="admin-list-stack">
          {categories.map((c) => (
            <Panel key={c.id} className="admin-row-card">
              <div className="admin-row-card__body">
                <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '16px', fontWeight: 700, color: A.txt }}>{c.title}</span>
                  <span style={{ fontSize: '12px', color: A.txt3, fontFamily: 'monospace' }}>{c.id}</span>
                </div>
                {c.description && <div style={{ fontSize: '13px', color: A.txt2, marginTop: '4px', lineHeight: 1.5 }}>{c.description}</div>}
                {(c.seo_tags ?? []).length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', marginTop: '10px' }}>
                    {c.seo_tags.map((t) => <StatusBadge key={t} label={t} tone="cyan" />)}
                  </div>
                )}
              </div>
              <div className="admin-row-card__actions">
                <Button variant="outlined" size="sm" onClick={() => navigate(`/admin/categories/${c.id}`)}>Изменить</Button>
                <Button variant="text" size="sm" onClick={() => handleDelete(c.id)} style={{ color: '#f87171' }}>Удалить</Button>
              </div>
            </Panel>
          ))}
        </div>
      )}
    </div>
  )
}
