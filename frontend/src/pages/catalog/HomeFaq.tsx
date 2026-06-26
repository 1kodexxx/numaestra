import { useEffect, useState } from 'react'

const FAQ = [
  {
    q: 'Сколько версий песни я получу?',
    a: 'Четыре уникальные версии одного трека — с разной подачей, аранжировкой и вокалом. Выбираете лучшую и скачиваете все.',
  },
  {
    q: 'Нужна ли регистрация?',
    a: 'Нет. Достаточно email или телефона при оформлении — на почту придут ссылки на готовые треки и страницу заказа.',
  },
  {
    q: 'Как быстро будет готово?',
    a: 'Обычно около 10 минут после оплаты. Статус можно отслеживать на странице заказа — она обновляется автоматически.',
  },
  {
    q: 'Безопасна ли оплата?',
    a: 'Да. Платёж проходит через Robokassa — один раз, без автопродлений и скрытых списаний.',
  },
] as const

export function HomeFaq({ priceLabel }: { priceLabel: string }) {
  const [open, setOpen] = useState<number | null>(0)

  useEffect(() => {
    injectJsonLd('ld-faq-home', {
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: FAQ.map((item) => ({
        '@type': 'Question',
        name: item.q,
        acceptedAnswer: { '@type': 'Answer', text: item.a },
      })),
    })
    return () => document.getElementById('ld-faq-home')?.remove()
  }, [])

  return (
    <section className="home-faq fade-in" style={{ marginTop: 40 }} aria-label="Частые вопросы">
      <div style={{ textAlign: 'center', marginBottom: 20 }}>
        <h2 style={{ fontSize: 20, fontWeight: 800, letterSpacing: '-0.02em', marginBottom: 6 }}>Частые вопросы</h2>
        <p style={{ fontSize: 13, color: theme.text2 }}>{priceLabel} · 4 версии · Robokassa · без подписок</p>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {FAQ.map((item, i) => {
          const isOpen = open === i
          return (
            <div
              key={item.q}
              style={{
                background: theme.surface,
                border: `1px solid ${isOpen ? 'rgba(0,229,192,0.28)' : theme.border}`,
                borderRadius: 14,
                overflow: 'hidden',
                transition: 'border-color 0.2s',
              }}
            >
              <button
                type="button"
                onClick={() => setOpen(isOpen ? null : i)}
                aria-expanded={isOpen}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 12,
                  padding: '14px 16px',
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  fontFamily: 'inherit',
                  fontSize: 14,
                  fontWeight: 600,
                  color: '#fff',
                  textAlign: 'left',
                }}
              >
                {item.q}
                <span style={{ color: theme.accent, fontSize: 18, lineHeight: 1, flexShrink: 0 }}>{isOpen ? '−' : '+'}</span>
              </button>
              {isOpen && (
                <div style={{ padding: '0 16px 14px', fontSize: 13, color: theme.text2, lineHeight: 1.6 }}>
                  {item.a}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </section>
  )
}
