import { Link } from 'react-router-dom'
import { theme } from '@shared/lib/theme'

const STEPS = [
  { icon: '🎯', title: 'Опишите повод', text: 'Категория или конструктор — 2 минуты' },
  { icon: '💳', title: 'Оплатите один раз', text: 'Без подписок, через Robokassa' },
  { icon: '🎧', title: 'Получите 4 версии', text: 'Готово примерно за 10 минут' },
] as const

export function HowItWorksStrip() {
  return (
    <section className="home-how-strip fade-in" aria-label="Как это работает">
      <div className="home-how-strip__grid">
        {STEPS.map((s) => (
          <div key={s.title} className="home-how-strip__item">
            <span className="home-how-strip__icon" aria-hidden>{s.icon}</span>
            <div className="home-how-strip__title">{s.title}</div>
            <div className="home-how-strip__text">{s.text}</div>
          </div>
        ))}
      </div>
      <div style={{ textAlign: 'center', marginTop: 14 }}>
        <Link
          to="/how-it-works"
          style={{ fontSize: 13, fontWeight: 600, color: theme.accent, textDecoration: 'none' }}
        >
          Подробнее о процессе →
        </Link>
      </div>
    </section>
  )
}
