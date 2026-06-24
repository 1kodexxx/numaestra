import { useState } from 'react'
import { NavLink, Navigate, Outlet, useNavigate } from 'react-router-dom'
import { useAdminSession } from '@features/admin-session'
import { Spinner } from '@shared/ui'
import { useSeo } from '@shared/lib/seo'
import { A } from './AdminUI'

const NAV = [
  { to: '/admin/dashboard', label: 'Дашборд', icon: '📊' },
  { to: '/admin/categories', label: 'Категории', icon: '🗂️' },
  { to: '/admin/examples', label: 'Примеры работ', icon: '🎧' },
  { to: '/admin/reviews', label: 'Отзывы', icon: '💬' },
  { to: '/admin/orders', label: 'Заказы', icon: '🧾' },
  { to: '/admin/accounts', label: 'Suno-аккаунты', icon: '🎚️' },
]

function NavItem({ to, label, icon }: { to: string; label: string; icon: string }) {
  const [h, setH] = useState(false)
  return (
    <NavLink to={to} style={{ textDecoration: 'none' }}>
      {({ isActive }) => (
        <div
          onMouseEnter={() => setH(true)}
          onMouseLeave={() => setH(false)}
          style={{
            display: 'flex', alignItems: 'center', gap: '12px',
            padding: '11px 13px', borderRadius: '12px', marginBottom: '4px',
            position: 'relative',
            background: isActive ? 'rgba(0,229,192,0.12)' : h ? 'rgba(255,255,255,0.05)' : 'transparent',
            border: `1px solid ${isActive ? 'rgba(0,229,192,0.28)' : 'transparent'}`,
            transition: 'all 0.15s',
          }}
        >
          {isActive && (
            <span style={{ position: 'absolute', left: 0, top: '22%', height: '56%', width: '3px', borderRadius: '2px', background: A.accent }} />
          )}
          <span style={{ fontSize: '16px', lineHeight: 1 }}>{icon}</span>
          <span style={{
            fontSize: '14px', fontWeight: isActive ? 700 : 500,
            color: isActive ? A.accent : h ? A.txt : A.txt2, transition: 'color 0.15s',
          }}>
            {label}
          </span>
        </div>
      )}
    </NavLink>
  )
}

export function AdminLayout() {
  const { login, loading, signOut } = useAdminSession()
  const navigate = useNavigate()
  const [logoutH, setLogoutH] = useState(false)

  useSeo({ title: 'Админ-панель', noindex: true })

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', background: A.bg }}>
        <Spinner />
      </div>
    )
  }
  if (!login) return <Navigate to="/admin/login" replace />

  async function handleLogout() {
    await signOut()
    navigate('/admin/login', { replace: true })
  }

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: A.bg, color: A.txt }}>
      <aside style={{
        width: 256, flexShrink: 0,
        borderRight: `1px solid ${A.border}`,
        background: A.surface,
        display: 'flex', flexDirection: 'column',
        padding: '22px 16px',
        position: 'sticky', top: 0, height: '100vh',
      }}>
        {/* Brand */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '0 6px', marginBottom: '28px' }}>
          <span style={{
            width: 34, height: 34, borderRadius: '10px',
            background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: '17px', boxShadow: '0 4px 14px rgba(0,229,192,0.3)',
          }}>🎵</span>
          <div>
            <div style={{ fontSize: '16px', fontWeight: 800, letterSpacing: '-0.02em', lineHeight: 1 }}>Numaestra</div>
            <div style={{ fontSize: '11px', color: A.txt3, fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase', marginTop: '3px' }}>Admin</div>
          </div>
        </div>

        <nav>
          {NAV.map(n => <NavItem key={n.to} {...n} />)}
        </nav>

        <div style={{ flex: 1 }} />

        {/* User + logout */}
        <div style={{ borderTop: `1px solid ${A.border}`, paddingTop: '14px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '0 6px 12px' }}>
            <span style={{
              width: 32, height: 32, borderRadius: '50%', flexShrink: 0,
              background: 'rgba(0,229,192,0.12)', border: '1px solid rgba(0,229,192,0.3)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: '13px', fontWeight: 700, color: A.accent,
            }}>
              {login.slice(0, 1).toUpperCase()}
            </span>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: '10px', color: A.txt3, textTransform: 'uppercase', letterSpacing: '0.06em' }}>Вы вошли</div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: A.txt, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{login}</div>
            </div>
          </div>
          <button
            onClick={handleLogout}
            onMouseEnter={() => setLogoutH(true)}
            onMouseLeave={() => setLogoutH(false)}
            style={{
              width: '100%', padding: '10px', borderRadius: '12px',
              background: logoutH ? 'rgba(239,68,68,0.1)' : 'transparent',
              border: `1px solid ${logoutH ? 'rgba(239,68,68,0.35)' : A.border}`,
              color: logoutH ? '#f87171' : A.txt2,
              fontSize: '13px', fontWeight: 600, fontFamily: 'inherit', cursor: 'pointer',
              transition: 'all 0.15s',
            }}
          >
            Выйти
          </button>
        </div>
      </aside>

      <main style={{ flex: 1, minWidth: 0, padding: '36px 40px', overflowX: 'auto' }}>
        <Outlet />
      </main>
    </div>
  )
}
