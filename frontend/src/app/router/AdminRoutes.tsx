import { Navigate, Route, Routes } from 'react-router-dom'
import { AdminLayout } from '@widgets/admin-layout'
import { lazyWithReload } from '@shared/lib/lazyWithReload'

const AdminLoginPage = lazyWithReload(() => import('@pages/admin/login/AdminLoginPage').then((m) => ({ default: m.AdminLoginPage })))
const AdminDashboardPage = lazyWithReload(() => import('@pages/admin/dashboard/AdminDashboardPage').then((m) => ({ default: m.AdminDashboardPage })))
const AdminExamplesPage = lazyWithReload(() => import('@pages/admin/examples/AdminExamplesPage').then((m) => ({ default: m.AdminExamplesPage })))
const AdminReviewsPage = lazyWithReload(() => import('@pages/admin/reviews/AdminReviewsPage').then((m) => ({ default: m.AdminReviewsPage })))
const AdminCategoriesPage = lazyWithReload(() => import('@pages/admin/categories/AdminCategoriesPage').then((m) => ({ default: m.AdminCategoriesPage })))
const AdminCategoryEditPage = lazyWithReload(() => import('@pages/admin/categories/AdminCategoryEditPage').then((m) => ({ default: m.AdminCategoryEditPage })))
const AdminOrdersPage = lazyWithReload(() => import('@pages/admin/orders/AdminOrdersPage').then((m) => ({ default: m.AdminOrdersPage })))
const AdminOrderDetailPage = lazyWithReload(() => import('@pages/admin/orders/AdminOrderDetailPage').then((m) => ({ default: m.AdminOrderDetailPage })))
const AdminGenresPage = lazyWithReload(() => import('@pages/admin/genres/AdminGenresPage').then((m) => ({ default: m.AdminGenresPage })))
const AdminAccountsPage = lazyWithReload(() => import('@pages/admin/accounts/AdminAccountsPage').then((m) => ({ default: m.AdminAccountsPage })))
const AdminPromoCodesPage = lazyWithReload(() => import('@pages/admin/promo-codes/AdminPromoCodesPage').then((m) => ({ default: m.AdminPromoCodesPage })))

export function AdminRoutes() {
  return (
    <Routes>
      <Route path="login" element={<AdminLoginPage />} />
      <Route element={<AdminLayout />}>
        <Route index element={<Navigate to="/admin/dashboard" replace />} />
        <Route path="dashboard" element={<AdminDashboardPage />} />
        <Route path="categories" element={<AdminCategoriesPage />} />
        <Route path="categories/:id" element={<AdminCategoryEditPage />} />
        <Route path="genres" element={<AdminGenresPage />} />
        <Route path="examples" element={<AdminExamplesPage />} />
        <Route path="reviews" element={<AdminReviewsPage />} />
        <Route path="orders" element={<AdminOrdersPage />} />
        <Route path="orders/:id" element={<AdminOrderDetailPage />} />
        <Route path="accounts" element={<AdminAccountsPage />} />
        <Route path="promo-codes" element={<AdminPromoCodesPage />} />
      </Route>
    </Routes>
  )
}
