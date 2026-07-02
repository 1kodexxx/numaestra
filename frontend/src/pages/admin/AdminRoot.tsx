import { Outlet } from 'react-router-dom'
import { AdminSessionProvider } from '@features/admin-session'
import { NewOrderAlerts } from '@features/admin-order-alerts'

// Общий провайдер сессии для /admin/login и защищённых /admin/* маршрутов —
// единственная точка, где выполняется проверка текущей сессии (GET /admin/me).
export function AdminRoot() {
  return (
    <AdminSessionProvider>
      {/* Звук + браузерное уведомление о новых заказах, пока открыта админка. */}
      <NewOrderAlerts />
      <Outlet />
    </AdminSessionProvider>
  )
}
