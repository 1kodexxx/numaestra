import { useEffect, useState } from 'react'
import { adminGenreApi } from '@entities/admin-genre'
import type { AdminGenre, GenrePayload } from '@entities/admin-genre'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, PageHeader, Panel, Grid2, ErrorBanner, EmptyState, StatusBadge, Field } from '@widgets/admin-layout'

const EMPTY: GenrePayload = { slug: '', label: '', suno_value: '', sort_order: 0, is_active: true }

export function AdminGenresPage() {
  const [genres, setGenres] = useState<AdminGenre[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<GenrePayload>(EMPTY)
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function load() {
    setLoading(true)
    adminGenreApi.list()
      .then((data) => { setGenres(data); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  function set<K extends keyof GenrePayload>(key: K, value: GenrePayload[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  function openCreate() {
    setEditingId(null)
    setForm({ ...EMPTY, sort_order: (genres.length + 1) * 10 })
    setFormError(null)
    setShowForm(true)
  }

  function openEdit(g: AdminGenre) {
    setEditingId(g.id)
    setForm({ slug: g.slug, label: g.label, suno_value: g.suno_value, sort_order: g.sort_order, is_active: g.is_active })
    setFormError(null)
    setShowForm(true)
  }

  async function handleSubmit(ev: React.FormEvent) {
    ev.preventDefault()
    setFormError(null); setSubmitting(true)
    try {
      if (editingId != null) {
        await adminGenreApi.update(editingId, {
          label: form.label,
          suno_value: form.suno_value,
          sort_order: form.sort_order,
          is_active: form.is_active ?? true,
        })
      } else {
        await adminGenreApi.create(form)
      }
      setShowForm(false); load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось сохранить жанр')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) return <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>
  if (error) return <ErrorBanner>{error}</ErrorBanner>

  return (
    <div className="admin-page">
      <PageHeader
        title="Жанры"
        subtitle="Справочник музыкальных жанров для квизов и конструктора"
        action={<Button variant="tonal" size="sm" onClick={openCreate}>+ Жанр</Button>}
      />

      {showForm && (
        <Panel style={{ padding: '22px', marginBottom: '16px' }}>
          <form onSubmit={handleSubmit} className="admin-form-stack admin-form-stack--sm">
            <Grid2>
              <TextField label="Slug (латиница)" value={form.slug} onChange={(v) => set('slug', v)} required disabled={editingId != null} surfaceColor={A.surface} />
              <TextField label="Название" value={form.label} onChange={(v) => set('label', v)} required surfaceColor={A.surface} />
            </Grid2>
            <Grid2>
              <TextField label="Suno value (англ. промпт)" value={form.suno_value} onChange={(v) => set('suno_value', v)} required surfaceColor={A.surface} />
              <Field label="Порядок">
                <input type="number" value={form.sort_order} onChange={(e) => set('sort_order', Number(e.target.value))}
                  style={{ background: A.surface2, border: `1px solid ${A.border}`, borderRadius: '12px', padding: '12px 14px', color: A.txt, fontSize: '14px', fontFamily: 'inherit', outline: 'none', width: '100%' }} />
              </Field>
            </Grid2>
            {editingId != null && (
              <label style={{ display: 'flex', alignItems: 'center', gap: '9px', fontSize: '14px', color: A.txt2, cursor: 'pointer' }}>
                <input type="checkbox" checked={form.is_active ?? true} onChange={(e) => set('is_active', e.target.checked)} style={{ width: 16, height: 16, accentColor: A.accent }} />
                Активен
              </label>
            )}
            {formError && <ErrorBanner>{formError}</ErrorBanner>}
            <div style={{ display: 'flex', gap: '10px' }}>
              <Button type="submit" loading={submitting}>{editingId != null ? 'Сохранить' : 'Добавить'}</Button>
              <Button type="button" variant="outlined" onClick={() => setShowForm(false)}>Отмена</Button>
            </div>
          </form>
        </Panel>
      )}

      {genres.length === 0
        ? <Panel><EmptyState icon="🎸" text="Жанров пока нет." /></Panel>
        : (
          <div className="admin-list-stack">
            {genres.map((g) => (
              <Panel key={g.id} className="admin-row-card">
                <div className="admin-row-card__body">
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span style={{ fontSize: '14px', fontWeight: 600, color: A.txt }}>{g.label}</span>
                    {!g.is_active && <StatusBadge label="скрыт" tone="muted" />}
                  </div>
                  <div style={{ fontSize: '12px', color: A.txt3, marginTop: '6px', fontFamily: 'monospace' }}>
                    {g.slug} · {g.suno_value}
                  </div>
                </div>
                <div className="admin-row-card__actions">
                  <Button variant="outlined" size="sm" onClick={() => openEdit(g)}>Изменить</Button>
                  <Button variant="text" size="sm" onClick={async () => {
                    if (!confirm(`Удалить жанр «${g.label}»?`)) return
                    await adminGenreApi.remove(g.id)
                    load()
                  }} style={{ color: '#f87171' }}>Удалить</Button>
                </div>
              </Panel>
            ))}
          </div>
        )}
    </div>
  )
}
