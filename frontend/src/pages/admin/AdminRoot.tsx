import { Outlet } from 'react-router-dom'
import { AdminSessionProvider } from '@features/admin-session'

// Общий провайдер сессии для /admin/login и защищённых /admin/* маршрутов —
// единственная точка, где выполняется проверка текущей сессии (GET /admin/me).
export function AdminRoot() {
  return (
    <AdminSessionProvider>
      <Outlet />
    </AdminSessionProvider>
  )
}
