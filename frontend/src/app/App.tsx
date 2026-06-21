import { BrowserRouter, useLocation } from 'react-router-dom'
import { Navbar } from '@widgets/navbar'
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
        {children}
      </div>
      {!isFullscreen && (
        <footer
          className="text-center p-4 text-xs"
          style={{ color: '#444', borderTop: '1px solid #1a1a1a' }}
        >
          © 2025 Numaestra · Персональные песни на заказ
        </footer>
      )}
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
