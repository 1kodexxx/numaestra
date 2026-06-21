import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { useAdminSession } from '@features/admin-session'
import { ApiError } from '@shared/api'

const inputCls = [
  'w-full bg-bg3 border border-border rounded-lg px-4 py-3',
  'text-txt text-[15px] outline-none transition-colors',
  'focus:border-accent2 placeholder:text-muted',
].join(' ')

export function AdminLoginPage() {
  const { login: currentLogin, loading, signIn } = useAdminSession()
  const navigate = useNavigate()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  if (!loading && currentLogin) return <Navigate to="/admin/categories" replace />

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await signIn(login, password)
      navigate('/admin/categories', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось войти')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-6">
      <form onSubmit={handleSubmit} className="w-full max-w-sm bg-bg2 border border-border rounded-xl p-8">
        <div className="text-xl font-extrabold mb-1 bg-linear-to-br from-accent to-gold bg-clip-text text-transparent">
          Numaestra Admin
        </div>
        <div className="text-sm text-muted mb-6">Вход для администратора</div>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5 text-sm text-muted">
            <label htmlFor="admin-login">Логин</label>
            <input
              id="admin-login"
              className={inputCls}
              value={login}
              onChange={(e) => setLogin(e.target.value)}
              autoComplete="username"
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5 text-sm text-muted">
            <label htmlFor="admin-password">Пароль</label>
            <input
              id="admin-password"
              type="password"
              className={inputCls}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </div>

          {error && (
            <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={submitting || !login || !password}
            className="mt-2 py-3 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] disabled:opacity-60 disabled:cursor-not-allowed transition-all"
          >
            {submitting ? 'Входим...' : 'Войти'}
          </button>
        </div>
      </form>
    </div>
  )
}
