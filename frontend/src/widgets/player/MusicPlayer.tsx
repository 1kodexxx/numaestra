import { useState, useRef, useEffect, useCallback } from 'react'
import type { Track } from '@entities/order'

interface MusicPlayerProps {
  tracks: Track[]
}

const WAVE_HEIGHTS = Array.from({ length: 48 }, (_, i) =>
  30 + Math.sin(i * 0.63) * 22 + Math.sin(i * 1.41) * 12 + Math.sin(i * 2.3) * 6
)

function fmt(sec: number): string {
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

export function MusicPlayer({ tracks }: MusicPlayerProps) {
  const [idx, setIdx] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [volume, setVolume] = useState(0.85)
  const [dragging, setDragging] = useState(false)
  const audioRef = useRef<HTMLAudioElement>(null)
  const barRef = useRef<HTMLDivElement>(null)

  const track = tracks[idx]
  const progress = duration > 0 ? currentTime / duration : 0

  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    el.volume = volume
    el.src = track.audio_url

    const onTime = () => !dragging && setCurrentTime(el.currentTime)
    const onMeta = () => setDuration(el.duration || track.duration_sec)
    const onEnd  = () => {
      if (idx < tracks.length - 1) { setIdx(idx + 1); setPlaying(true) }
      else setPlaying(false)
    }

    el.addEventListener('timeupdate', onTime)
    el.addEventListener('loadedmetadata', onMeta)
    el.addEventListener('ended', onEnd)
    setCurrentTime(0)
    setDuration(track.duration_sec)

    return () => {
      el.removeEventListener('timeupdate', onTime)
      el.removeEventListener('loadedmetadata', onMeta)
      el.removeEventListener('ended', onEnd)
    }
  }, [idx, track, tracks.length, dragging])

  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    if (playing) el.play().catch(() => setPlaying(false))
    else el.pause()
  }, [playing])

  useEffect(() => {
    if (audioRef.current) audioRef.current.volume = volume
  }, [volume])

  const seek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const el = audioRef.current
    const bar = barRef.current
    if (!el || !bar || !duration) return
    const rect = bar.getBoundingClientRect()
    const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
    el.currentTime = ratio * duration
    setCurrentTime(ratio * duration)
  }, [duration])

  function switchTrack(next: number) {
    setIdx(next)
    setCurrentTime(0)
    setPlaying(true)
  }

  return (
    <div className="rounded-2xl overflow-hidden" style={{ background: '#141414', border: '1px solid #252525' }}>
      <audio ref={audioRef} preload="metadata" />

      {/* Track tabs */}
      <div className="flex border-b" style={{ borderColor: '#252525' }}>
        {tracks.map((t, i) => (
          <button
            key={t.index}
            onClick={() => switchTrack(i)}
            className="flex-1 py-3 text-sm font-semibold transition-all"
            style={{
              background: i === idx ? '#00e5c0' : 'transparent',
              color: i === idx ? '#0d0d0d' : '#777',
              border: 'none',
              cursor: 'pointer',
            }}
          >
            Вариант {t.index || i + 1}
          </button>
        ))}
      </div>

      <div className="p-6">
        {/* Waveform */}
        <div
          className="flex items-end justify-center gap-[2px] mb-6 cursor-pointer select-none"
          style={{ height: '56px' }}
          ref={barRef}
          onClick={seek}
        >
          {WAVE_HEIGHTS.map((h, i) => {
            const barProgress = i / WAVE_HEIGHTS.length
            const active = barProgress <= progress
            return (
              <div
                key={i}
                style={{
                  width: '4px',
                  height: `${h}%`,
                  background: active ? '#00e5c0' : '#2a2a2a',
                  borderRadius: '2px',
                  transformOrigin: 'bottom',
                  animationDelay: `${i * 0.035}s`,
                  transition: 'background 0.1s',
                }}
                className={playing && active ? 'wave-animate' : ''}
              />
            )
          })}
        </div>

        {/* Progress bar */}
        <div
          className="rounded-full mb-2 cursor-pointer"
          style={{ height: '4px', background: '#252525' }}
          onClick={seek}
        >
          <div
            className="h-full rounded-full transition-all"
            style={{ width: `${progress * 100}%`, background: '#00e5c0' }}
          />
        </div>

        {/* Time */}
        <div className="flex justify-between text-xs mb-6" style={{ color: '#777' }}>
          <span>{fmt(currentTime)}</span>
          <span>{fmt(duration)}</span>
        </div>

        {/* Controls */}
        <div className="flex items-center justify-center gap-6">
          <button
            onClick={() => idx > 0 && switchTrack(idx - 1)}
            disabled={idx === 0}
            className="transition-opacity"
            style={{
              background: 'none', border: 'none', cursor: idx > 0 ? 'pointer' : 'default',
              opacity: idx > 0 ? 1 : 0.3, color: '#fff',
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
              <path d="M6 6h2v12H6zm3.5 6 8.5 6V6z"/>
            </svg>
          </button>

          <button
            onClick={() => setPlaying(!playing)}
            className="flex items-center justify-center rounded-full transition-transform hover:scale-105 active:scale-95"
            style={{
              width: '64px', height: '64px',
              background: '#00e5c0',
              border: 'none', cursor: 'pointer',
            }}
          >
            {playing ? (
              <svg width="24" height="24" viewBox="0 0 24 24" fill="#0d0d0d">
                <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>
              </svg>
            ) : (
              <svg width="24" height="24" viewBox="0 0 24 24" fill="#0d0d0d">
                <path d="M8 5v14l11-7z"/>
              </svg>
            )}
          </button>

          <button
            onClick={() => idx < tracks.length - 1 && switchTrack(idx + 1)}
            disabled={idx === tracks.length - 1}
            className="transition-opacity"
            style={{
              background: 'none', border: 'none', cursor: idx < tracks.length - 1 ? 'pointer' : 'default',
              opacity: idx < tracks.length - 1 ? 1 : 0.3, color: '#fff',
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
              <path d="M6 18l8.5-6L6 6v12zm2.5-6 5.5 3.9V8.1L8.5 12zM16 6h2v12h-2z"/>
            </svg>
          </button>
        </div>

        {/* Volume */}
        <div className="flex items-center gap-3 mt-6">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="#777">
            <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02z"/>
          </svg>
          <input
            type="range" min="0" max="1" step="0.01"
            value={volume}
            onChange={(e) => setVolume(Number(e.target.value))}
            className="flex-1 h-1 rounded-full appearance-none cursor-pointer"
            style={{
              background: `linear-gradient(to right, #00e5c0 ${volume * 100}%, #252525 ${volume * 100}%)`,
            }}
          />
          <svg width="16" height="16" viewBox="0 0 24 24" fill="#777">
            <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
          </svg>
        </div>
      </div>
    </div>
  )
}
