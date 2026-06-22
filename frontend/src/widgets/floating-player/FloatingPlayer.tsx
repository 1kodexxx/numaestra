import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { synthDemoTrack, hashStr } from '@shared/lib/demoAudio'
import type { ExampleSong } from '@shared/data/examples'

const ACCENT = '#00e5c0'
const TEXT2 = 'rgba(255,255,255,0.5)'

function fmt(s: number) {
  if (!isFinite(s)) return '0:00'
  const m = Math.floor(s / 60)
  return `${m}:${Math.floor(s % 60).toString().padStart(2, '0')}`
}

/** Плавающий мини-плеер демо-примера (как в Spotify). Синтезирует звучание,
 *  автозапуск, перемотка по полосе, закрытие. */
export function FloatingPlayer({ example, onClose }: { example: ExampleSong; onClose: () => void }) {
  const navigate = useNavigate()
  const audioRef = useRef<HTMLAudioElement>(null)
  const [ready, setReady] = useState(false)
  const [playing, setPlaying] = useState(false)
  const [time, setTime] = useState(0)
  const [dur, setDur] = useState(0)

  useEffect(() => {
    let cancelled = false
    let createdUrl = ''
    setReady(false); setPlaying(false); setTime(0); setDur(0)
    void synthDemoTrack(hashStr(example.id)).then((res) => {
      const el = audioRef.current
      if (cancelled || !res) { if (res) URL.revokeObjectURL(res.url); return }
      createdUrl = res.url
      if (el) {
        el.src = res.url
        setDur(res.duration)
        setReady(true)
        el.play().then(() => setPlaying(true)).catch(() => {})
      }
    })
    return () => { cancelled = true; if (createdUrl) URL.revokeObjectURL(createdUrl) }
  }, [example.id])

  function toggle() {
    const el = audioRef.current
    if (!el || !ready) return
    if (playing) { el.pause(); setPlaying(false) }
    else { el.play().then(() => setPlaying(true)).catch(() => {}) }
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
    <div style={{
      position: 'fixed', left: 0, right: 0, bottom: 0, zIndex: 35,
      background: 'rgba(15,15,15,0.92)', backdropFilter: 'blur(20px)', WebkitBackdropFilter: 'blur(20px)',
      borderTop: '1px solid rgba(0,229,192,0.2)', boxShadow: '0 -10px 30px rgba(0,0,0,0.45)',
    }}>
      <audio
        ref={audioRef}
        preload="metadata"
        onTimeUpdate={(e) => setTime(e.currentTarget.currentTime)}
        onLoadedMetadata={(e) => { if (isFinite(e.currentTarget.duration)) setDur(e.currentTarget.duration) }}
        onEnded={() => setPlaying(false)}
      />
      <div className="fade-up" style={{
        maxWidth: 1000, margin: '0 auto', padding: '12px 16px',
        display: 'flex', alignItems: 'center', gap: '14px',
      }}>
        {/* art + play */}
        <button
          onClick={toggle}
          aria-label={playing ? 'Пауза' : 'Воспроизвести'}
          style={{
            position: 'relative', flexShrink: 0, width: 48, height: 48, borderRadius: '12px',
            border: 'none', cursor: 'pointer', overflow: 'hidden',
            background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >
          {ready ? (
            <svg width="20" height="20" viewBox="0 0 24 24" fill="#062420">
              {playing ? <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" /> : <path d="M8 5v14l11-7z" />}
            </svg>
          ) : (
            <span className="spin-anim" style={{ width: 18, height: 18, borderRadius: '50%', border: '2px solid rgba(6,36,32,0.4)', borderTopColor: '#062420' }} />
          )}
        </button>

        {/* title + progress */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px' }}>
            <span style={{ fontSize: '14px', fontWeight: 700, color: '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {example.title}
            </span>
            <span style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.04em', flexShrink: 0 }}>ДЕМО</span>
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

        {/* actions */}
        <button
          onClick={() => navigate(`/examples/${example.id}`)}
          style={{
            flexShrink: 0, padding: '8px 14px', borderRadius: '12px', cursor: 'pointer',
            background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.3)',
            color: ACCENT, fontSize: '12px', fontWeight: 600, fontFamily: 'inherit', whiteSpace: 'nowrap',
          }}
        >
          Подробнее
        </button>
        <button
          onClick={onClose}
          aria-label="Закрыть плеер"
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
