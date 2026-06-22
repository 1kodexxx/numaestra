import { useEffect, useState } from 'react'
import { adminAccountApi } from '@entities/admin-account'
import type { AccountStatus, AdminAccount } from '@entities/admin-account'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, PageHeader, Panel, ErrorBanner, EmptyState, StatusBadge, Field, Select } from '@widgets/admin-layout'

const STATUS: Record<AccountStatus, { label: string; tone: 'green' | 'amber' | 'red' | 'cyan' | 'muted' }> = {
  active: { label: 'Активен', tone: 'green' },
  busy: { label: 'Занят', tone: 'cyan' },
  cooldown: { label: 'Остывает', tone: 'amber' },
  out_of_tokens: { label: 'Нет токенов', tone: 'amber' },
  banned: { label: 'Заблокирован', tone: 'red' },
}
const STATUS_OPTIONS: AccountStatus[] = ['active', 'cooldown', 'banned', 'out_of_tokens']

export function AdminAccountsPage() {
  const [accounts, setAccounts] = useState<AdminAccount[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [email, setEmail] = useState('')
  const [session, setSession] = useState('')
  const [maxConcurrent, setMaxConcurrent] = useState('1')
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function load() {
    setLoading(true)
    adminAccountApi
      .list()
      .then((data) => { setAccounts(data); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null); setSubmitting(true)
    try {
      await adminAccountApi.add({ email, session, max_concurrent: Number(maxConcurrent) || 1 })
      setEmail(''); setSession(''); setMaxConcurrent('1'); setShowForm(false)
      load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось добавить аккаунт')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleStatusChange(accountId: string, status: AccountStatus) {
    try {
      await adminAccountApi.setStatus(accountId, status)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось изменить статус')
    }
  }

  return (
    <div style={{ maxWidth: 880 }}>
      <PageHeader
        title="Suno-аккаунты"
        subtitle="Пул аккаунтов, к которым подключается воркер генерации"
        action={<Button variant={showForm ? 'outlined' : 'filled'} onClick={() => setShowForm(v => !v)}>{showForm ? 'Отмена' : '+ Аккаунт'}</Button>}
      />

      {showForm && (
        <Panel style={{ padding: '24px', marginBottom: '24px', maxWidth: 560 }}>
          <form onSubmit={handleAdd} style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
            <TextField label="Email аккаунта Suno" type="email" value={email} onChange={setEmail} required surfaceColor={A.surface} />
            <TextField
              label="Сессия (cookie/токен Suno)"
              value={session} onChange={setSession}
              multiline rows={3} required
              supportingText="Будет зашифрована при сохранении."
              surfaceColor={A.surface}
            />
            <div style={{ maxWidth: 200 }}>
              <Field label="Лимит параллельных задач">
                <input
                  type="number" min={1} value={maxConcurrent}
                  onChange={(e) => setMaxConcurrent(e.target.value)}
                  style={{
                    background: A.surface2, border: `1px solid ${A.border}`, borderRadius: '12px',
                    padding: '12px 14px', color: A.txt, fontSize: '14px', fontFamily: 'inherit', outline: 'none',
                  }}
                />
              </Field>
            </div>
            {formError && <ErrorBanner>{formError}</ErrorBanner>}
            <Button type="submit" loading={submitting} style={{ alignSelf: 'flex-start' }}>Добавить аккаунт</Button>
          </form>
        </Panel>
      )}

      {loading && <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>}
      {error && <ErrorBanner>{error}</ErrorBanner>}

      {!loading && accounts.length === 0 && <Panel><EmptyState icon="🎚️" text="Аккаунтов пока нет — добавьте первый." /></Panel>}

      {!loading && accounts.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {accounts.map((a) => {
            const s = STATUS[a.status]
            const slotsFull = a.concurrent_tasks >= a.max_concurrent_tasks
            return (
              <Panel key={a.id} style={{ padding: '16px 20px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px' }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontSize: '15px', fontWeight: 700, color: A.txt, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{a.email}</div>
                  <div style={{ fontSize: '13px', color: A.txt2, marginTop: '5px', display: 'flex', gap: '14px' }}>
                    <span>🪙 Токены: <b style={{ color: A.txt }}>{a.token_balance}</b></span>
                    <span style={{ color: slotsFull ? '#fbbf24' : A.txt2 }}>
                      ⚙️ Слоты: <b style={{ color: slotsFull ? '#fbbf24' : A.txt }}>{a.concurrent_tasks}/{a.max_concurrent_tasks}</b>
                    </span>
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexShrink: 0 }}>
                  <StatusBadge label={s.label} tone={s.tone} dot />
                  <Select value="" onChange={(v) => v && handleStatusChange(a.id, v as AccountStatus)} style={{ padding: '8px 12px', fontSize: '13px' }}>
                    <option value="">Сменить статус…</option>
                    {STATUS_OPTIONS.filter((o) => o !== a.status).map((o) => (
                      <option key={o} value={o}>{STATUS[o].label}</option>
                    ))}
                  </Select>
                </div>
              </Panel>
            )
          })}
        </div>
      )}
    </div>
  )
}
