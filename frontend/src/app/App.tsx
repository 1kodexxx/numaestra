import { BrowserRouter, useLocation } from 'react-router-dom'
import { Navbar } from '@widgets/navbar'
import { AppRouter } from './router/AppRouter'

// /admin — отдельный раздел со своим layout (AdminLayout), публичная
// навигация и футер сайта ему не нужны.
function PublicChrome({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation()
  const isAdmin = pathname.startsWith('/admin')
  if (isAdmin) return <>{children}</>
  return (
    <>
      <Navbar />
      <main>{children}</main>
      <footer className="text-center p-6 text-muted text-[13px] border-t border-border">
        © 2025 Numaestra · Персональные песни на заказ
      </footer>
    </>
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
