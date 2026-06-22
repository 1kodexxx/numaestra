import { BrowserRouter, useLocation } from 'react-router-dom'
import { Navbar } from '@widgets/navbar'
import { Footer } from '@widgets/footer'
import { AppRouter } from './router/AppRouter'

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
  return (
    <BrowserRouter>
      <PublicChrome>
        <AppRouter />
      </PublicChrome>
    </BrowserRouter>
  )
}
