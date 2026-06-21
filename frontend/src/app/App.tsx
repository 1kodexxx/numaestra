import { BrowserRouter } from 'react-router-dom'
import { Navbar } from '@widgets/navbar'
import { AppRouter } from './router/AppRouter'

export function App() {
  return (
    <BrowserRouter>
      <Navbar />
      <main>
        <AppRouter />
      </main>
      <footer className="text-center p-6 text-muted text-[13px] border-t border-border">
        © 2025 Numaestra · Персональные песни на заказ
      </footer>
    </BrowserRouter>
  )
}
