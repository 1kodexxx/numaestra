import { Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import { Spinner } from '@shared/ui'
import { lazyWithReload } from '@shared/lib/lazyWithReload'
import { CatalogPage } from '@pages/catalog'
import { AdminRoot } from '@pages/admin'
import { StatusOrderSkeleton } from '@pages/status'
import { AdminRoutes } from './AdminRoutes'

const CategoryPage = lazyWithReload(() => import('@pages/category').then((m) => ({ default: m.CategoryPage })))
const ExampleDetailPage = lazyWithReload(() => import('@pages/examples').then((m) => ({ default: m.ExampleDetailPage })))
const ReviewsPage = lazyWithReload(() => import('@pages/reviews').then((m) => ({ default: m.ReviewsPage })))
const HowItWorksPage = lazyWithReload(() => import('@pages/how-it-works').then((m) => ({ default: m.HowItWorksPage })))
const SharePage = lazyWithReload(() => import('@pages/share').then((m) => ({ default: m.SharePage })))
const StatusPage = lazyWithReload(() => import('@pages/status').then((m) => ({ default: m.StatusPage })))
const LegalPage = lazyWithReload(() => import('@pages/legal').then((m) => ({ default: m.LegalPage })))
const OrderSuccessPage = lazyWithReload(() => import('@pages/payment').then((m) => ({ default: m.OrderSuccessPage })))
const OrderFailPage = lazyWithReload(() => import('@pages/payment').then((m) => ({ default: m.OrderFailPage })))
const NotFoundPage = lazyWithReload(() => import('@pages/not-found').then((m) => ({ default: m.NotFoundPage })))

function PageFallback() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '40vh' }}>
      <Spinner />
    </div>
  )
}

function StatusFallback() {
  return (
    <div style={{ maxWidth: 680, margin: '0 auto', padding: 'clamp(24px, 6vw, 40px) clamp(16px, 4vw, 24px) 60px' }}>
      <StatusOrderSkeleton />
    </div>
  )
}

function CategoryFallback() {
  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: 'clamp(24px, 6vw, 40px) clamp(16px, 4vw, 24px)', display: 'flex', gap: 24 }}>
      <div style={{ flex: '0 0 240px', display: 'flex', flexDirection: 'column', gap: 10 }}>
        {Array.from({ length: 5 }, (_, i) => (
          <div key={i} className="skeleton" style={{ height: 36, borderRadius: 10 }} />
        ))}
      </div>
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 14 }}>
        <div className="skeleton" style={{ height: 28, width: '60%', borderRadius: 8 }} />
        <div className="skeleton" style={{ height: 120, borderRadius: 12 }} />
        <div className="skeleton" style={{ height: 52, borderRadius: 12 }} />
      </div>
    </div>
  )
}

function Lazy({ children, fallback }: { children: React.ReactNode; fallback?: React.ReactNode }) {
  return <Suspense fallback={fallback ?? <PageFallback />}>{children}</Suspense>
}

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<CatalogPage />} />
      <Route path="/category/:id" element={<Lazy fallback={<CategoryFallback />}><CategoryPage /></Lazy>} />
      <Route path="/examples/:id" element={<Lazy><ExampleDetailPage /></Lazy>} />
      <Route path="/reviews" element={<Lazy><ReviewsPage /></Lazy>} />
      <Route path="/how-it-works" element={<Lazy><HowItWorksPage /></Lazy>} />
      <Route path="/s/:id" element={<Lazy><SharePage /></Lazy>} />
      <Route path="/status/:orderId" element={<Lazy fallback={<StatusFallback />}><StatusPage /></Lazy>} />
      <Route path="/status" element={<Lazy fallback={<StatusFallback />}><StatusPage /></Lazy>} />
      <Route path="/order/success" element={<Lazy><OrderSuccessPage /></Lazy>} />
      <Route path="/order/fail" element={<Lazy><OrderFailPage /></Lazy>} />
      <Route path="/legal/:slug" element={<Lazy><LegalPage /></Lazy>} />

      <Route path="/admin/*" element={<AdminRoot />}>
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
