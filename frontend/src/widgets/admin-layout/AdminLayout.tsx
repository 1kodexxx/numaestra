import { NavLink, Navigate, Outlet, useNavigate } from 'react-router-dom'
import { useAdminSession } from '@features/admin-session'
import { Spinner } from '@shared/ui'

const navLinkCls = ({ isActive }: { isActive: boolean }) =>
  [
    'block px-4 py-2.5 rounded-lg text-sm font-medium no-underline transition-colors',
    isActive ? 'bg-accent/15 text-accent' : 'text-muted hover:text-txt hover:bg-bg3',
  ].join(' ')

export function AdminLayout() {
  const { login, loading, signOut } = useAdminSession()
  const navigate = useNavigate()

  if (loading) return <div className="text-center py-20"><Spinner /></div>
  if (!login) return <Navigate to="/admin/login" replace />

  async function handleLogout() {
    await signOut()
    navigate('/admin/login', { replace: true })
  }

  return (
    <div className="flex min-h-screen">
      <aside className="w-60 shrink-0 bg-bg2 border-r border-border p-5 flex flex-col gap-1">
        <div className="text-lg font-extrabold mb-6 bg-linear-to-br from-accent to-gold bg-clip-text text-transparent">
          Numaestra Admin
        </div>
        <NavLink to="/admin/categories" className={navLinkCls}>Категории</NavLink>
        <NavLink to="/admin/orders" className={navLinkCls}>Заказы</NavLink>
        <NavLink to="/admin/accounts" className={navLinkCls}>Suno-аккаунты</NavLink>

        <div className="flex-1" />

        <div className="text-xs text-muted mb-2 px-4">Вы вошли как {login}</div>
        <button
          className="px-4 py-2.5 rounded-lg text-sm font-medium bg-transparent border border-border text-muted cursor-pointer hover:text-error hover:border-error transition-colors"
          onClick={handleLogout}
        >
          Выйти
        </button>
      </aside>
      <main className="flex-1 p-8 overflow-x-auto">
        <Outlet />
      </main>
    </div>
  )
}
