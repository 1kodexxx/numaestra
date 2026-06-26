import { copyText } from '@shared/ui'
import { theme } from '@shared/lib/theme'
import { GOALS, reachGoal } from '@shared/lib/analytics'

interface ShareBarProps {
  url: string
  text: string
}

const ACCENT = theme.accent

type Net = { key: string; label: string; color: string; href: (u: string, t: string) => string }

const NETWORKS: Net[] = [
  { key: 'tg', label: 'Telegram', color: '#2aabee', href: (u, t) => `https://t.me/share/url?url=${u}&text=${t}` },
  { key: 'vk', label: 'VK', color: '#0077ff', href: (u, t) => `https://vk.com/share.php?url=${u}&title=${t}` },
  { key: 'wa', label: 'WhatsApp', color: '#25d366', href: (u, t) => `https://wa.me/?text=${t}%20${u}` },
  { key: 'ok', label: 'OK', color: '#ee8208', href: (u, t) => `https://connect.ok.ru/offer?url=${u}&title=${t}` },
]

const canNativeShare = typeof navigator !== 'undefined' && typeof navigator.share === 'function'

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
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        style={common}
        onClick={onClick}
        onMouseEnter={onEnter}
        onMouseLeave={onLeave}
      >
        {dot}{label}
      </a>
    )
  }
  return <button type="button" onClick={onClick} style={common} onMouseEnter={onEnter} onMouseLeave={onLeave}>{dot}{label}</button>
}

export function ShareBar({ url, text }: ShareBarProps) {
  const u = encodeURIComponent(url)
  const t = encodeURIComponent(text)

  async function nativeShare() {
    try {
      await navigator.share({ title: 'Numaestra', text, url })
      reachGoal(GOALS.SHARE, { method: 'native' })
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') return
      copyText(url, 'Ссылка скопирована')
      reachGoal(GOALS.SHARE, { method: 'copy_fallback' })
    }
  }

  function trackNetwork(key: string) {
    reachGoal(GOALS.SHARE, { method: key })
  }

  function copyLink() {
    copyText(url, 'Ссылка скопирована')
    reachGoal(GOALS.SHARE, { method: 'copy' })
  }

  return (
    <div style={{
      background: theme.surface, border: `1px solid ${theme.border}`,
      borderRadius: '20px', padding: '20px 22px',
    }}>
      <div style={{ fontSize: '11px', fontWeight: 700, color: theme.text3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '14px' }}>
        Поделиться песней
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
        {canNativeShare && (
          <ShareButton label="Поделиться…" color={ACCENT} onClick={nativeShare} />
        )}
        {NETWORKS.map((n) => (
          <ShareButton
            key={n.key}
            label={n.label}
            color={n.color}
            href={n.href(u, t)}
            onClick={() => trackNetwork(n.key)}
          />
        ))}
        <ShareButton label="Скопировать ссылку" color={ACCENT} onClick={copyLink} />
      </div>
    </div>
  )
}
