import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { useAdminSession } from '@features/admin-session'
import { ApiError } from '@shared/api'
import { Button, TextField } from '@shared/ui'

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
      <form onSubmit={handleSubmit} className="w-full max-w-sm bg-bg2 border border-border rounded-2xl p-8 elevation-2 scale-in">
        <div className="text-xl font-extrabold mb-1 bg-linear-to-br from-accent to-gold bg-clip-text text-transparent">
          Numaestra Admin
        </div>
        <div className="text-sm text-muted mb-7">Вход для администратора</div>

        <div className="flex flex-col gap-5">
          <TextField
            label="Логин"
            value={login}
            onChange={setLogin}
            autoComplete="username"
            autoFocus
            surfaceColor="#121212"
          />
          <TextField
            label="Пароль"
            type="password"
            value={password}
            onChange={setPassword}
            autoComplete="current-password"
            surfaceColor="#121212"
          />

          {error && (
            <div className="bg-error/10 border border-error/30 rounded-xl px-4 py-3 text-error text-sm">
              {error}
            </div>
          )}

          <Button type="submit" size="lg" fullWidth loading={submitting} disabled={!login || !password}>
            Войти
          </Button>
        </div>
      </form>
    </div>
  )
}
