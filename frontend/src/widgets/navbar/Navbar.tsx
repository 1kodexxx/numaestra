import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { BrandMark } from '@shared/ui'

/* Текстовая ссылка навбара с подсветкой активного маршрута. */
function NavTextLink({ to, label, active }: { to: string; label: string; active: boolean }) {
  return (
    <Link
      to={to}
      className="state-layer nav-pill"
      style={{
        textDecoration: 'none',
        fontSize: '13px',
        fontWeight: 600,
        color: active ? '#00e5c0' : 'rgba(255,255,255,0.7)',
        display: 'inline-flex',
        alignItems: 'center',
        height: '38px',
        padding: '0 12px',
        borderRadius: '20px',
        transition: 'color 0.15s',
      }}
      onMouseEnter={(e) => { if (!active) e.currentTarget.style.color = '#fff' }}
      onMouseLeave={(e) => { if (!active) e.currentTarget.style.color = 'rgba(255,255,255,0.7)' }}
    >
      {label}
    </Link>
  )
}

export function Navbar() {
  const { pathname } = useLocation()
  const [wide, setWide] = useState(() => (typeof window !== 'undefined' ? window.innerWidth >= 640 : true))

  useEffect(() => {
    const fn = () => setWide(window.innerWidth >= 640)
    window.addEventListener('resize', fn)
    return () => window.removeEventListener('resize', fn)
  }, [])

  return (
    <nav
      className="nav-enter"
      style={{
        position: 'sticky',
        top: 0,
        zIndex: 50,
        height: '60px',
        display: 'flex',
        alignItems: 'center',
        padding: '0 28px',
        background: 'rgba(8,8,8,0.85)',
        backdropFilter: 'blur(24px)',
        WebkitBackdropFilter: 'blur(24px)',
        borderBottom: '1px solid rgba(255,255,255,0.05)',
      }}
    >
      <Link to="/" className="brand-link" style={{ textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '10px' }}>
        <BrandMark size={26} />
        <span style={{ fontSize: '17px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em' }}>
          Numaestra
        </span>
      </Link>

      <div style={{ flex: 1 }} />

      {/* Вторичные ссылки — на мобильных скрыты (доступны в футере). */}
      {wide && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '2px', marginRight: '8px' }}>
          <NavTextLink to="/how-it-works" label="Как это работает" active={pathname === '/how-it-works'} />
          <NavTextLink to="/reviews" label="Отзывы" active={pathname === '/reviews'} />
        </div>
      )}

      <Link
        to="/status"
        className="state-layer nav-pill"
        style={{
          textDecoration: 'none',
          fontSize: '13px',
          fontWeight: 600,
          color: pathname === '/status' ? '#00e5c0' : 'rgba(255,255,255,0.7)',
          display: 'inline-flex',
          alignItems: 'center',
          height: '38px',
          padding: '0 18px',
          borderRadius: '20px',
          border: '1px solid',
          borderColor: pathname === '/status' ? 'rgba(0,229,192,0.35)' : 'rgba(255,255,255,0.12)',
          transition: 'all 0.15s',
        }}
        onMouseEnter={(e) => {
          if (pathname !== '/status') {
            e.currentTarget.style.color = '#fff'
            e.currentTarget.style.borderColor = 'rgba(255,255,255,0.22)'
          }
        }}
        onMouseLeave={(e) => {
          if (pathname !== '/status') {
            e.currentTarget.style.color = 'rgba(255,255,255,0.7)'
            e.currentTarget.style.borderColor = 'rgba(255,255,255,0.12)'
          }
        }}
      >
        Мой заказ
      </Link>
    </nav>
  )
}
