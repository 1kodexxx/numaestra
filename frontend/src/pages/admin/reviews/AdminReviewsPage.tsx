import { useEffect, useState } from 'react'
import { adminReviewApi } from '@entities/admin-review'
import type { AdminReview } from '@entities/admin-review'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, PageHeader, Panel, ErrorBanner, EmptyState, StatusBadge } from '@widgets/admin-layout'

function Stars({ value }: { value: number }) {
  return (
    <span style={{ letterSpacing: '1px', fontSize: '13px' }} aria-label={`${value} из 5`}>
      {[1, 2, 3, 4, 5].map((n) => (
        <span key={n} style={{ color: n <= value ? '#fbbf24' : 'rgba(255,255,255,0.18)' }}>★</span>
      ))}
    </span>
  )
}

export function AdminReviewsPage() {
  const [reviews, setReviews] = useState<AdminReview[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  function load() {
    setLoading(true)
    adminReviewApi
      .list()
      .then((data) => { setReviews(data); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  async function act(fn: () => Promise<unknown>) {
    try {
      await fn()
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Действие не удалось')
    }
  }

  const visible = reviews.filter((r) => r.is_published).length

  return (
    <div style={{ maxWidth: 880 }}>
      <PageHeader
        title="Отзывы"
        subtitle={loading ? 'Загрузка…' : `${reviews.length} всего · ${visible} опубликовано`}
      />

      {loading && <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>}
      {error && <ErrorBanner>{error}</ErrorBanner>}

      {!loading && reviews.length === 0 && <Panel><EmptyState icon="💬" text="Отзывов пока нет." /></Panel>}

      {!loading && reviews.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          {reviews.map((r) => (
            <ReviewCard key={r.id} review={r} onAct={act} />
          ))}
        </div>
      )}
    </div>
  )
}

function ReviewCard({ review, onAct }: { review: AdminReview; onAct: (fn: () => Promise<unknown>) => Promise<void> }) {
  const [replyOpen, setReplyOpen] = useState(false)
  const [reply, setReply] = useState(review.admin_reply)
  const [saving, setSaving] = useState(false)

  async function saveReply() {
    setSaving(true)
    await onAct(() => adminReviewApi.reply(review.id, reply))
    setSaving(false)
    setReplyOpen(false)
  }

  return (
    <Panel style={{ padding: '18px 20px', opacity: review.is_published ? 1 : 0.6 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
          <span style={{ fontSize: '15px', fontWeight: 700, color: A.txt, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{review.author_name}</span>
          <Stars value={review.rating} />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
          {review.is_published
            ? <StatusBadge label="Опубликован" tone="green" dot />
            : <StatusBadge label="Скрыт" tone="muted" />}
          <span style={{ fontSize: '12px', color: A.txt3 }}>{new Date(review.created_at).toLocaleDateString('ru-RU')}</span>
        </div>
      </div>

      <div style={{ fontSize: '14px', color: A.txt, lineHeight: 1.6, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{review.body}</div>

      {review.admin_reply && !replyOpen && (
        <div style={{ marginTop: '10px', paddingLeft: '12px', borderLeft: `2px solid ${A.accent}`, fontSize: '13px', color: A.txt2, lineHeight: 1.5 }}>
          <span style={{ color: A.accent, fontWeight: 600 }}>Ваш ответ:</span> {review.admin_reply}
        </div>
      )}

      {replyOpen && (
        <div style={{ marginTop: '14px' }}>
          <TextField label="Ответ на отзыв" value={reply} onChange={setReply} multiline rows={3} surfaceColor={A.surface} />
          <div style={{ display: 'flex', gap: '8px', marginTop: '10px' }}>
            <Button size="sm" loading={saving} onClick={saveReply}>Сохранить ответ</Button>
            <Button size="sm" variant="text" onClick={() => { setReply(review.admin_reply); setReplyOpen(false) }}>Отмена</Button>
          </div>
        </div>
      )}

      <div style={{ display: 'flex', gap: '8px', marginTop: '14px', flexWrap: 'wrap' }}>
        {!replyOpen && (
          <Button variant="outlined" size="sm" onClick={() => setReplyOpen(true)}>
            {review.admin_reply ? 'Изменить ответ' : 'Ответить'}
          </Button>
        )}
        <Button variant="outlined" size="sm" onClick={() => onAct(() => adminReviewApi.setPublished(review.id, !review.is_published))}>
          {review.is_published ? 'Скрыть' : 'Показать'}
        </Button>
        <Button
          variant="text" size="sm"
          onClick={() => { if (confirm('Удалить отзыв безвозвратно?')) onAct(() => adminReviewApi.remove(review.id)) }}
          style={{ color: '#f87171' }}
        >
          Удалить
        </Button>
      </div>
    </Panel>
  )
}
