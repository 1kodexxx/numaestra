import { Link } from 'react-router-dom'

function WaveIcon() {
  return (
    <svg width="28" height="20" viewBox="0 0 28 20" fill="none" style={{ flexShrink: 0 }}>
      <rect x="0"  y="10" width="4" height="10" rx="2" fill="#00e5c0"/>
      <rect x="6"  y="4"  width="4" height="16" rx="2" fill="#00e5c0"/>
      <rect x="12" y="0"  width="4" height="20" rx="2" fill="#00e5c0"/>
      <rect x="18" y="5"  width="4" height="15" rx="2" fill="#00e5c0"/>
      <rect x="24" y="8"  width="4" height="12" rx="2" fill="#00e5c0"/>
    </svg>
  )
}

export function Navbar() {
  return (
    <nav
      className="sticky top-0 z-50 flex items-center gap-3 px-6"
      style={{
        height: '56px',
        background: 'rgba(13,13,13,0.95)',
        backdropFilter: 'blur(12px)',
        borderBottom: '1px solid #252525',
      }}
    >
      <Link to="/" className="flex items-center gap-2.5 no-underline select-none">
        <WaveIcon />
        <span className="text-lg font-bold text-white tracking-tight">Numaestra</span>
      </Link>
      <div className="flex-1" />
      <Link
        to="/status"
        className="text-sm no-underline px-4 py-1.5 rounded-full transition-all"
        style={{ color: '#777', border: '1px solid #252525' }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff'; e.currentTarget.style.borderColor = '#444' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = '#777'; e.currentTarget.style.borderColor = '#252525' }}
      >
        Мой заказ
      </Link>
    </nav>
  )
}
