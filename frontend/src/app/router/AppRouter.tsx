import { Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import { Spinner } from '@shared/ui'
import { CatalogPage } from '@pages/catalog'
import { CategoryPage } from '@pages/category'
import { ExampleDetailPage } from '@pages/examples'
import { ReviewsPage } from '@pages/reviews'
import { HowItWorksPage } from '@pages/how-it-works'
import { SharePage } from '@pages/share'
import { StatusPage } from '@pages/status'
import { LegalPage } from '@pages/legal'
import { OrderSuccessPage, OrderFailPage } from '@pages/payment'
import { NotFoundPage } from '@pages/not-found'
import { AdminRoot } from '@pages/admin'
import { AdminRoutes } from './AdminRoutes'

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
      <Route path="/order/success" element={<OrderSuccessPage />} />
      <Route path="/order/fail" element={<OrderFailPage />} />
      <Route path="/legal/:slug" element={<LegalPage />} />

      <Route path="/admin" element={<AdminRoot />}>
        <Route
          path="*"
          element={(
            <Suspense fallback={(
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '40vh' }}>
                <Spinner />
              </div>
            )}
            >
              <AdminRoutes />
            </Suspense>
          )}
        />
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
