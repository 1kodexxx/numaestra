import { Link } from 'react-router-dom'

export function Navbar() {
  return (
    <nav className="sticky top-0 z-100 flex items-center gap-4 px-6 py-3.5 bg-[rgba(13,13,26,0.92)] backdrop-blur-md border-b border-border">
      <Link
        to="/"
        className="text-[22px] font-extrabold bg-linear-to-br from-accent to-gold bg-clip-text text-transparent no-underline select-none"
      >
        🎵 Numaestra
      </Link>
      <div className="flex-1" />
      <Link to="/" className="text-muted no-underline text-sm transition-colors hover:text-txt">
        Каталог
      </Link>
      <Link to="/status" className="text-muted no-underline text-sm transition-colors hover:text-txt">
        Мой заказ
      </Link>
    </nav>
  )
}
