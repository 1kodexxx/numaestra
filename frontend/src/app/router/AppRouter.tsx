import { Routes, Route, Navigate } from 'react-router-dom'
import { CatalogPage } from '@pages/catalog'
import { QuizPage } from '@pages/quiz'
import { StatusPage } from '@pages/status'
import {
  AdminRoot,
  AdminLoginPage,
  AdminCategoriesPage,
  AdminCategoryEditPage,
  AdminOrdersPage,
  AdminOrderDetailPage,
  AdminAccountsPage,
} from '@pages/admin'
import { AdminLayout } from '@widgets/admin-layout'

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<CatalogPage />} />
      <Route path="/quiz/:categoryId" element={<QuizPage />} />
      <Route path="/status" element={<StatusPage />} />
      {/* После возврата с Robokassa — сразу на статус */}
      <Route path="/order/success" element={<Navigate to="/status" replace />} />

      <Route path="/admin" element={<AdminRoot />}>
        <Route path="login" element={<AdminLoginPage />} />
        <Route element={<AdminLayout />}>
          <Route index element={<Navigate to="/admin/categories" replace />} />
          <Route path="categories" element={<AdminCategoriesPage />} />
          <Route path="categories/:id" element={<AdminCategoryEditPage />} />
          <Route path="orders" element={<AdminOrdersPage />} />
          <Route path="orders/:id" element={<AdminOrderDetailPage />} />
          <Route path="accounts" element={<AdminAccountsPage />} />
        </Route>
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
