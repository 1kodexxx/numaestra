import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import type { ExampleSong } from '@shared/data/examples'
import { theme } from '@shared/lib/theme'
import { useAudioBuffering } from '@shared/lib/useAudioBuffering'

const ACCENT = theme.accent
const TEXT2 = theme.text2

function fmt(s: number) {
  if (!isFinite(s)) return '0:00'
  const m = Math.floor(s / 60)
  return `${m}:${Math.floor(s % 60).toString().padStart(2, '0')}`
}

/**
 * Плавающий мини-плеер демо-примера (как в Spotify).
 */
export function FloatingPlayer({
  example,
  track,
  audioRef,
  onClose,
}: {
  example: ExampleSong
  track: { url: string; duration: number } | null
  audioRef: React.RefObject<HTMLAudioElement | null>
  onClose: () => void
}) {
  const navigate = useNavigate()
  const [playing, setPlaying] = useState(false)
  const [time, setTime] = useState(0)
  const [dur, setDur] = useState(track?.duration ?? 0)
  const [buffering, setBuffering] = useAudioBuffering(audioRef)
  const ready = !!track

  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    const onTime = () => setTime(el.currentTime)
    const onMeta = () => { if (isFinite(el.duration) && el.duration > 0) setDur(el.duration) }
    const onPlay = () => setPlaying(true)
    const onPause = () => setPlaying(false)
    const onEnded = () => setPlaying(false)
    el.addEventListener('timeupdate', onTime)
    el.addEventListener('loadedmetadata', onMeta)
    el.addEventListener('play', onPlay)
    el.addEventListener('pause', onPause)
    el.addEventListener('ended', onEnded)
    setPlaying(!el.paused && !el.ended)
    setTime(el.currentTime)
    if (isFinite(el.duration) && el.duration > 0) setDur(el.duration)
    return () => {
      el.removeEventListener('timeupdate', onTime)
      el.removeEventListener('loadedmetadata', onMeta)
      el.removeEventListener('play', onPlay)
      el.removeEventListener('pause', onPause)
      el.removeEventListener('ended', onEnded)
    }
  }, [audioRef, example.id])

  function toggle() {
    const el = audioRef.current
    if (!el || !ready) return
    if (el.paused) {
      setBuffering(true) // показываем загрузку сразу, пока трек подгружается
      el.play().catch(() => setBuffering(false))
    } else {
      el.pause()
    }
  }

  function seek(e: React.MouseEvent<HTMLDivElement>) {
    const el = audioRef.current
    if (!el || !dur) return
    const r = e.currentTarget.getBoundingClientRect()
    const p = Math.max(0, Math.min(1, (e.clientX - r.left) / r.width))
    el.currentTime = p * dur
    setTime(p * dur)
  }

  const pct = dur > 0 ? (time / dur) * 100 : 0

  return (
    <div className="floating-player-shell slide-up-in">
      <div className="floating-player-inner">
        <button
          type="button"
          onClick={toggle}
          aria-label={playing ? 'Пауза' : 'Воспроизвести'}
          className="chip-press"
          style={{
            position: 'relative', flexShrink: 0, width: 48, height: 48, borderRadius: '12px',
            border: 'none', cursor: 'pointer', overflow: 'hidden',
            background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >
          {example.coverUrl && (
            <img src={example.coverUrl} alt="" aria-hidden
              style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' }} />
          )}
          {example.coverUrl && (
            <span aria-hidden style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,0.35)' }} />
          )}
          <span style={{ position: 'relative', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          {ready && !buffering ? (
            <svg width="20" height="20" viewBox="0 0 24 24" fill={example.coverUrl ? '#fff' : '#062420'}>
              {playing ? <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" /> : <path d="M8 5v14l11-7z" />}
            </svg>
          ) : (
            <span className="spin-anim" style={{ width: 18, height: 18, borderRadius: '50%', border: '2px solid rgba(255,255,255,0.4)', borderTopColor: '#fff' }} />
          )}
          </span>
        </button>

        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px' }}>
            <span style={{ fontSize: '14px', fontWeight: 700, color: '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {example.title}
            </span>
            {!example.audioUrl && <span style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.04em', flexShrink: 0 }}>ДЕМО</span>}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px' }}>
            <span style={{ fontSize: '11px', color: TEXT2, fontVariantNumeric: 'tabular-nums', width: 32 }}>{fmt(time)}</span>
            <div onClick={seek} style={{ flex: 1, height: '12px', display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
              <div style={{ width: '100%', height: '4px', background: 'rgba(255,255,255,0.12)', borderRadius: '2px', position: 'relative' }}>
                <div style={{ width: `${pct}%`, height: '100%', borderRadius: '2px', background: 'linear-gradient(90deg, #00bfa5, #00e5c0)' }} />
              </div>
            </div>
            <span style={{ fontSize: '11px', color: TEXT2, fontVariantNumeric: 'tabular-nums', width: 32, textAlign: 'right' }}>{fmt(dur)}</span>
          </div>
        </div>

        <button
          type="button"
          onClick={() => navigate(`/examples/${example.id}`)}
          className="floating-player-detail-btn chip-press"
          aria-label="Подробнее о примере"
          style={{
            flexShrink: 0, padding: '8px 14px', borderRadius: '12px', cursor: 'pointer',
            background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.3)',
            color: ACCENT, fontSize: '12px', fontWeight: 600, fontFamily: 'inherit', whiteSpace: 'nowrap',
          }}
        >
          <span className="floating-player-detail-label">Подробнее</span>
          <span className="floating-player-detail-icon" aria-hidden>→</span>
        </button>
        <button
          type="button"
          onClick={onClose}
          aria-label="Закрыть плеер"
          className="chip-press"
          style={{
            flexShrink: 0, width: 36, height: 36, borderRadius: '50%', cursor: 'pointer',
            background: 'transparent', border: 'none', color: TEXT2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
            <path d="M18 6 6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  )
}
