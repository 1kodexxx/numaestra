import { useEffect, useMemo, useState } from 'react'
import { reviewApi } from '@entities/review'
import type { PublicReview } from '@entities/review'
import { Button, TextField, Spinner } from '@shared/ui'
import { ApiError } from '@shared/api'
import { useSeo } from '@shared/lib/seo'
import { useScrollReveal } from '@shared/lib/useScrollReveal'

const ACCENT = '#00e5c0'
const BORDER = 'rgba(255,255,255,0.08)'
const SURFACE = '#0f0f0f'
const TEXT2 = 'rgba(255,255,255,0.55)'
const TEXT3 = 'rgba(255,255,255,0.32)'

/* ─── звёзды ─── */
function Stars({ value, onChange, size = 18 }: { value: number; onChange?: (v: number) => void; size?: number }) {
  const interactive = !!onChange
  return (
    <div style={{ display: 'inline-flex', gap: '2px' }} role={interactive ? 'radiogroup' : undefined} aria-label="Оценка">
      {[1, 2, 3, 4, 5].map((n) => (
        <span
          key={n}
          onClick={interactive ? () => onChange!(n) : undefined}
          role={interactive ? 'radio' : undefined}
          aria-checked={interactive ? n === value : undefined}
          aria-label={interactive ? `${n} из 5` : undefined}
          className={interactive ? 'star-interactive' : undefined}
          style={{
            fontSize: `${size}px`,
            lineHeight: 1,
            cursor: interactive ? 'pointer' : 'default',
            color: n <= value ? '#fbbf24' : 'rgba(255,255,255,0.18)',
            transition: 'color 0.12s',
          }}
        >
          ★
        </span>
      ))}
    </div>
  )
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
}

export function ReviewsPage() {
  const [reviews, setReviews] = useState<PublicReview[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [name, setName] = useState('')
  const [rating, setRating] = useState(5)
  const [body, setBody] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [sent, setSent] = useState(false)
  const revealRef = useScrollReveal<HTMLDivElement>()

  useSeo({
    title: 'Отзывы о Numaestra — что говорят клиенты',
    description: 'Реальные отзывы людей, заказавших персональную песню в Numaestra. Оставьте свой отзыв — без регистрации.',
  })

  function load() {
    setLoading(true)
    reviewApi
      .list(1, 50)
      .then((data) => { setReviews(data.reviews); setTotal(data.total); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  const avg = useMemo(() => {
    if (reviews.length === 0) return 0
    return reviews.reduce((s, r) => s + r.rating, 0) / reviews.length
  }, [reviews])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null); setSubmitting(true)
    try {
      const created = await reviewApi.create({ author_name: name.trim(), rating, body: body.trim() })
      setReviews((prev) => [created, ...prev])
      setTotal((t) => t + 1)
      setName(''); setBody(''); setRating(5); setSent(true)
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось отправить отзыв')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <div ref={revealRef} style={{ maxWidth: 760, margin: '0 auto', padding: '40px 20px 0' }} className="fade-in">
        {/* Hero */}
        <div style={{ textAlign: 'center', marginBottom: '32px' }} className="hero-enter">
          <h1 style={{ fontSize: 'clamp(26px, 5vw, 38px)', fontWeight: 800, letterSpacing: '-0.03em', marginBottom: '10px' }}>
            Отзывы о <span className="gradient-text-flow">Numaestra</span>
          </h1>
          <p style={{ fontSize: '15px', color: TEXT2, lineHeight: 1.6, maxWidth: 460, margin: '0 auto' }}>
            Что говорят люди, заказавшие персональную песню. Поделитесь и вы — регистрация не нужна.
          </p>
          {total > 0 && (
            <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', marginTop: '16px' }}>
              <Stars value={Math.round(avg)} size={20} />
              <span style={{ fontSize: '14px', color: TEXT2 }}>
                <b style={{ color: '#fff' }}>{avg.toFixed(1)}</b> · {total} отзыв{plural(total)}
              </span>
            </div>
          )}
        </div>

        {/* Форма */}
        <div className="fade-up hero-enter-d1" style={{ background: SURFACE, border: `1px solid ${BORDER}`, borderRadius: '18px', padding: '24px', marginBottom: '36px' }}>
          <div style={{ fontSize: '16px', fontWeight: 700, marginBottom: '18px' }}>Оставить отзыв</div>
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <TextField label="Ваше имя" value={name} onChange={(v) => { setName(v); setSent(false) }} required surfaceColor={SURFACE} />
            <div>
              <div style={{ fontSize: '12px', color: TEXT2, fontWeight: 500, marginBottom: '8px' }}>Оценка</div>
              <Stars value={rating} onChange={(v) => { setRating(v); setSent(false) }} size={28} />
            </div>
            <TextField
              label="Ваш отзыв"
              value={body}
              onChange={(v) => { setBody(v); setSent(false) }}
              multiline rows={4} required
              placeholder="Расскажите о вашем опыте: какой повод, понравился ли результат…"
              surfaceColor={SURFACE}
            />
            {formError && (
              <div style={{ fontSize: '13px', color: '#f87171', background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.25)', borderRadius: '10px', padding: '10px 14px' }}>
                {formError}
              </div>
            )}
            {sent && (
              <div className="pop-in" style={{ fontSize: '13px', color: '#4ade80', background: 'rgba(34,197,94,0.08)', border: '1px solid rgba(34,197,94,0.25)', borderRadius: '10px', padding: '10px 14px' }}>
                Спасибо! Ваш отзыв опубликован.
              </div>
            )}
            <Button type="submit" loading={submitting} disabled={!name.trim() || !body.trim()} style={{ alignSelf: 'flex-start' }}>
              Отправить отзыв
            </Button>
          </form>
        </div>

        {/* Список */}
        {loading && <div style={{ padding: '40px', textAlign: 'center' }}><Spinner /></div>}
        {error && <div style={{ fontSize: '14px', color: '#f87171', textAlign: 'center', padding: '20px' }}>{error}</div>}

        {!loading && !error && reviews.length === 0 && (
          <div style={{ textAlign: 'center', padding: '40px 20px', color: TEXT2 }}>
            <div style={{ fontSize: '36px', marginBottom: '10px', opacity: 0.85 }}>💬</div>
            <div style={{ fontSize: '14px' }}>Пока нет отзывов — станьте первым!</div>
          </div>
        )}

        {!loading && reviews.length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            {reviews.map((r) => (
              <article key={r.id} data-reveal className="reveal reveal-up interactive-card glow-hover" style={{ background: SURFACE, border: `1px solid ${BORDER}`, borderRadius: '16px', padding: '18px 20px' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '8px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
                    <span style={{ fontSize: '15px', fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.author_name}</span>
                    <Stars value={r.rating} size={14} />
                  </div>
                  <span style={{ fontSize: '12px', color: TEXT3, flexShrink: 0 }}>{formatDate(r.created_at)}</span>
                </div>
                <div style={{ fontSize: '14px', color: 'rgba(255,255,255,0.82)', lineHeight: 1.6, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{r.body}</div>
                {r.admin_reply && (
                  <div style={{ marginTop: '12px', paddingLeft: '14px', borderLeft: `2px solid ${ACCENT}`, background: 'rgba(0,229,192,0.04)', borderRadius: '0 10px 10px 0', padding: '10px 14px' }}>
                    <div style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.04em', textTransform: 'uppercase', marginBottom: '4px' }}>Ответ Numaestra</div>
                    <div style={{ fontSize: '13px', color: 'rgba(255,255,255,0.75)', lineHeight: 1.55, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{r.admin_reply}</div>
                  </div>
                )}
              </article>
            ))}
          </div>
        )}
      </div>
    </>
  )
}

function plural(n: number): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return ''
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return 'а'
  return 'ов'
}
