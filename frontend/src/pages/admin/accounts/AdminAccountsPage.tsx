import { useEffect, useState } from 'react'
import { adminAccountApi } from '@entities/admin-account'
import type { AccountStatus, AdminAccount } from '@entities/admin-account'
import { Spinner } from '@shared/ui'
import { ApiError } from '@shared/api'

const inputCls = [
  'w-full bg-bg3 border border-border rounded-lg px-3 py-2',
  'text-txt text-sm outline-none transition-colors',
  'focus:border-accent2 placeholder:text-muted',
].join(' ')

const STATUS_LABEL: Record<AccountStatus, string> = {
  active: 'Активен',
  busy: 'Занят',
  cooldown: 'Остывает',
  out_of_tokens: 'Нет токенов',
  banned: 'Заблокирован',
}

const STATUS_OPTIONS: AccountStatus[] = ['active', 'cooldown', 'banned', 'out_of_tokens']

export function AdminAccountsPage() {
  const [accounts, setAccounts] = useState<AdminAccount[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [email, setEmail] = useState('')
  const [session, setSession] = useState('')
  const [maxConcurrent, setMaxConcurrent] = useState(1)
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
    setFormError(null)
    setSubmitting(true)
    try {
      await adminAccountApi.add({ email, session, max_concurrent: maxConcurrent })
      setEmail(''); setSession(''); setMaxConcurrent(1)
      setShowForm(false)
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
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Пул Suno-аккаунтов</h1>
        <button
          className="px-4 py-2 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] transition-all"
          onClick={() => setShowForm((v) => !v)}
        >
          {showForm ? 'Отмена' : '+ Аккаунт'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleAdd} className="bg-bg2 border border-border rounded-xl p-6 mb-6 flex flex-col gap-3 max-w-xl">
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted">Email аккаунта Suno</label>
            <input className={inputCls} type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted">Сессия (cookie/токен Suno — будет зашифрован при сохранении)</label>
            <textarea className={inputCls} rows={3} value={session} onChange={(e) => setSession(e.target.value)} required />
          </div>
          <div className="flex flex-col gap-1 max-w-40">
            <label className="text-xs text-muted">Лимит параллельных задач</label>
            <input type="number" min={1} className={inputCls} value={maxConcurrent} onChange={(e) => setMaxConcurrent(Number(e.target.value))} />
          </div>
          {formError && <div className="bg-error/10 border border-error/30 rounded-lg px-3 py-2 text-error text-sm">{formError}</div>}
          <button
            type="submit"
            disabled={submitting}
            className="self-start px-4 py-2 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] disabled:opacity-60 transition-all"
          >
            {submitting ? 'Добавляем...' : 'Добавить'}
          </button>
        </form>
      )}

      {loading && <div className="py-10"><Spinner /></div>}
      {error && <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm mb-4">{error}</div>}

      {!loading && (
        <div className="flex flex-col gap-2">
          {accounts.map((a) => (
            <div key={a.id} className="bg-bg2 border border-border rounded-lg px-5 py-4 flex items-center justify-between gap-4">
              <div>
                <div className="font-semibold">{a.email}</div>
                <div className="text-sm text-muted mt-0.5">
                  Токенов: {a.token_balance} · Занято слотов: {a.concurrent_tasks}/{a.max_concurrent_tasks}
                </div>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                <span className="bg-bg3 border border-border text-muted px-2.5 py-1 rounded-full text-xs">
                  {STATUS_LABEL[a.status]}
                </span>
                <select
                  className="bg-bg3 border border-border rounded-lg px-2 py-1.5 text-xs text-txt outline-none cursor-pointer"
                  value=""
                  onChange={(e) => { if (e.target.value) handleStatusChange(a.id, e.target.value as AccountStatus) }}
                >
                  <option value="">Сменить статус…</option>
                  {STATUS_OPTIONS.filter((s) => s !== a.status).map((s) => (
                    <option key={s} value={s}>{STATUS_LABEL[s]}</option>
                  ))}
                </select>
              </div>
            </div>
          ))}
          {accounts.length === 0 && <div className="text-muted text-sm">Аккаунтов пока нет.</div>}
        </div>
      )}
    </div>
  )
}
