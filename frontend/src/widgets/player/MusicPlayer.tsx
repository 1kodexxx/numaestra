import { useState, useRef, useEffect, useCallback } from 'react'
import { IconButton, useRipple } from '@shared/ui'
import type { Track } from '@entities/order'

/* ─── precomputed waveform heights ─── */
const BARS = Array.from({ length: 52 }, (_, i) =>
  28 + Math.sin(i * 0.61) * 22 + Math.sin(i * 1.37) * 13 + Math.sin(i * 2.7) * 6
)

function fmt(s: number) {
  const m = Math.floor(s / 60)
  return `${m}:${Math.floor(s % 60).toString().padStart(2, '0')}`
}

/* ─── icons ─── */
const PlayIcon = () => (
  <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
    <path d="M8 5v14l11-7z"/>
  </svg>
)
const PauseIcon = () => (
  <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
    <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>
  </svg>
)
const PrevIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
    <path d="M6 6h2v12H6zm3.5 6 8.5 6V6z"/>
  </svg>
)
const NextIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
    <path d="M6 18l8.5-6L6 6v12zm2.5-6 5.5 3.9V8.1L8.5 12zM16 6h2v12h-2z"/>
  </svg>
)
const VolIcon = ({ muted }: { muted: boolean }) => muted ? (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" style={{ opacity: 0.4 }}>
    <path d="M16.5 12A4.5 4.5 0 0 0 14 7.97v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3 3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06a8.99 8.99 0 0 0 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4 9.91 6.09 12 8.18V4z"/>
  </svg>
) : (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" style={{ opacity: 0.4 }}>
    <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
  </svg>
)

/* ─── main component ─── */
export function MusicPlayer({ tracks }: { tracks: Track[] }) {
  const [idx, setIdx]         = useState(0)
  const [playing, setPlaying] = useState(false)
  const [time, setTime]       = useState(0)
  const [dur, setDur]         = useState(0)
  const [vol, setVol]         = useState(0.85)
  const [muted, setMuted]     = useState(false)
  const audioRef  = useRef<HTMLAudioElement>(null)

  // Клампим idx: если tracks стал короче (например, сменился заказ),
  // не выходим за границы массива.
  useEffect(() => {
    if (idx > tracks.length - 1) setIdx(0)
  }, [tracks.length, idx])

  const track    = tracks[idx] ?? tracks[0]
  const progress = dur > 0 ? time / dur : 0

  /* sync audio src.
   * Зависим от ПРИМИТИВОВ (audio_url / duration_sec), а не от объекта track:
   * StatusPage опрашивает заказ каждые 10 с и передаёт новый массив tracks с
   * новыми идентичностями объектов. Если зависеть от track, эффект бы
   * перезапускался каждые 10 с, сбрасывая el.src и воспроизведение на 0.
   * Громкость намеренно НЕ в зависимостях — ей занимается отдельный эффект ниже,
   * иначе перетаскивание ползунка громкости перезагружало бы трек. */
  useEffect(() => {
    const el = audioRef.current
    if (!el || !track) return
    el.src = track.audio_url
    setTime(0)
    setDur(track.duration_sec)

    const onMeta = () => setDur(el.duration || track.duration_sec)
    const onTime = () => setTime(el.currentTime)
    const onEnd  = () => {
      if (idx < tracks.length - 1) { setIdx(i => i + 1); setPlaying(true) }
      else setPlaying(false)
    }
    el.addEventListener('loadedmetadata', onMeta)
    el.addEventListener('timeupdate', onTime)
    el.addEventListener('ended', onEnd)
    return () => {
      el.removeEventListener('loadedmetadata', onMeta)
      el.removeEventListener('timeupdate', onTime)
      el.removeEventListener('ended', onEnd)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [idx, track?.audio_url, track?.duration_sec, tracks.length])

  /* play/pause. Зависит и от idx: при переключении трека во время
   * воспроизведения src меняется, и play() нужно вызвать заново для нового src
   * (иначе новый трек загрузится, но не зазвучит). */
  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    if (playing) el.play().catch(() => setPlaying(false))
    else el.pause()
  }, [playing, idx])

  /* volume */
  useEffect(() => {
    if (audioRef.current) audioRef.current.volume = muted ? 0 : vol
  }, [vol, muted])

  const seek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const el = audioRef.current
    if (!el || !dur) return
    const r = e.currentTarget.getBoundingClientRect()
    const p = Math.max(0, Math.min(1, (e.clientX - r.left) / r.width))
    el.currentTime = p * dur
    setTime(p * dur)
  }, [dur])

  function switchTrack(next: number) {
    setIdx(next)
    setTime(0)
    setPlaying(true)
  }

  const pct = `${(progress * 100).toFixed(1)}%`
  const playRipple = useRipple('rgba(0,0,0,0.4)')

  return (
    <div style={{
      background: '#0f0f0f',
      border: '1px solid rgba(255,255,255,0.07)',
      borderRadius: '20px',
      overflow: 'hidden',
      position: 'relative',
    }}>
      <audio ref={audioRef} preload="metadata" />

      {/* Ambient glow when playing */}
      {playing && (
        <div style={{
          position: 'absolute', inset: 0, pointerEvents: 'none', borderRadius: '20px',
          background: 'radial-gradient(ellipse 60% 40% at 50% 0%, rgba(0,229,192,0.07) 0%, transparent 70%)',
          transition: 'opacity 0.5s',
        }} />
      )}

      {/* Track tabs */}
      <div className="music-player-tabs">
        {tracks.map((t, i) => (
          <button
            key={t.index}
            type="button"
            className="music-player-tab chip-press"
            aria-label={`Вариант ${t.index || i + 1}`}
            onClick={() => switchTrack(i)}
            style={{
              background: i === idx ? 'rgba(0,229,192,0.1)' : 'transparent',
              color: i === idx ? '#00e5c0' : 'rgba(255,255,255,0.3)',
              borderBottom: i === idx ? '2px solid #00e5c0' : '2px solid transparent',
            }}
          >
            <span className="music-player-tab-short">{t.index || i + 1}</span>
            <span className="music-player-tab-full">Вариант {t.index || i + 1}</span>
          </button>
        ))}
      </div>

      {/* Body */}
      <div className="music-player-body">

        {/* Waveform */}
        <div
          onClick={seek}
          style={{
            display: 'flex',
            alignItems: 'flex-end',
            justifyContent: 'center',
            height: '72px',
            gap: '2px',
            cursor: 'pointer',
            marginBottom: '20px',
          }}
        >
          {BARS.map((h, i) => {
            const active = i / BARS.length <= progress
            return (
              <div
                key={i}
                className={playing && active ? 'wave-animate' : ''}
                style={{
                  width: '4px',
                  height: `${h}%`,
                  borderRadius: '3px',
                  transformOrigin: 'bottom',
                  background: active ? '#00e5c0' : 'rgba(255,255,255,0.08)',
                  boxShadow: active && playing ? '0 0 6px rgba(0,229,192,0.4)' : 'none',
                  animationDelay: `${i * 0.032}s`,
                  transition: 'background 0.08s',
                }}
              />
            )
          })}
        </div>

        {/* Progress bar */}
        <div
          onClick={seek}
          style={{ height: '6px', display: 'flex', alignItems: 'center', cursor: 'pointer', marginBottom: '8px' }}
        >
        <div style={{ height: '3px', width: '100%', background: 'rgba(255,255,255,0.08)', borderRadius: '2px', position: 'relative' }}>
          <div style={{
            width: pct, height: '100%', borderRadius: '2px',
            background: 'linear-gradient(90deg, #00bfa5, #00e5c0)',
            position: 'relative',
          }}>
            {dur > 0 && (
              <div style={{
                position: 'absolute', right: '-5px', top: '50%', transform: 'translateY(-50%)',
                width: '10px', height: '10px', borderRadius: '50%',
                background: '#00e5c0', boxShadow: '0 0 0 3px rgba(0,229,192,0.2)',
              }} />
            )}
          </div>
        </div>
        </div>

        {/* Time */}
        <div style={{
          display: 'flex', justifyContent: 'space-between',
          fontSize: '12px', color: 'rgba(255,255,255,0.28)',
          fontVariantNumeric: 'tabular-nums', marginBottom: '24px',
          fontFamily: 'monospace',
        }}>
          <span>{fmt(time)}</span>
          <span>{dur > 0 ? fmt(dur) : '—'}</span>
        </div>

        {/* Controls */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '16px', marginBottom: '24px' }}>
          <IconButton label="Предыдущий вариант" disabled={idx === 0} onClick={() => idx > 0 && switchTrack(idx - 1)}>
            <PrevIcon />
          </IconButton>

          <button
            onClick={() => setPlaying(p => !p)}
            onPointerDown={playRipple.onPointerDown}
            aria-label={playing ? 'Пауза' : 'Воспроизвести'}
            style={{
              position: 'relative', overflow: 'hidden',
              width: '64px', height: '64px',
              borderRadius: '50%',
              background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
              border: 'none', cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: '#080808',
              boxShadow: playing
                ? '0 0 0 8px rgba(0,229,192,0.12), 0 8px 24px rgba(0,229,192,0.3)'
                : '0 4px 16px rgba(0,229,192,0.2)',
              transition: 'all 0.2s cubic-bezier(0.34,1.56,0.64,1)',
              transform: 'scale(1)',
              WebkitTapHighlightColor: 'transparent',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.transform = 'scale(1.06)' }}
            onMouseLeave={(e) => { e.currentTarget.style.transform = 'scale(1)' }}
            onMouseDown={(e) => { e.currentTarget.style.transform = 'scale(0.95)' }}
            onMouseUp={(e) => { e.currentTarget.style.transform = 'scale(1.06)' }}
          >
            {playing ? <PauseIcon /> : <PlayIcon />}
            {playRipple.rippleEl}
          </button>

          <IconButton label="Следующий вариант" disabled={idx === tracks.length - 1} onClick={() => idx < tracks.length - 1 && switchTrack(idx + 1)}>
            <NextIcon />
          </IconButton>
        </div>

        {/* Volume */}
        <div className="music-player-volume">
          <IconButton label={muted ? 'Включить звук' : 'Выключить звук'} size={36} onClick={() => setMuted(m => !m)}>
            <VolIcon muted={muted} />
          </IconButton>
          <input
            type="range" min="0" max="1" step="0.01"
            value={muted ? 0 : vol}
            onChange={(e) => { setVol(Number(e.target.value)); setMuted(false) }}
            style={{
              flex: 1,
              background: `linear-gradient(to right, #00e5c0 ${(muted ? 0 : vol) * 100}%, rgba(255,255,255,0.1) ${(muted ? 0 : vol) * 100}%)`,
            }}
          />
        </div>

      </div>
    </div>
  )
}
