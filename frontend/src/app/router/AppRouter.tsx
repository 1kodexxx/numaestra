import { Routes, Route, Navigate } from 'react-router-dom'
import { CatalogPage } from '@pages/catalog'
import { CategoryPage } from '@pages/category'
import { ExampleDetailPage } from '@pages/examples'
import { ReviewsPage } from '@pages/reviews'
import { HowItWorksPage } from '@pages/how-it-works'
import { SharePage } from '@pages/share'
import { StatusPage } from '@pages/status'
import { LegalPage } from '@pages/legal'
import { NotFoundPage } from '@pages/not-found'
import {
  AdminRoot,
  AdminLoginPage,
  AdminDashboardPage,
  AdminExamplesPage,
  AdminReviewsPage,
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
      <Route path="/category/:id" element={<CategoryPage />} />
      <Route path="/examples/:id" element={<ExampleDetailPage />} />
      <Route path="/reviews" element={<ReviewsPage />} />
      <Route path="/how-it-works" element={<HowItWorksPage />} />
      <Route path="/s/:id" element={<SharePage />} />
      <Route path="/status/:orderId" element={<StatusPage />} />
      <Route path="/status" element={<StatusPage />} />
      <Route path="/order/success" element={<Navigate to="/status" replace />} />
      <Route path="/legal/:slug" element={<LegalPage />} />

      <Route path="/admin" element={<AdminRoot />}>
        <Route path="login" element={<AdminLoginPage />} />
        <Route element={<AdminLayout />}>
          <Route index element={<Navigate to="/admin/dashboard" replace />} />
          <Route path="dashboard" element={<AdminDashboardPage />} />
          <Route path="categories" element={<AdminCategoriesPage />} />
          <Route path="categories/:id" element={<AdminCategoryEditPage />} />
          <Route path="examples" element={<AdminExamplesPage />} />
          <Route path="reviews" element={<AdminReviewsPage />} />
          <Route path="orders" element={<AdminOrdersPage />} />
          <Route path="orders/:id" element={<AdminOrderDetailPage />} />
          <Route path="accounts" element={<AdminAccountsPage />} />
        </Route>
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
