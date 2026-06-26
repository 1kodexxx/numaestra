import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@shared/ui'
import { useSeo } from '@shared/lib/seo'
import { usePublicConfig } from '@shared/lib/usePublicConfig'
import { useScrollReveal } from '@shared/lib/useScrollReveal'
import { theme } from '@shared/lib/theme'

const ACCENT = theme.accent
const ACCENT2 = theme.accent2
const BORDER = theme.border
const TEXT2 = 'rgba(255,255,255,0.55)'
const TEXT3 = 'rgba(255,255,255,0.32)'

/* ─── актуальный demo-first маршрут ─── */
interface Step {
  icon: string
  title: string
  text: string
}

const STEPS: Step[] = [
  {
    icon: '🎯',
    title: 'Соберите свой запрос',
    text: 'Нажмите готовую категорию (свадьба, юбилей, признание…) или откройте конструктор: жанр, настроение, темп, вокал. Можно добавить свои жанры — до трёх.',
  },
  {
    icon: '📝',
    title: 'Опишите повод и детали',
    text: 'Короткий квиз или свободный текст: имена, история, важные слова. Чем точнее бриф — тем личнее песня. Текст пишет нейросеть, вам не нужно ничего сочинять.',
  },
  {
    icon: '✉️',
    title: 'Оставьте email',
    text: 'Без регистрации. На почту придёт ссылка на готовый трек, а заказ откроется на странице статуса — там же появится демо.',
  },
  {
    icon: '🎧',
    title: 'Послушайте демо бесплатно',
    text: 'Через пару минут на странице заказа соберётся бесплатный фрагмент — самый яркий момент вашей будущей песни. Платите, только если понравилось.',
  },
  {
    icon: '💳',
    title: 'Понравилось — оплатите',
    text: 'Один платёж, без подписок и доплат. Безопасно через Robokassa; сумму определяет сервер. После оплаты запускается полная генерация.',
  },
  {
    icon: '✨',
    title: 'Получите 4 версии за ~10 минут',
    text: 'Нейросеть выдаёт сразу 4 уникальные версии — без водяного знака, в полном качестве. Слушайте в плеере, скачивайте, делитесь. Ссылки остаются у вас навсегда.',
  },
]

/* ─── технологии под капотом ─── */
const TECH: { icon: string; title: string; text: string }[] = [
  { icon: '🆓', title: 'Демо до оплаты', text: 'Бесплатный фрагмент собирается ещё до платежа — вы слышите результат заранее и не покупаете «вслепую».' },
  { icon: '🧠', title: 'AI-студия нового поколения', text: 'Песню целиком — текст, мелодию, аранжировку и вокал — создаёт передовая нейросеть. Без шаблонов: каждый трек уникален.' },
  { icon: '🎵', title: '4 версии за один заказ', text: 'Два разных текста × две музыкальные интерпретации. Выбираете ту, что попала в сердце.' },
  { icon: '🇷🇺', title: 'Живой русский вокал', text: 'Вокал и слова на русском, ровно под ваш повод, настроение и жанр.' },
  { icon: '🔒', title: 'Безопасная оплата', text: 'Robokassa и фиксированная серверная цена. Никаких скрытых списаний и подписок.' },
  { icon: '♾️', title: 'Треки навсегда', text: 'Готовые версии хранятся в облаке: скачивайте в любой момент и делитесь ссылкой с близкими.' },
]

function MetricChip({ icon, label }: { icon: string; label: string }) {
  return (
    <div style={{
      display: 'inline-flex', alignItems: 'center', gap: '7px',
      background: 'rgba(255,255,255,0.04)', border: `1px solid ${BORDER}`,
      borderRadius: '999px', padding: '7px 14px',
    }}>
      <span style={{ fontSize: '13px' }}>{icon}</span>
      <span style={{ fontSize: '12.5px', fontWeight: 600, color: 'rgba(255,255,255,0.85)' }}>{label}</span>
    </div>
  )
}

function SectionTitle({ kicker, title }: { kicker: string; title: string }) {
  return (
    <div style={{ textAlign: 'center', marginBottom: '24px' }}>
      <div style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: '8px' }}>
        {kicker}
      </div>
      <h2 style={{ fontSize: 'clamp(20px, 3.6vw, 26px)', fontWeight: 800, letterSpacing: '-0.02em' }}>{title}</h2>
    </div>
  )
}

export function HowItWorksPage() {
  const navigate = useNavigate()
  const { price_label } = usePublicConfig()
  const revealRef = useScrollReveal<HTMLDivElement>()

  const steps = useMemo(
    () =>
      STEPS.map((s, i) =>
        i === 4
          ? { ...s, text: `Один платёж — ${price_label}, без подписок и доплат. Безопасно через Robokassa; сумму определяет сервер. После оплаты запускается полная генерация.` }
          : s,
      ),
    [price_label],
  )

  useSeo({
    title: 'Как это работает — демо бесплатно, песня за 10 минут | Numaestra',
    description:
      `Как Numaestra создаёт персональную песню: соберите запрос, послушайте бесплатное демо и оплатите, только если понравилось. 4 версии трека за ~10 минут, один платёж ${price_label}, без регистрации.`,
  })

  return (
    <div ref={revealRef} style={{ maxWidth: 760, margin: '0 auto', padding: '44px 20px 0' }} className="fade-in">
      {/* ═══ Hero ═══ */}
      <div style={{ textAlign: 'center', marginBottom: '36px' }} className="hero-enter">
        <div
          style={{
            display: 'inline-flex', alignItems: 'center', gap: '7px',
            background: 'rgba(0,229,192,0.08)', border: '1px solid rgba(0,229,192,0.2)',
            borderRadius: '20px', padding: '5px 13px', marginBottom: '18px',
          }}
        >
          <span style={{ fontSize: '12px' }}>🎙️</span>
          <span style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            Как это работает
          </span>
        </div>
        <h1 style={{ fontSize: 'clamp(27px, 5.4vw, 42px)', fontWeight: 800, letterSpacing: '-0.03em', lineHeight: 1.08, marginBottom: '14px' }}>
          Сначала послушайте,<br /><span className="gradient-text-flow">потом платите</span>
        </h1>
        <p style={{ fontSize: '15px', color: TEXT2, lineHeight: 1.6, maxWidth: 500, margin: '0 auto 22px' }}>
          Соберите запрос, получите бесплатное демо своей будущей песни и оплатите, только если понравилось. Полные 4 версии — за ~10 минут.
        </p>

        {/* декоративный эквалайзер */}
        <div className="hero-enter-d1" style={{ display: 'flex', justifyContent: 'center', alignItems: 'flex-end', gap: '4px', height: '34px', marginBottom: '22px' }}>
          {Array.from({ length: 17 }, (_, i) => (
            <span
              key={i}
              className="wave-animate"
              style={{
                width: '4px',
                height: `${35 + ((i * 41) % 60)}%`,
                borderRadius: '3px',
                transformOrigin: 'bottom',
                background: `linear-gradient(${ACCENT}, ${ACCENT2})`,
                boxShadow: '0 0 8px rgba(0,229,192,0.3)',
                animationDelay: `${i * 0.07}s`,
                animationDuration: `${0.7 + (i % 5) * 0.1}s`,
                opacity: 0.9,
              }}
            />
          ))}
        </div>

        <div className="hero-enter-d2" style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: '8px' }}>
          <MetricChip icon="🆓" label="Демо бесплатно" />
          <MetricChip icon="⚡" label="~10 минут" />
          <MetricChip icon="🎵" label="4 версии" />
          <MetricChip icon="🔓" label="Без регистрации" />
        </div>
      </div>

      {/* ═══ Demo-first баннер ═══ */}
      <div
        data-reveal
        className="reveal reveal-scale"
        style={{
          position: 'relative', overflow: 'hidden',
          borderRadius: '20px', padding: '22px 24px', marginBottom: '44px',
          background: 'linear-gradient(135deg, rgba(0,229,192,0.12), rgba(0,191,165,0.03))',
          border: '1px solid rgba(0,229,192,0.28)',
        }}
      >
        <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none', background: 'radial-gradient(ellipse 60% 80% at 100% 0%, rgba(0,229,192,0.12) 0%, transparent 70%)' }} />
        <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: '16px', flexWrap: 'wrap' }}>
          <div className="float-y" style={{ fontSize: '34px', flexShrink: 0 }}>🎧</div>
          <div style={{ flex: 1, minWidth: 220 }}>
            <div style={{ fontSize: '16px', fontWeight: 800, marginBottom: '4px' }}>Никакого риска для вас</div>
            <div style={{ fontSize: '13.5px', color: TEXT2, lineHeight: 1.55 }}>
              Демо-фрагмент собирается <b style={{ color: '#fff' }}>до оплаты</b>. Не понравилось — просто не платите. Понравилось — получаете полную песню без водяного знака.
            </div>
          </div>
        </div>
      </div>

      {/* ═══ Алгоритм ═══ */}
      <SectionTitle kicker="Маршрут" title="От идеи до песни — 6 шагов" />
      <ol className="how-steps-timeline" style={{ marginBottom: '52px' }}>
        {steps.map((s, i) => (
          <li
            key={i}
            data-reveal
            className="how-step-item reveal reveal-up interactive-card glow-hover"
            style={{
              display: 'flex', gap: '16px', alignItems: 'flex-start',
              background: 'linear-gradient(160deg, #101513 0%, #0c0c0c 60%)',
              border: `1px solid ${BORDER}`, borderRadius: '16px', padding: '18px 20px',
              '--reveal-delay': `${Math.min(i * 70, 420)}ms`,
            } as React.CSSProperties}
          >
            <div
              aria-hidden
              style={{
                flexShrink: 0, width: 40, height: 40, borderRadius: '50%',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                background: 'linear-gradient(135deg, rgba(0,229,192,0.22), rgba(0,191,165,0.05))',
                border: '1px solid rgba(0,229,192,0.35)',
                fontSize: '15px', fontWeight: 800, color: ACCENT,
                boxShadow: '0 4px 14px -6px rgba(0,229,192,0.5)',
              }}
            >
              {i + 1}
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '18px', lineHeight: 1 }}>{s.icon}</span>
                <h3 style={{ fontSize: '16px', fontWeight: 700, letterSpacing: '-0.01em' }}>{s.title}</h3>
                {i === 3 && (
                  <span style={{ fontSize: '9.5px', fontWeight: 800, letterSpacing: '0.06em', textTransform: 'uppercase', color: '#04130f', background: `linear-gradient(135deg, ${ACCENT}, ${ACCENT2})`, borderRadius: '5px', padding: '2px 7px' }}>
                    Бесплатно
                  </span>
                )}
              </div>
              <p style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.6 }}>{s.text}</p>
            </div>
          </li>
        ))}
      </ol>

      {/* ═══ Технологии ═══ */}
      <SectionTitle kicker="Под капотом" title="Что делает песню особенной" />
      <div
        style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(230px, 1fr))', gap: '12px', marginBottom: '52px' }}
      >
        {TECH.map((t, i) => (
          <div
            key={i}
            data-reveal
            className="reveal reveal-up interactive-card glow-hover"
            style={{
              background: 'linear-gradient(160deg, #101513 0%, #0c0c0c 60%)',
              border: `1px solid ${BORDER}`, borderRadius: '16px', padding: '18px 18px',
              '--reveal-delay': `${Math.min(i * 60, 360)}ms`,
            } as React.CSSProperties}
          >
            <div style={{
              width: 42, height: 42, borderRadius: '12px', marginBottom: '12px',
              display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '20px',
              background: 'linear-gradient(135deg, rgba(0,229,192,0.16), rgba(0,191,165,0.04))',
              border: '1px solid rgba(0,229,192,0.22)',
            }}>
              {t.icon}
            </div>
            <h3 style={{ fontSize: '15px', fontWeight: 700, marginBottom: '6px', letterSpacing: '-0.01em' }}>{t.title}</h3>
            <p style={{ fontSize: '13px', color: TEXT2, lineHeight: 1.55 }}>{t.text}</p>
          </div>
        ))}
      </div>

      {/* ═══ Цифры ═══ */}
      <div
        data-reveal
        className="reveal reveal-up"
        style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: '12px', marginBottom: '48px' }}
      >
        {[
          { value: '~10', unit: 'минут', label: 'до готовых версий' },
          { value: '4', unit: 'версии', label: 'на один заказ' },
          { value: '0 ₽', unit: '', label: 'за демо до оплаты' },
          { value: '30+', unit: 'поводов', label: 'и свой конструктор' },
        ].map((m, i) => (
          <div key={i} style={{
            textAlign: 'center', padding: '20px 14px', borderRadius: '16px',
            background: 'linear-gradient(160deg, #101513 0%, #0c0c0c 60%)', border: `1px solid ${BORDER}`,
          }}>
            <div style={{ fontSize: '30px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em', lineHeight: 1 }}>
              {m.value}
              {m.unit && <span style={{ fontSize: '13px', color: TEXT2, fontWeight: 600, marginLeft: '4px' }}>{m.unit}</span>}
            </div>
            <div style={{ fontSize: '12px', color: TEXT3, marginTop: '8px' }}>{m.label}</div>
          </div>
        ))}
      </div>

      {/* ═══ CTA ═══ */}
      <div data-reveal className="reveal reveal-up" style={{ textAlign: 'center', margin: '8px 0 16px' }}>
        <div style={{ fontSize: '18px', fontWeight: 800, marginBottom: '6px', letterSpacing: '-0.02em' }}>Соберите свою песню</div>
        <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '18px' }}>Демо бесплатно — рискуете только парой минут</div>
        <Button size="lg" onClick={() => navigate('/')}>Начать бесплатно →</Button>
        <div style={{ fontSize: '12px', color: TEXT3, marginTop: '12px' }}>
          4 версии · готово за ~10 минут · {price_label} без подписок
        </div>
      </div>
    </div>
  )
}
