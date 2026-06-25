import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@shared/ui'
import { useSeo } from '@shared/lib/seo'
import { usePublicConfig } from '@shared/lib/usePublicConfig'
import { useScrollReveal } from '@shared/lib/useScrollReveal'

const ACCENT = '#00e5c0'
const BORDER = 'rgba(255,255,255,0.08)'
const SURFACE = '#0f0f0f'
const TEXT2 = 'rgba(255,255,255,0.55)'
const TEXT3 = 'rgba(255,255,255,0.32)'

interface Step {
  icon: string
  title: string
  text: string
}

const STEPS: Step[] = [
  {
    icon: '🎯',
    title: 'Выберите формат песни',
    text: 'На главной — два пути: нажмите готовую категорию (свадьба, юбилей, признание…) или откройте конструктор и соберите свой запрос: жанр, настроение, темп, вокал.',
  },
  {
    icon: '📝',
    title: 'Опишите повод и детали',
    text: 'Ответьте на короткие вопросы квиза или впишите детали в свободной форме: имена, историю, важные слова. Чем точнее бриф — тем личнее получится песня.',
  },
  {
    icon: '✉️',
    title: 'Оставьте контакты',
    text: 'Укажите email (и при желании телефон) и подтвердите согласие с офертой. Регистрация не нужна — на почту придёт ссылка на готовый трек.',
  },
  {
    icon: '💳',
    title: 'Оплатите заказ',
    text: 'Фиксированная цена за 4 версии песни, без подписок и доплат. Оплата проходит безопасно через Robokassa; сумму определяет сервер.',
  },
  {
    icon: '🤖',
    title: 'Нейросеть создаёт песню',
    text: 'После оплаты запускается генерация: ИИ пишет текст под ваш повод, подбирает мелодию, аранжировку и вокал — и выдаёт сразу 4 уникальные версии трека.',
  },
  {
    icon: '🎧',
    title: 'Получите результат за ~10 минут',
    text: 'Готовые треки появятся на странице заказа и придут на email: слушайте все 4 версии в плеере, скачивайте и делитесь. Ссылки остаются у вас навсегда.',
  },
]

export function HowItWorksPage() {
  const navigate = useNavigate()
  const { price_label } = usePublicConfig()
  const revealRef = useScrollReveal<HTMLDivElement>()

  const steps = useMemo(
    () =>
      STEPS.map((s, i) =>
        i === 3 ? { ...s, text: `Фиксированная цена — ${price_label} за 4 версии песни, без подписок и доплат. Оплата проходит безопасно через Robokassa; сумму определяет сервер.` } : s,
      ),
    [price_label],
  )

  useSeo({
    title: 'Как это работает — заказать песню за 6 шагов | Numaestra',
    description:
      `Пошаговый процесс заказа персональной песни в Numaestra: от выбора повода до 4 готовых версий трека за 10 минут. Без регистрации, один платёж ${price_label}.`,
  })

  return (
    <>
      <div ref={revealRef} style={{ maxWidth: 720, margin: '0 auto', padding: '44px 20px 0' }} className="fade-in">
        {/* Hero */}
        <div style={{ textAlign: 'center', marginBottom: '40px' }} className="hero-enter">
          <div
            style={{
              display: 'inline-flex', alignItems: 'center', gap: '7px',
              background: 'rgba(0,229,192,0.08)', border: '1px solid rgba(0,229,192,0.18)',
              borderRadius: '20px', padding: '5px 13px', marginBottom: '16px',
            }}
          >
            <span style={{ fontSize: '12px' }}>🎙️</span>
            <span style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
              Как это работает
            </span>
          </div>
          <h1 style={{ fontSize: 'clamp(26px, 5vw, 40px)', fontWeight: 800, letterSpacing: '-0.03em', lineHeight: 1.1, marginBottom: '12px' }}>
            От идеи до готовой песни — <span className="gradient-text-flow">за 6 шагов</span>
          </h1>
          <p style={{ fontSize: '15px', color: TEXT2, lineHeight: 1.6, maxWidth: 480, margin: '0 auto' }}>
            Весь процесс занимает около 10 минут. Регистрация не нужна, оплата один раз — {price_label} за 4 версии трека.
          </p>
        </div>

        {/* Алгоритм */}
        <ol style={{ listStyle: 'none', padding: 0, margin: 0, display: 'flex', flexDirection: 'column', gap: '14px' }}>
          {steps.map((s, i) => (
            <li
              key={i}
              data-reveal
              className="reveal reveal-up interactive-card glow-hover"
              style={{
                display: 'flex', gap: '16px', alignItems: 'flex-start',
                background: SURFACE, border: `1px solid ${BORDER}`, borderRadius: '16px', padding: '18px 20px',
                '--reveal-delay': `${Math.min(i * 80, 480)}ms`,
              } as React.CSSProperties}
            >
              {/* Номер шага */}
              <div
                aria-hidden
                style={{
                  flexShrink: 0, width: 38, height: 38, borderRadius: '50%',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  background: 'linear-gradient(135deg, rgba(0,229,192,0.18), rgba(0,191,165,0.06))',
                  border: '1px solid rgba(0,229,192,0.3)',
                  fontSize: '15px', fontWeight: 800, color: ACCENT,
                }}
              >
                {i + 1}
              </div>
              <div style={{ minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
                  <span style={{ fontSize: '17px', lineHeight: 1 }}>{s.icon}</span>
                  <h2 style={{ fontSize: '16px', fontWeight: 700, letterSpacing: '-0.01em' }}>{s.title}</h2>
                </div>
                <p style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.6 }}>{s.text}</p>
              </div>
            </li>
          ))}
        </ol>

        {/* CTA */}
        <div data-reveal className="reveal reveal-up" style={{ textAlign: 'center', margin: '40px 0 8px' }}>
          <div style={{ fontSize: '15px', fontWeight: 700, marginBottom: '14px' }}>Готовы начать?</div>
          <Button size="lg" onClick={() => navigate('/')}>Заказать песню →</Button>
          <div style={{ fontSize: '12px', color: TEXT3, marginTop: '12px' }}>
            4 версии трека · готово за 10 минут · {price_label} без подписок
          </div>
        </div>
      </div>
    </>
  )
}
