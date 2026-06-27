import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { BrandMark } from '@shared/ui'
import { theme } from '@shared/lib/theme'
import { useFocusTrap } from '@shared/lib/useFocusTrap'

const ACCENT = theme.accent

const NAV_ITEMS = [
  { to: '/', label: 'Каталог', isActive: (path: string) => path === '/' || path.startsWith('/category') },
  { to: '/how-it-works', label: 'Как это работает', isActive: (path: string) => path === '/how-it-works' },
  { to: '/reviews', label: 'Отзывы', isActive: (path: string) => path === '/reviews' },
] as const

function NavTextLink({ to, label, active, onNavigate }: { to: string; label: string; active: boolean; onNavigate?: () => void }) {
  return (
    <Link
      to={to}
      onClick={onNavigate}
      className="state-layer nav-pill"
      style={{
        textDecoration: 'none',
        fontSize: '13px',
        fontWeight: 600,
        color: active ? ACCENT : 'rgba(255,255,255,0.7)',
        display: 'inline-flex',
        alignItems: 'center',
        height: '38px',
        padding: '0 14px',
        borderRadius: '20px',
        background: active ? 'rgba(0,229,192,0.1)' : 'transparent',
        border: `1px solid ${active ? 'rgba(0,229,192,0.22)' : 'transparent'}`,
        boxShadow: active ? '0 2px 12px -4px rgba(0,229,192,0.45)' : 'none',
        transition: 'all 0.18s',
      }}
      onMouseEnter={(e) => { if (!active) { e.currentTarget.style.color = '#fff'; e.currentTarget.style.background = 'rgba(255,255,255,0.05)' } }}
      onMouseLeave={(e) => { if (!active) { e.currentTarget.style.color = 'rgba(255,255,255,0.7)'; e.currentTarget.style.background = 'transparent' } }}
    >
      {label}
    </Link>
  )
}

function MenuIcon({ open }: { open: boolean }) {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden>
      {open ? (
        <>
          <path d="M6 6l12 12M18 6L6 18" />
        </>
      ) : (
        <>
          <path d="M4 7h16M4 12h16M4 17h16" />
        </>
      )}
    </svg>
  )
}

function TicketIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M4 9V6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v3a2 2 0 0 0 0 4v3a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-3a2 2 0 0 0 0-4Z" />
      <path d="M9 4v16" strokeDasharray="2 3" />
    </svg>
  )
}

/** «Мой заказ» — акцентный CTA: при активной странице залит градиентом, иначе акцентный outline. */
function StatusOrderLink({ active, compact, onNavigate }: { active: boolean; compact?: boolean; onNavigate?: () => void }) {
  return (
    <Link
      to="/status"
      onClick={onNavigate}
      className="state-layer nav-pill"
      style={{
        textDecoration: 'none',
        fontSize: '13px',
        fontWeight: 700,
        color: active ? '#04130f' : 'rgba(0,229,192,0.92)',
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '7px',
        height: '38px',
        padding: compact ? '0 16px' : '0 18px',
        borderRadius: '20px',
        background: active ? 'linear-gradient(135deg, #00e5c0, #00bfa5)' : 'rgba(0,229,192,0.08)',
        border: `1px solid ${active ? 'transparent' : 'rgba(0,229,192,0.3)'}`,
        boxShadow: active ? '0 6px 18px -6px rgba(0,229,192,0.55)' : 'none',
        transition: 'all 0.18s',
        whiteSpace: 'nowrap',
      }}
      onMouseEnter={(e) => {
        if (!active) {
          e.currentTarget.style.background = 'rgba(0,229,192,0.16)'
          e.currentTarget.style.borderColor = 'rgba(0,229,192,0.5)'
          e.currentTarget.style.color = '#00e5c0'
        }
      }}
      onMouseLeave={(e) => {
        if (!active) {
          e.currentTarget.style.background = 'rgba(0,229,192,0.08)'
          e.currentTarget.style.borderColor = 'rgba(0,229,192,0.3)'
          e.currentTarget.style.color = 'rgba(0,229,192,0.92)'
        }
      }}
    >
      <TicketIcon />
      Мой заказ
    </Link>
  )
}

export function Navbar() {
  const { pathname } = useLocation()
  const [wide, setWide] = useState(() => (typeof window !== 'undefined' ? window.innerWidth >= 640 : true))
  const [menuOpen, setMenuOpen] = useState(false)
  const [scrolled, setScrolled] = useState(false)
  const menuTrapRef = useFocusTrap(menuOpen && !wide)

  // Scroll-aware шапка: у верха почти прозрачная, при скролле — плотнее, с тенью
  // и более ярким акцентным хайрлайном. Премиальный «приклеивающийся» эффект.
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  useEffect(() => {
    const fn = () => {
      const nextWide = window.innerWidth >= 640
      setWide(nextWide)
      if (nextWide) setMenuOpen(false)
    }
    window.addEventListener('resize', fn)
    return () => window.removeEventListener('resize', fn)
  }, [])

  useEffect(() => {
    setMenuOpen(false)
  }, [pathname])

  useEffect(() => {
    if (!menuOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [menuOpen])

  useEffect(() => {
    if (!menuOpen) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [menuOpen])

  const statusActive = pathname === '/status' || pathname.startsWith('/status/')

  return (
    <>
      <nav
        className="nav-enter site-navbar"
        style={{
          position: 'sticky',
          top: 0,
          zIndex: 50,
          height: '60px',
          display: 'flex',
          alignItems: 'center',
          padding: '0 clamp(16px, 4vw, 28px)',
          paddingTop: 'env(safe-area-inset-top, 0px)',
          background: scrolled ? 'rgba(8,8,8,0.92)' : 'rgba(8,8,8,0.6)',
          backdropFilter: `blur(${scrolled ? 28 : 16}px) saturate(140%)`,
          WebkitBackdropFilter: `blur(${scrolled ? 28 : 16}px) saturate(140%)`,
          boxShadow: scrolled ? '0 12px 36px -20px rgba(0,0,0,0.75)' : 'none',
          transition: 'background 0.28s ease, box-shadow 0.28s ease, backdrop-filter 0.28s ease',
        }}
      >
        {/* Градиентный хайрлайн снизу — тонкий акцентный край, ярче при скролле */}
        <span
          aria-hidden
          style={{
            position: 'absolute',
            left: 0,
            right: 0,
            bottom: 0,
            height: '1px',
            background: 'linear-gradient(90deg, transparent, rgba(0,229,192,0.4), transparent)',
            opacity: scrolled ? 1 : 0.45,
            transition: 'opacity 0.28s ease',
          }}
        />

        <Link to="/" className="brand-link" style={{ textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
          <span style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
            <span
              aria-hidden
              style={{
                position: 'absolute',
                inset: '-7px',
                background: 'radial-gradient(circle, rgba(0,229,192,0.28), transparent 70%)',
                filter: 'blur(7px)',
                opacity: 0.75,
                pointerEvents: 'none',
              }}
            />
            <BrandMark size={26} />
          </span>
          <span style={{ fontSize: '17px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em' }}>
            Numaestra
          </span>
        </Link>

        <div style={{ flex: 1 }} />

        {wide && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '2px', marginRight: '8px' }}>
            {NAV_ITEMS.slice(1).map((item) => (
              <NavTextLink
                key={item.to}
                to={item.to}
                label={item.label}
                active={item.isActive(pathname)}
              />
            ))}
          </div>
        )}

        {!wide && (
          <button
            type="button"
            className="mobile-nav-toggle state-layer chip-press"
            aria-expanded={menuOpen}
            aria-controls="mobile-nav-sheet"
            aria-label={menuOpen ? 'Закрыть меню' : 'Открыть меню'}
            onClick={() => setMenuOpen((v) => !v)}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '40px',
              height: '40px',
              marginRight: '6px',
              borderRadius: '12px',
              border: '1px solid rgba(255,255,255,0.1)',
              background: menuOpen ? 'rgba(0,229,192,0.1)' : 'rgba(255,255,255,0.04)',
              color: menuOpen ? ACCENT : 'rgba(255,255,255,0.85)',
              cursor: 'pointer',
            }}
          >
            <MenuIcon open={menuOpen} />
          </button>
        )}

        {wide && <StatusOrderLink active={statusActive} />}
      </nav>

      {!wide && menuOpen && (
        <>
          <button
            type="button"
            className="mobile-nav-backdrop modal-backdrop"
            aria-label="Закрыть меню"
            onClick={() => setMenuOpen(false)}
          />
          <div id="mobile-nav-sheet" ref={menuTrapRef} className="mobile-nav-sheet slide-up-in" role="dialog" aria-modal="true" aria-label="Навигация">
            <div className="mobile-nav-sheet-handle" aria-hidden />
            <nav className="mobile-nav-links">
              {NAV_ITEMS.map((item) => {
                const active = item.isActive(pathname)
                return (
                  <Link
                    key={item.to}
                    to={item.to}
                    onClick={() => setMenuOpen(false)}
                    className={`mobile-nav-link state-layer${active ? ' mobile-nav-link--active' : ''}`}
                  >
                    <span>{item.label}</span>
                    {active && <span className="mobile-nav-link-dot" aria-hidden />}
                  </Link>
                )
              })}
            </nav>
            <div className="mobile-nav-sheet-footer">
              <StatusOrderLink active={statusActive} onNavigate={() => setMenuOpen(false)} />
            </div>
          </div>
        </>
      )}
    </>
  )
}
