import { useEffect, useRef, useState } from 'react'
import { adminExampleApi } from '@entities/admin-example'
import type { AdminExample, ExamplePayload } from '@entities/admin-example'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, PageHeader, Panel, ErrorBanner, EmptyState, StatusBadge, Field } from '@widgets/admin-layout'

const EMPTY: ExamplePayload = {
  id: '', title: '', category: '', description: '', mood: '',
  audio_url: '', cover_url: '', sort_order: 0, is_active: true,
}

export function AdminExamplesPage() {
  const [examples, setExamples] = useState<AdminExample[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [form, setForm] = useState<ExamplePayload>(EMPTY)
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  function load() {
    setLoading(true)
    adminExampleApi.list()
      .then((data) => { setExamples(data); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  function set<K extends keyof ExamplePayload>(key: K, value: ExamplePayload[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  function openCreate() {
    setEditingId(null)
    setForm({ ...EMPTY, sort_order: examples.length + 1 })
    setFormError(null)
    setShowForm(true)
  }

  function openEdit(e: AdminExample) {
    setEditingId(e.id)
    setForm({ ...e })
    setFormError(null)
    setShowForm(true)
  }

  async function handleSubmit(ev: React.FormEvent) {
    ev.preventDefault()
    setFormError(null); setSubmitting(true)
    try {
      if (editingId != null) {
        await adminExampleApi.update(editingId, form)
      } else {
        await adminExampleApi.create(form)
      }
      setShowForm(false); load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось сохранить пример')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleCoverFile(ev: React.ChangeEvent<HTMLInputElement>) {
    const file = ev.target.files?.[0]
    ev.target.value = ''
    if (!file || !editingId) return
    setFormError(null); setUploading(true)
    try {
      const { cover_url } = await adminExampleApi.uploadCover(editingId, file)
      set('cover_url', cover_url)
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось загрузить обложку')
    } finally {
      setUploading(false)
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(`Удалить пример "${id}"?`)) return
    try {
      await adminExampleApi.remove(id)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить пример')
    }
  }

  return (
    <div style={{ maxWidth: 980 }}>
      <PageHeader
        title="Примеры работ"
        subtitle={`${examples.length} примеров · блок «Послушать примеры» на главной`}
        action={<Button variant={showForm ? 'outlined' : 'filled'} onClick={() => (showForm ? setShowForm(false) : openCreate())}>{showForm ? 'Отмена' : '+ Новый пример'}</Button>}
      />

      {showForm && (
        <Panel style={{ padding: '24px', marginBottom: '24px' }}>
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
              <TextField label="ID (слаг)" value={form.id ?? ''} onChange={(v) => set('id', v)} required={editingId == null} disabled={editingId != null} surfaceColor={A.surface} />
              <TextField label="Название" value={form.title} onChange={(v) => set('title', v)} required surfaceColor={A.surface} />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
              <TextField label="Категория (метка)" value={form.category} onChange={(v) => set('category', v)} placeholder="Юбилей" surfaceColor={A.surface} />
              <TextField label="Настроение" value={form.mood} onChange={(v) => set('mood', v)} placeholder="Праздник" surfaceColor={A.surface} />
            </div>
            <TextField label="Описание" value={form.description} onChange={(v) => set('description', v)} multiline rows={2} surfaceColor={A.surface} />
            <TextField label="URL аудио (mp3)" value={form.audio_url} onChange={(v) => set('audio_url', v)} placeholder="https://...mp3" surfaceColor={A.surface} />

            <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: '16px', alignItems: 'end' }}>
              <TextField label="URL обложки" value={form.cover_url} onChange={(v) => set('cover_url', v)} placeholder="https://...webp" surfaceColor={A.surface} />
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', paddingBottom: '2px' }}>
                {form.cover_url && (
                  <img src={form.cover_url} alt="" style={{ width: 56, height: 40, objectFit: 'cover', borderRadius: '8px', border: `1px solid ${A.border}` }} onError={(e) => { e.currentTarget.style.display = 'none' }} />
                )}
                <input ref={fileRef} type="file" accept="image/png,image/jpeg,image/webp" onChange={handleCoverFile} style={{ display: 'none' }} />
                <Button type="button" variant="outlined" size="sm" loading={uploading} disabled={editingId == null} onClick={() => fileRef.current?.click()}>
                  Загрузить
                </Button>
              </div>
            </div>
            {editingId == null && <div style={{ fontSize: '12px', color: A.txt3, marginTop: '-8px' }}>Загрузка обложки доступна после создания примера. URL аудио/обложки можно вставить вручную.</div>}

            <div style={{ display: 'grid', gridTemplateColumns: '160px 1fr', gap: '16px', alignItems: 'center' }}>
              <Field label="Порядок">
                <input type="number" value={form.sort_order} onChange={(e) => set('sort_order', Number(e.target.value))}
                  style={{ background: A.surface2, border: `1px solid ${A.border}`, borderRadius: '12px', padding: '12px 14px', color: A.txt, fontSize: '14px', fontFamily: 'inherit', outline: 'none', width: '100%' }} />
              </Field>
              <label style={{ display: 'flex', alignItems: 'center', gap: '9px', fontSize: '14px', color: A.txt2, cursor: 'pointer', marginTop: '22px' }}>
                <input type="checkbox" checked={form.is_active} onChange={(e) => set('is_active', e.target.checked)} style={{ width: 16, height: 16, accentColor: A.accent, cursor: 'pointer' }} />
                Показывать на главной
              </label>
            </div>

            {formError && <ErrorBanner>{formError}</ErrorBanner>}
            <Button type="submit" loading={submitting} style={{ alignSelf: 'flex-start' }}>{editingId != null ? 'Сохранить пример' : 'Создать пример'}</Button>
          </form>
        </Panel>
      )}

      {loading && <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>}
      {error && <ErrorBanner>{error}</ErrorBanner>}

      {!loading && examples.length === 0 && <Panel><EmptyState icon="🎧" text="Примеров пока нет — добавьте первый." /></Panel>}

      {!loading && examples.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {examples.map((e) => (
            <Panel key={e.id} style={{ padding: '14px 18px', display: 'flex', alignItems: 'center', gap: '16px' }}>
              <div style={{ width: 64, height: 44, borderRadius: '10px', overflow: 'hidden', flexShrink: 0, background: A.surface2, border: `1px solid ${A.border}`, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                {e.cover_url
                  ? <img src={e.cover_url} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} onError={(ev) => { ev.currentTarget.style.display = 'none' }} />
                  : <span style={{ fontSize: '18px', opacity: 0.5 }}>🎵</span>}
              </div>
              <div style={{ minWidth: 0, flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '15px', fontWeight: 700, color: A.txt }}>{e.title}</span>
                  <span style={{ fontSize: '12px', color: A.txt3, fontFamily: 'monospace' }}>#{e.sort_order}</span>
                  {!e.is_active && <StatusBadge label="Скрыт" tone="muted" />}
                </div>
                <div style={{ fontSize: '12px', color: A.txt2, marginTop: '4px', display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                  {e.category && <span>{e.category}</span>}
                  {e.mood && <span>· {e.mood}</span>}
                  {e.audio_url && <a href={e.audio_url} target="_blank" rel="noreferrer" style={{ color: A.accent, textDecoration: 'none' }}>🎧 аудио</a>}
                </div>
              </div>
              <div style={{ display: 'flex', gap: '8px', flexShrink: 0 }}>
                <Button variant="outlined" size="sm" onClick={() => openEdit(e)}>Изменить</Button>
                <Button variant="text" size="sm" onClick={() => handleDelete(e.id)} style={{ color: '#f87171' }}>Удалить</Button>
              </div>
            </Panel>
          ))}
        </div>
      )}
    </div>
  )
}
