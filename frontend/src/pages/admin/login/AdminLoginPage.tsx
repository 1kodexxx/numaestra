import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { useAdminSession } from '@features/admin-session'
import { ApiError } from '@shared/api'
import { Button, TextField } from '@shared/ui'
import { A, ErrorBanner } from '@widgets/admin-layout'

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
    <div style={{
      minHeight: '100vh', background: A.bg, color: A.txt,
      display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '24px',
    }}>
      {/* ambient glow */}
      <div style={{
        position: 'fixed', inset: 0, pointerEvents: 'none',
        background: 'radial-gradient(ellipse 50% 40% at 50% 0%, rgba(0,229,192,0.08) 0%, transparent 70%)',
      }} />

      <form
        onSubmit={handleSubmit}
        className="scale-in"
        style={{
          position: 'relative', width: '100%', maxWidth: '380px',
          background: A.surface, border: `1px solid ${A.border}`,
          borderRadius: '24px', padding: '36px 32px',
          boxShadow: 'var(--elevation-3)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '11px', marginBottom: '24px' }}>
          <span style={{
            width: 40, height: 40, borderRadius: '12px',
            background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: '20px', boxShadow: '0 4px 16px rgba(0,229,192,0.3)',
          }}>🎵</span>
          <div>
            <div style={{ fontSize: '18px', fontWeight: 800, letterSpacing: '-0.02em', lineHeight: 1 }}>Numaestra</div>
            <div style={{ fontSize: '11px', color: A.txt3, fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase', marginTop: '3px' }}>Панель администратора</div>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <TextField label="Логин" value={login} onChange={setLogin} autoComplete="username" autoFocus surfaceColor={A.surface} />
          <TextField label="Пароль" type="password" value={password} onChange={setPassword} autoComplete="current-password" surfaceColor={A.surface} />

          {error && <ErrorBanner>{error}</ErrorBanner>}

          <Button type="submit" size="lg" fullWidth loading={submitting} disabled={!login || !password}>
            Войти
          </Button>
        </div>
      </form>
    </div>
  )
}
