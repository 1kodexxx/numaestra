import { useEffect } from 'react'
import { BrowserRouter, useLocation } from 'react-router-dom'
import { Navbar } from '@widgets/navbar'
import { Footer } from '@widgets/footer'
import { AppRouter } from './router/AppRouter'
import { ErrorBoundary } from './ErrorBoundary'
import { usePublicConfig } from '@shared/lib/usePublicConfig'

/* Структурированные данные (JSON-LD) для поисковиков — инжектим один раз
   с актуальным origin, поэтому корректно при любом домене. */
function useStructuredData() {
  const { price_kopecks } = usePublicConfig()

  useEffect(() => {
    const origin = window.location.origin
    const priceRub = String(Math.round(price_kopecks / 100))
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
        description: 'Уникальная песня под ваш повод — 4 готовые версии трека за 10 минут.',
        offers: { '@type': 'Offer', price: priceRub, priceCurrency: 'RUB' },
      },
    ]
    const el = document.getElementById('ld-json') as HTMLScriptElement | null
    const s = el ?? document.createElement('script')
    s.id = 'ld-json'
    s.type = 'application/ld+json'
    s.text = JSON.stringify(data)
    if (!el) document.head.appendChild(s)
  }, [price_kopecks])
}

function PublicChrome({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation()
  const isAdmin = pathname.startsWith('/admin')
  const isFullscreen = pathname === '/' || pathname.startsWith('/category/')

  if (isAdmin) {
    return (
      <div key={pathname} className="route-fade" style={{ minHeight: '100%' }}>
        {children}
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Navbar />
      <div style={{ flex: 1, minHeight: 0, overflow: isFullscreen ? 'hidden' : 'auto' }}>
        {/* Перемонтируем по pathname → проигрывается плавное появление на каждой навигации */}
        <div key={pathname} className={isFullscreen ? 'route-fade' : 'route-fade-up'} style={{ minHeight: '100%' }}>
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
