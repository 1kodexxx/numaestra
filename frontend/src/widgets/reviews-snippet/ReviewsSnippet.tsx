import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { reviewApi } from '@entities/review'
import type { PublicReview } from '@entities/review'
import { theme } from '@shared/lib/theme'

function Stars({ value }: { value: number }) {
  return (
    <span aria-label={`Оценка ${value} из 5`} style={{ letterSpacing: '1px', color: theme.gold, fontSize: 14 }}>
      {'★'.repeat(value)}{'☆'.repeat(5 - value)}
    </span>
  )
}

export function ReviewsSnippet() {
  const [reviews, setReviews] = useState<PublicReview[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    reviewApi
      .list(1, 4)
      .then((data) => setReviews(data.reviews.slice(0, 3)))
      .catch(() => setReviews([]))
      .finally(() => setLoading(false))
  }, [])

  if (!loading && reviews.length === 0) return null

  return (
    <section className="fade-in" style={{ marginTop: 40 }}>
      <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 12, marginBottom: 16 }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 800, letterSpacing: '-0.02em', color: '#fff' }}>⭐ Отзывы клиентов</div>
          <div style={{ fontSize: 12, color: theme.text3, marginTop: 4 }}>Реальные истории после заказа</div>
        </div>
        <Link to="/reviews" style={{ fontSize: 13, fontWeight: 600, color: theme.accent, textDecoration: 'none' }}>
          Все отзывы →
        </Link>
      </div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
          gap: 12,
        }}
      >
        {loading
          ? Array.from({ length: 3 }, (_, i) => (
              <div
                key={i}
                className="skeleton"
                style={{ height: 120, borderRadius: 16, background: 'rgba(255,255,255,0.04)' }}
              />
            ))
          : reviews.map((r) => (
              <article
                key={r.id}
                className="interactive-card"
                style={{
                  background: theme.surface,
                  border: `1px solid ${theme.border}`,
                  borderRadius: 16,
                  padding: '16px 18px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                  <span style={{ fontSize: 14, fontWeight: 700 }}>{r.author_name}</span>
                  <Stars value={r.rating} />
                </div>
                <p
                  style={{
                    fontSize: 13,
                    color: theme.text2,
                    lineHeight: 1.55,
                    margin: 0,
                    display: '-webkit-box',
                    WebkitLineClamp: 3,
                    WebkitBoxOrient: 'vertical',
                    overflow: 'hidden',
                  }}
                >
                  {r.body}
                </p>
              </article>
            ))}
      </div>
    </section>
  )
}
