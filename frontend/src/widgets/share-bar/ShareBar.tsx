import { copyText } from '@shared/ui'

interface ShareBarProps {
  url: string
  text: string
}

const ACCENT = '#00e5c0'

type Net = { key: string; label: string; color: string; href: (u: string, t: string) => string }

// Сети с реальными web-share интентами. TikTok, MAX, Instagram и пр. не имеют
// стабильных URL-интентов в вебе — их закрывает нативное «Поделиться» (ниже).
const NETWORKS: Net[] = [
  { key: 'tg', label: 'Telegram', color: '#2aabee', href: (u, t) => `https://t.me/share/url?url=${u}&text=${t}` },
  { key: 'vk', label: 'VK', color: '#0077ff', href: (u, t) => `https://vk.com/share.php?url=${u}&title=${t}` },
  { key: 'wa', label: 'WhatsApp', color: '#25d366', href: (u, t) => `https://wa.me/?text=${t}%20${u}` },
  { key: 'ok', label: 'OK', color: '#ee8208', href: (u, t) => `https://connect.ok.ru/offer?url=${u}&title=${t}` },
]

function ShareButton({ label, color, onClick, href }: { label: string; color: string; onClick?: () => void; href?: string }) {
  const common: React.CSSProperties = {
    display: 'inline-flex', alignItems: 'center', gap: '7px',
    padding: '9px 14px', borderRadius: '12px', cursor: 'pointer',
    background: 'rgba(255,255,255,0.04)', border: `1px solid ${color}55`,
    color: '#fff', fontSize: '13px', fontWeight: 600, fontFamily: 'inherit',
    textDecoration: 'none', transition: 'all 0.15s', whiteSpace: 'nowrap',
  }
  const dot = <span style={{ width: 8, height: 8, borderRadius: '50%', background: color, flexShrink: 0 }} />
  const onEnter = (e: React.MouseEvent<HTMLElement>) => { e.currentTarget.style.background = `${color}1f`; e.currentTarget.style.borderColor = color }
  const onLeave = (e: React.MouseEvent<HTMLElement>) => { e.currentTarget.style.background = 'rgba(255,255,255,0.04)'; e.currentTarget.style.borderColor = `${color}55` }

  if (href) {
    return <a href={href} target="_blank" rel="noopener noreferrer" style={common} onMouseEnter={onEnter} onMouseLeave={onLeave}>{dot}{label}</a>
  }
  return <button onClick={onClick} style={common} onMouseEnter={onEnter} onMouseLeave={onLeave}>{dot}{label}</button>
}

export function ShareBar({ url, text }: ShareBarProps) {
  const u = encodeURIComponent(url)
  const t = encodeURIComponent(text)
  const canNativeShare = typeof navigator !== 'undefined' && !!navigator.share

  function nativeShare() {
    navigator.share?.({ title: 'Numaestra', text, url }).catch(() => {})
  }

  return (
    <div style={{
      background: '#0f0f0f', border: '1px solid rgba(255,255,255,0.07)',
      borderRadius: '20px', padding: '20px 22px',
    }}>
      <div style={{ fontSize: '11px', fontWeight: 700, color: 'rgba(255,255,255,0.32)', letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '14px' }}>
        Поделиться песней
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
        {canNativeShare && (
          <button
            onClick={nativeShare}
            style={{
              display: 'inline-flex', alignItems: 'center', gap: '7px',
              padding: '9px 16px', borderRadius: '12px', cursor: 'pointer',
              background: 'linear-gradient(135deg, #00e5c0, #00bfa5)', border: 'none',
              color: '#062420', fontSize: '13px', fontWeight: 700, fontFamily: 'inherit',
            }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#062420" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="18" cy="5" r="3" /><circle cx="6" cy="12" r="3" /><circle cx="18" cy="19" r="3" />
              <path d="m8.6 13.5 6.8 4M15.4 6.5l-6.8 4" />
            </svg>
            Поделиться
          </button>
        )}
        {NETWORKS.map((n) => (
          <ShareButton key={n.key} label={n.label} color={n.color} href={n.href(u, t)} />
        ))}
        <ShareButton label="Скопировать ссылку" color={ACCENT} onClick={() => copyText(url, 'Ссылка скопирована')} />
      </div>
      {canNativeShare && (
        <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.3)', marginTop: '12px' }}>
          «Поделиться» откроет TikTok, MAX, Instagram и другие установленные приложения.
        </div>
      )}
    </div>
  )
}
