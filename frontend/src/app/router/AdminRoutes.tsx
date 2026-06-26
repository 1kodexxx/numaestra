import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AdminLayout } from '@widgets/admin-layout'

const AdminLoginPage = lazy(() => import('@pages/admin/login/AdminLoginPage').then((m) => ({ default: m.AdminLoginPage })))
const AdminDashboardPage = lazy(() => import('@pages/admin/dashboard/AdminDashboardPage').then((m) => ({ default: m.AdminDashboardPage })))
const AdminExamplesPage = lazy(() => import('@pages/admin/examples/AdminExamplesPage').then((m) => ({ default: m.AdminExamplesPage })))
const AdminReviewsPage = lazy(() => import('@pages/admin/reviews/AdminReviewsPage').then((m) => ({ default: m.AdminReviewsPage })))
const AdminCategoriesPage = lazy(() => import('@pages/admin/categories/AdminCategoriesPage').then((m) => ({ default: m.AdminCategoriesPage })))
const AdminCategoryEditPage = lazy(() => import('@pages/admin/categories/AdminCategoryEditPage').then((m) => ({ default: m.AdminCategoryEditPage })))
const AdminOrdersPage = lazy(() => import('@pages/admin/orders/AdminOrdersPage').then((m) => ({ default: m.AdminOrdersPage })))
const AdminOrderDetailPage = lazy(() => import('@pages/admin/orders/AdminOrderDetailPage').then((m) => ({ default: m.AdminOrderDetailPage })))
const AdminGenresPage = lazy(() => import('@pages/admin/genres/AdminGenresPage').then((m) => ({ default: m.AdminGenresPage })))
const AdminAccountsPage = lazy(() => import('@pages/admin/accounts/AdminAccountsPage').then((m) => ({ default: m.AdminAccountsPage })))

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
      </Route>
    </Routes>
  )
}
