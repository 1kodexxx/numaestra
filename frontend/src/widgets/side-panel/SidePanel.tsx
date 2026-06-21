import { useState } from 'react'

const ACCENT = '#00e5c0'
const TEXT2 = 'rgba(255,255,255,0.48)'

/* Стабильный целочисленный хэш строки (djb2) — для детерминированного выбора фото. */
function hashSeed(s: string): number {
  let h = 5381
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) >>> 0
  return h % 100000
}

/* Тематическая стоковая картинка-заглушка (loremflickr, без ключей).
 * keyword задаёт тему, lock делает картинку стабильной для одного и того же seed. */
export function stockImage(seed: string, keyword = 'music', size = 96): string {
  return `https://loremflickr.com/${size}/${size}/${encodeURIComponent(keyword)}?lock=${hashSeed(seed)}`
}

/* ─── panel header ─── */
export function PanelHeader({ icon, title, sub }: { icon: string; title: string; sub: string }) {
  return (
    <div style={{ padding: '4px 11px 16px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <span style={{ fontSize: '17px', lineHeight: 1 }}>{icon}</span>
        <span style={{ fontSize: '17px', fontWeight: 800, color: '#fff', letterSpacing: '-0.02em' }}>{title}</span>
      </div>
      <div style={{ fontSize: '12px', color: TEXT2, marginTop: '4px', fontWeight: 500 }}>{sub}</div>
      <div style={{ height: '2px', borderRadius: '1px', marginTop: '12px', background: 'linear-gradient(90deg, rgba(0,229,192,0.5), transparent)' }} />
    </div>
  )
}

/* ─── image thumbnail (with gradient fallback) ─── */
export function Thumb({ src, alt, active, children }: { src: string; alt: string; active: boolean; children?: React.ReactNode }) {
  return (
    <span style={{
      position: 'relative', flexShrink: 0, width: 46, height: 46, borderRadius: '12px', overflow: 'hidden',
      background: 'linear-gradient(135deg, rgba(0,229,192,0.18), rgba(0,191,165,0.05))',
      border: `1px solid ${active ? 'rgba(0,229,192,0.5)' : 'rgba(255,255,255,0.1)'}`,
      boxShadow: active ? '0 6px 18px rgba(0,229,192,0.28)' : 'none',
      transform: active ? 'scale(1.06)' : 'scale(1)',
      transition: 'all 0.2s cubic-bezier(0.34,1.4,0.64,1)',
    }}>
      <img
        src={src}
        alt={alt}
        loading="lazy"
        onError={(e) => { e.currentTarget.style.opacity = '0' }}
        style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }}
      />
      {children}
    </span>
  )
}

/* ─── play overlay (examples) ─── */
export function PlayOverlay({ active }: { active: boolean }) {
  return (
    <span style={{
      position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: active ? 'rgba(0,0,0,0.2)' : 'rgba(0,0,0,0.4)',
      transition: 'background 0.2s',
    }}>
      <span style={{
        width: 24, height: 24, borderRadius: '50%',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        background: active ? 'linear-gradient(135deg, #00e5c0, #00bfa5)' : 'rgba(255,255,255,0.9)',
        boxShadow: active ? '0 2px 10px rgba(0,229,192,0.5)' : 'none',
        transition: 'all 0.2s',
      }}>
        <svg width="10" height="10" viewBox="0 0 24 24" fill={active ? '#062420' : '#0a0a0a'}>
          <path d="M8 5v14l11-7z" />
        </svg>
      </span>
    </span>
  )
}

/* ─── rank corner badge (top categories) ─── */
export function RankCorner({ n }: { n: number }) {
  return (
    <span style={{
      position: 'absolute', left: '3px', bottom: '3px',
      minWidth: 18, height: 18, padding: '0 4px', borderRadius: '6px',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      fontSize: '11px', fontWeight: 800, color: '#062420',
      background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
      boxShadow: '0 2px 6px rgba(0,0,0,0.45)',
    }}>
      {n}
    </span>
  )
}

/* ─── side panel item ─── */
export function SideItem({
  title, sub, onClick, leading,
}: {
  title: string
  sub?: string
  onClick: () => void
  leading?: (hovered: boolean) => React.ReactNode
}) {
  const [h, setH] = useState(false)
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      style={{
        width: '100%', display: 'flex', alignItems: 'center', gap: '12px',
        textAlign: 'left',
        background: h ? 'rgba(255,255,255,0.05)' : 'transparent',
        border: `1px solid ${h ? 'rgba(255,255,255,0.09)' : 'transparent'}`,
        borderRadius: '14px',
        padding: '9px 10px', marginBottom: '4px',
        cursor: 'pointer',
        transform: h ? 'translateX(3px)' : 'translateX(0)',
        transition: 'all 0.18s cubic-bezier(0.34,1.4,0.64,1)',
      }}
    >
      {leading?.(h)}
      <span style={{ minWidth: 0, flex: 1 }}>
        <span style={{
          display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden',
          fontSize: '15px', fontWeight: 700, lineHeight: 1.25, letterSpacing: '-0.01em',
          color: '#fff',
        }}>
          {title}
        </span>
        {sub && (
          <span style={{
            display: 'block', fontSize: '12px', marginTop: '3px', fontWeight: 500,
            color: h ? ACCENT : TEXT2, transition: 'color 0.18s',
          }}>
            {sub}
          </span>
        )}
      </span>
    </button>
  )
}
