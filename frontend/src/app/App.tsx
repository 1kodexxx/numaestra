import { useEffect } from 'react'
import { BrowserRouter, useLocation } from 'react-router-dom'
import { Navbar } from '@widgets/navbar'
import { Footer } from '@widgets/footer'
import { AppRouter } from './router/AppRouter'
import { ErrorBoundary } from './ErrorBoundary'

/* Структурированные данные (JSON-LD) для поисковиков — инжектим один раз
   с актуальным origin, поэтому корректно при любом домене. */
function useStructuredData() {
  useEffect(() => {
    if (document.getElementById('ld-json')) return
    const origin = window.location.origin
    const data = [
      {
        '@context': 'https://schema.org',
        '@type': 'Organization',
        name: 'Numaestra',
        url: origin,
        logo: origin + '/favicon.svg',
        description: 'AI-студия персональных песен на заказ.',
      },
      {
        '@context': 'https://schema.org',
        '@type': 'WebSite',
        name: 'Numaestra',
        url: origin,
        inLanguage: 'ru-RU',
      },
      {
        '@context': 'https://schema.org',
        '@type': 'Service',
        name: 'Персональная песня на заказ',
        serviceType: 'Создание музыки на заказ',
        provider: { '@type': 'Organization', name: 'Numaestra', url: origin },
        areaServed: 'RU',
        description: 'Уникальная песня под ваш повод — 4 готовые версии трека за 24 часа.',
        offers: { '@type': 'Offer', price: '2000', priceCurrency: 'RUB' },
      },
    ]
    const s = document.createElement('script')
    s.id = 'ld-json'
    s.type = 'application/ld+json'
    s.text = JSON.stringify(data)
    document.head.appendChild(s)
  }, [])
}

function PublicChrome({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation()
  const isAdmin = pathname.startsWith('/admin')
  const isFullscreen = pathname === '/' || pathname.startsWith('/category/')

  if (isAdmin) return <>{children}</>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Navbar />
      <div style={{ flex: 1, minHeight: 0, overflow: isFullscreen ? 'hidden' : 'auto' }}>
        {/* Перемонтируем по pathname → проигрывается плавное появление на каждой навигации */}
        <div key={pathname} className="route-fade" style={{ minHeight: '100%' }}>
          {children}
          {!isFullscreen && <Footer />}
        </div>
      </div>
    </div>
  )
}

export function App() {
  useStructuredData()
  return (
    <ErrorBoundary>
      <BrowserRouter>
        <PublicChrome>
          <AppRouter />
        </PublicChrome>
      </BrowserRouter>
    </ErrorBoundary>
  )
}
