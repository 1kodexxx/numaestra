import { useNavigate, useParams } from 'react-router-dom'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import { MusicPlayer } from '@widgets/player'
import type { Track } from '@entities/order'

const CYAN = '#00e5c0'
const DARK = '#0d0d0d'

// Demo tracks with silent audio (data URI — very short WAV silence)
const SILENCE_URL = 'data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA='

function makeDemoTracks(count: number): Track[] {
  return Array.from({ length: count }, (_, i) => ({
    index: i + 1,
    audio_url: SILENCE_URL,
    duration_sec: 180 + i * 12,
  }))
}

export function ExampleDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const example = EXAMPLE_SONGS.find((e) => e.id === id)

  if (!example) {
    return (
      <div className="flex flex-col items-center justify-center" style={{ height: 'calc(100vh - 56px)' }}>
        <div className="text-5xl mb-4">🎵</div>
        <div className="text-lg font-bold mb-2">Пример не найден</div>
        <button
          onClick={() => navigate('/')}
          className="mt-4 px-6 py-2.5 rounded-xl text-sm font-semibold"
          style={{ background: CYAN, color: DARK, border: 'none', cursor: 'pointer' }}
        >
          На главную
        </button>
      </div>
    )
  }

  const demoTracks = makeDemoTracks(4)

  return (
    <div className="max-w-2xl mx-auto px-6 py-10">
      <button
        onClick={() => navigate(-1)}
        className="flex items-center gap-1.5 text-sm mb-8"
        style={{ background: 'none', border: 'none', color: '#777', cursor: 'pointer' }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = '#777' }}
      >
        ← Назад
      </button>

      {/* Header */}
      <div
        className="rounded-2xl p-8 mb-6 flex items-center justify-between"
        style={{ background: CYAN }}
      >
        <div>
          <div className="text-xs font-semibold mb-1 opacity-60" style={{ color: DARK }}>{example.category}</div>
          <div className="text-2xl font-extrabold" style={{ color: DARK }}>{example.title}</div>
          <div
            className="inline-block mt-2 px-3 py-1 rounded-full text-xs font-semibold"
            style={{ background: 'rgba(0,0,0,0.15)', color: DARK }}
          >
            {example.mood}
          </div>
        </div>
        <div style={{ fontSize: '56px' }}>🎵</div>
      </div>

      {/* Description */}
      <div
        className="rounded-2xl p-6 mb-6"
        style={{ background: '#141414', border: '1px solid #252525' }}
      >
        <div className="text-sm font-semibold mb-2" style={{ color: '#aaa' }}>О треке</div>
        <p className="leading-relaxed" style={{ color: '#ddd' }}>{example.description}</p>
      </div>

      {/* Player */}
      <div className="mb-6">
        <div className="text-sm font-semibold mb-3" style={{ color: '#aaa' }}>Слушать (демо)</div>
        <MusicPlayer tracks={demoTracks} />
      </div>

      {/* CTA */}
      <button
        onClick={() => navigate('/')}
        className="w-full py-4 rounded-2xl text-base font-bold"
        style={{ background: CYAN, color: DARK, border: 'none', cursor: 'pointer' }}
        onMouseEnter={(e) => { e.currentTarget.style.filter = 'brightness(1.1)' }}
        onMouseLeave={(e) => { e.currentTarget.style.filter = '' }}
      >
        Заказать свою песню →
      </button>
    </div>
  )
}
