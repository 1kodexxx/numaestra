import { useNavigate, useParams } from 'react-router-dom'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import { MusicPlayer } from '@widgets/player'
import { Button } from '@shared/ui'
import type { Track } from '@entities/order'

const ACCENT = '#00e5c0'
const TEXT2  = 'rgba(255,255,255,0.5)'
const TEXT3  = 'rgba(255,255,255,0.25)'
const BORDER = 'rgba(255,255,255,0.07)'

/* Demo tracks with short silent WAV — playable placeholder */
const SILENCE_URL = 'data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA='

function makeDemoTracks(count: number): Track[] {
  return Array.from({ length: count }, (_, i) => ({
    index: i + 1,
    audio_url: SILENCE_URL,
    duration_sec: 180 + i * 12,
  }))
}

const MOOD_ICON: Record<string, string> = {
  'Романтика': '🌹', 'Радость': '🎉', 'Энергия': '⚡',
  'Юмор': '😄', 'Ностальгия': '🕰️',
}

export function ExampleDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const example = EXAMPLE_SONGS.find((e) => e.id === id)

  if (!example) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 'calc(100vh - 60px)', padding: '24px' }}>
        <div style={{ fontSize: '48px', marginBottom: '16px' }}>🔍</div>
        <div style={{ fontSize: '20px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '8px' }}>Пример не найден</div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '24px' }}>Возможно, ссылка устарела</div>
        <Button size="lg" onClick={() => navigate('/')}>На главную</Button>
      </div>
    )
  }

  const demoTracks = makeDemoTracks(4)
  const moodIcon = MOOD_ICON[example.mood] ?? '🎵'

  return (
    <div style={{ maxWidth: 640, margin: '0 auto', padding: '32px 20px 60px' }} className="fade-in">
      <button
        onClick={() => navigate(-1)}
        style={{
          background: 'none', border: 'none', color: TEXT2, fontSize: '13px',
          cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '6px',
          padding: 0, marginBottom: '24px', transition: 'color 0.15s',
        }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = TEXT2 }}
      >
        ← Назад
      </button>

      {/* Header */}
      <div style={{
        background: '#0f0f0f', border: `1px solid ${BORDER}`,
        borderRadius: '24px', padding: '32px 30px',
        marginBottom: '16px', position: 'relative', overflow: 'hidden',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '20px',
      }}>
        <div style={{
          position: 'absolute', inset: 0, pointerEvents: 'none',
          background: 'radial-gradient(ellipse 70% 60% at 90% 10%, rgba(0,229,192,0.1) 0%, transparent 65%)',
        }} />
        <div style={{ position: 'relative', zIndex: 1 }}>
          <div style={{ fontSize: '12px', fontWeight: 600, color: ACCENT, marginBottom: '8px', letterSpacing: '0.02em' }}>
            {example.category}
          </div>
          <div style={{ fontSize: '26px', fontWeight: 800, letterSpacing: '-0.03em', lineHeight: 1.15, marginBottom: '12px' }}>
            {example.title}
          </div>
          <div style={{
            display: 'inline-flex', alignItems: 'center', gap: '6px',
            background: 'rgba(255,255,255,0.05)', border: `1px solid ${BORDER}`,
            borderRadius: '20px', padding: '5px 12px',
            fontSize: '12px', fontWeight: 500, color: 'rgba(255,255,255,0.6)',
          }}>
            <span>{moodIcon}</span> {example.mood}
          </div>
        </div>
        <div style={{
          flexShrink: 0, width: '72px', height: '72px', borderRadius: '20px',
          background: 'linear-gradient(145deg, #00e5c0, #00bfa5)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: '36px', boxShadow: '0 8px 28px rgba(0,229,192,0.25)',
          position: 'relative', zIndex: 1,
        }}>
          🎵
        </div>
      </div>

      {/* Description */}
      <div style={{
        background: '#0f0f0f', border: `1px solid ${BORDER}`,
        borderRadius: '20px', padding: '24px 26px', marginBottom: '16px',
      }}>
        <div style={{ fontSize: '11px', fontWeight: 700, color: TEXT3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '12px' }}>
          О треке
        </div>
        <p style={{ fontSize: '15px', lineHeight: 1.7, color: 'rgba(255,255,255,0.8)' }}>
          {example.description}
        </p>
      </div>

      {/* Player */}
      <div style={{ marginBottom: '16px' }}>
        <div style={{ fontSize: '11px', fontWeight: 700, color: TEXT3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          Прослушать
          <span style={{
            background: 'rgba(255,255,255,0.06)', borderRadius: '6px',
            padding: '2px 7px', fontSize: '9px', color: TEXT2, letterSpacing: '0.04em',
          }}>
            ДЕМО
          </span>
        </div>
        <MusicPlayer tracks={demoTracks} />
      </div>

      {/* CTA */}
      <div style={{
        background: 'linear-gradient(135deg, rgba(0,229,192,0.08), rgba(0,191,165,0.04))',
        border: '1px solid rgba(0,229,192,0.18)',
        borderRadius: '20px', padding: '24px 26px', textAlign: 'center',
      }}>
        <div style={{ fontSize: '18px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '6px' }}>
          Хотите такую же — но про вас?
        </div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '20px' }}>
          4 уникальные версии за 24 часа · 2 000 ₽
        </div>
        <Button size="lg" fullWidth onClick={() => navigate('/')}>
          Заказать свою песню →
        </Button>
      </div>
    </div>
  )
}
