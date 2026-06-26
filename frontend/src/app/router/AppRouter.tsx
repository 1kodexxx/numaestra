import { lazy, Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import { Spinner } from '@shared/ui'
import { CatalogPage } from '@pages/catalog'
import { AdminRoot } from '@pages/admin'
import { AdminRoutes } from './AdminRoutes'

const CategoryPage = lazy(() => import('@pages/category').then((m) => ({ default: m.CategoryPage })))
const ExampleDetailPage = lazy(() => import('@pages/examples').then((m) => ({ default: m.ExampleDetailPage })))
const ReviewsPage = lazy(() => import('@pages/reviews').then((m) => ({ default: m.ReviewsPage })))
const HowItWorksPage = lazy(() => import('@pages/how-it-works').then((m) => ({ default: m.HowItWorksPage })))
const SharePage = lazy(() => import('@pages/share').then((m) => ({ default: m.SharePage })))
const StatusPage = lazy(() => import('@pages/status').then((m) => ({ default: m.StatusPage })))
const LegalPage = lazy(() => import('@pages/legal').then((m) => ({ default: m.LegalPage })))
const OrderSuccessPage = lazy(() => import('@pages/payment').then((m) => ({ default: m.OrderSuccessPage })))
const OrderFailPage = lazy(() => import('@pages/payment').then((m) => ({ default: m.OrderFailPage })))
const NotFoundPage = lazy(() => import('@pages/not-found').then((m) => ({ default: m.NotFoundPage })))

function PageFallback() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '40vh' }}>
      <Spinner />
    </div>
  )
}

function Lazy({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<PageFallback />}>{children}</Suspense>
}

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<CatalogPage />} />
      <Route path="/category/:id" element={<Lazy><CategoryPage /></Lazy>} />
      <Route path="/examples/:id" element={<Lazy><ExampleDetailPage /></Lazy>} />
      <Route path="/reviews" element={<Lazy><ReviewsPage /></Lazy>} />
      <Route path="/how-it-works" element={<Lazy><HowItWorksPage /></Lazy>} />
      <Route path="/s/:id" element={<Lazy><SharePage /></Lazy>} />
      <Route path="/status/:orderId" element={<Lazy><StatusPage /></Lazy>} />
      <Route path="/status" element={<Lazy><StatusPage /></Lazy>} />
      <Route path="/order/success" element={<Lazy><OrderSuccessPage /></Lazy>} />
      <Route path="/order/fail" element={<Lazy><OrderFailPage /></Lazy>} />
      <Route path="/legal/:slug" element={<Lazy><LegalPage /></Lazy>} />

      <Route path="/admin" element={<AdminRoot />}>
        <Route
          path="*"
          element={(
            <Suspense fallback={<PageFallback />}>
              <AdminRoutes />
            </Suspense>
          )}
        />
      </Route>

      <Route path="*" element={<Lazy><NotFoundPage /></Lazy>} />
    </Routes>
  )
}
