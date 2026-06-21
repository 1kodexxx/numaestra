import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { adminOrderApi } from '@entities/admin-order'
import type { AdminOrder } from '@entities/admin-order'
import { Spinner } from '@shared/ui'
import { ApiError } from '@shared/api'

const inputCls = [
  'w-full bg-bg3 border border-border rounded-lg px-3 py-2',
  'text-txt text-sm outline-none transition-colors',
  'focus:border-accent2 placeholder:text-muted resize-y',
].join(' ')

export function AdminOrderDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const [order, setOrder] = useState<AdminOrder | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const [sendOk, setSendOk] = useState(false)

  const [refunding, setRefunding] = useState(false)
  const [refundError, setRefundError] = useState<string | null>(null)

  function load() {
    setLoading(true)
    adminOrderApi
      .get(id)
      .then((o) => { setOrder(o); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [id])

  async function handleSendFeedback(e: React.FormEvent) {
    e.preventDefault()
    setSendError(null)
    setSendOk(false)
    setSending(true)
    try {
      await adminOrderApi.sendFeedback(id, message)
      setMessage('')
      setSendOk(true)
      load()
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : 'Не удалось отправить сообщение')
    } finally {
      setSending(false)
    }
  }

  async function handleRefund() {
    if (!confirm('Инициировать возврат платежа клиенту через Robokassa?')) return
    setRefundError(null)
    setRefunding(true)
    try {
      await adminOrderApi.refund(id)
      load()
    } catch (err) {
      setRefundError(err instanceof ApiError ? err.message : 'Не удалось выполнить возврат')
    } finally {
      setRefunding(false)
    }
  }

  if (loading) return <div className="py-10"><Spinner /></div>
  if (error) return <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm">{error}</div>
  if (!order) return null

  return (
    <div className="max-w-3xl">
      <Link to="/admin/orders" className="text-muted no-underline text-sm hover:text-txt transition-colors">← К списку заказов</Link>
      <h1 className="text-2xl font-bold mt-2 mb-6">Заказ #{order.invoice_id}</h1>

      <div className="bg-bg2 border border-border rounded-xl p-6 mb-6 grid grid-cols-2 gap-4 text-sm">
        <div><div className="text-muted">Email</div><div>{order.email || '—'}</div></div>
        <div><div className="text-muted">Телефон</div><div>{order.phone || '—'}</div></div>
        <div><div className="text-muted">Сумма</div><div>{(order.amount_kopecks / 100).toFixed(0)} ₽</div></div>
        <div><div className="text-muted">Статус оплаты</div><div>{order.payment_status}</div></div>
        <div><div className="text-muted">Статус генерации</div><div>{order.generation_status}</div></div>
        <div><div className="text-muted">Создан</div><div>{new Date(order.created_at).toLocaleString('ru-RU')}</div></div>
        <div className="col-span-2"><div className="text-muted">Бриф</div><div>{order.brief}</div></div>
      </div>

      {order.tracks.length > 0 && (
        <div className="bg-bg2 border border-border rounded-xl p-6 mb-6">
          <div className="text-sm font-semibold mb-3">Треки</div>
          <div className="flex flex-col gap-2">
            {order.tracks.map((t) => (
              <a key={t.index} href={t.audio_url} target="_blank" rel="noreferrer" className="text-accent text-sm no-underline hover:underline">
                🎧 Вариант {t.index}
              </a>
            ))}
          </div>
        </div>
      )}

      {order.payment_status === 'paid' && (
        <div className="mb-6">
          <button
            className="px-4 py-2 rounded-lg text-sm font-medium bg-transparent border border-error text-error cursor-pointer hover:bg-error/10 disabled:opacity-50 transition-colors"
            disabled={refunding}
            onClick={handleRefund}
          >
            {refunding ? 'Возвращаем...' : 'Вернуть оплату'}
          </button>
          {refundError && <div className="bg-error/10 border border-error/30 rounded-lg px-3 py-2 text-error text-sm mt-2">{refundError}</div>}
        </div>
      )}

      <div className="bg-bg2 border border-border rounded-xl p-6">
        <div className="text-sm font-semibold mb-3">Обратная связь клиенту</div>

        {order.admin_feedback && (
          <div className="bg-bg3 border border-border rounded-lg px-4 py-3 text-sm mb-4">
            <div className="text-muted text-xs mb-1">
              Последнее сообщение{order.admin_feedback_at ? ` · ${new Date(order.admin_feedback_at).toLocaleString('ru-RU')}` : ''}
            </div>
            {order.admin_feedback}
          </div>
        )}

        <form onSubmit={handleSendFeedback} className="flex flex-col gap-3">
          <textarea
            className={inputCls}
            rows={3}
            placeholder="Например: уточните, пожалуйста, дату праздника"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            required
          />
          {sendError && <div className="bg-error/10 border border-error/30 rounded-lg px-3 py-2 text-error text-sm">{sendError}</div>}
          {sendOk && <div className="bg-success/10 border border-success/30 rounded-lg px-3 py-2 text-success text-sm">Письмо отправлено клиенту.</div>}
          <button
            type="submit"
            disabled={sending || !order.email}
            className="self-start px-4 py-2 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] disabled:opacity-60 transition-all"
          >
            {sending ? 'Отправляем...' : 'Отправить'}
          </button>
          {!order.email && <div className="text-xs text-muted">У заказа не указан email — отправка недоступна.</div>}
        </form>
      </div>
    </div>
  )
}
