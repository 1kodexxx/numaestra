import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { orderApi } from '@entities/order'
import type { Track } from '@entities/order'
import { ApiError } from '@shared/api'
import { Spinner, Button } from '@shared/ui'
import { MusicPlayer } from '@widgets/player'
import { useSeo } from '@shared/lib/seo'
import { usePublicConfig } from '@shared/lib/usePublicConfig'
import { theme } from '@shared/lib/theme'

const ACCENT = theme.accent
const TEXT2 = theme.text2

/**
 * Публичная страница «поделиться песней» — доступна по ID заказа без токена.
 * Отдаёт только треки (без email/phone/brief владельца), поэтому безопасна
 * для шеринга: получатель слушает песню, но не получает доступ к заказу.
 */
export function SharePage() {
  const { price_label } = usePublicConfig()
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [tracks, setTracks] = useState<Track[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useSeo({
    title: 'Мне сделали песню в Numaestra 🎵',
    description: 'Послушайте персональную песню, созданную нейросетью в Numaestra.',
    noindex: true,
  })

  useEffect(() => {
    orderApi
      .getPublicShare(id)
      .then((res) => setTracks(res.tracks))
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Песня не найдена'))
  }, [id])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', paddingBottom: '40px' }}>
      <div style={{ flex: 1, maxWidth: 560, width: '100%', margin: '0 auto', padding: '48px 20px 24px' }} className="fade-in">
        <div style={{ textAlign: 'center', marginBottom: '28px' }}>
          <div style={{ fontSize: '13px', fontWeight: 700, color: ACCENT, letterSpacing: '0.06em', textTransform: 'uppercase', marginBottom: '10px' }}>
            🎵 Numaestra
          </div>
          <h1 style={{ fontSize: 'clamp(22px, 4.5vw, 30px)', fontWeight: 800, letterSpacing: '-0.03em', marginBottom: '8px' }}>
            Кто-то поделился с вами песней
          </h1>
          <p style={{ fontSize: '14px', color: TEXT2 }}>Персональный трек, созданный нейросетью</p>
        </div>

        {!tracks && !error && (
          <div style={{ padding: '60px 0', textAlign: 'center' }}><Spinner /></div>
        )}

        {error && (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <div style={{ fontSize: '40px', marginBottom: '12px' }}>🔇</div>
            <div style={{ fontSize: '15px', color: TEXT2 }}>{error}</div>
          </div>
        )}

        {tracks && tracks.length > 0 && <MusicPlayer tracks={tracks} />}

        {tracks && tracks.length === 0 && (
          <div style={{ textAlign: 'center', padding: '32px 0', color: TEXT2, fontSize: '14px' }}>
            Треки для этой ссылки пока недоступны.
          </div>
        )}

        <div style={{ textAlign: 'center', marginTop: '32px' }}>
          <Button size="lg" onClick={() => navigate('/')}>Заказать свою песню →</Button>
          <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.32)', marginTop: '10px' }}>
            4 версии трека · готово за 10 минут · {price_label}
          </div>
        </div>
      </div>
    </div>
  )
}
