import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { adminOrderApi } from '@entities/admin-order'
import type { AdminOrder } from '@entities/admin-order'
import { Spinner } from '@shared/ui'

const PER_PAGE = 20

const PAYMENT_LABEL: Record<AdminOrder['payment_status'], string> = {
  pending: 'Ожидает оплаты',
  paid: 'Оплачен',
  failed: 'Оплата не прошла',
  refunded: 'Возврат',
}

const GENERATION_LABEL: Record<AdminOrder['generation_status'], string> = {
  new: 'Новый',
  queued: 'В очереди',
  processing: 'Генерируется',
  completed: 'Готов',
  failed: 'Ошибка',
}

export function AdminOrdersPage() {
  const [orders, setOrders] = useState<AdminOrder[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setLoading(true)
    adminOrderApi
      .list(page, PER_PAGE)
      .then((data) => { setOrders(data.orders); setTotal(data.total); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }, [page])

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE))

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Заказы</h1>

      {loading && <div className="py-10"><Spinner /></div>}
      {error && <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm mb-4">{error}</div>}

      {!loading && (
        <>
          <div className="flex flex-col gap-2">
            {orders.map((o) => (
              <Link
                key={o.id}
                to={`/admin/orders/${o.id}`}
                className="bg-bg2 border border-border rounded-lg px-5 py-4 flex items-center justify-between gap-4 no-underline text-txt hover:border-accent2 transition-colors"
              >
                <div>
                  <div className="font-semibold">#{o.invoice_id} — {o.email || o.phone}</div>
                  <div className="text-sm text-muted mt-0.5 line-clamp-1">{o.brief}</div>
                </div>
                <div className="flex items-center gap-3 shrink-0 text-sm">
                  <span className="text-muted">{(o.amount_kopecks / 100).toFixed(0)} ₽</span>
                  <span className="bg-accent/15 border border-accent/30 text-accent px-2.5 py-1 rounded-full text-xs">
                    {PAYMENT_LABEL[o.payment_status]}
                  </span>
                  <span className="bg-bg3 border border-border text-muted px-2.5 py-1 rounded-full text-xs">
                    {GENERATION_LABEL[o.generation_status]}
                  </span>
                </div>
              </Link>
            ))}
            {orders.length === 0 && <div className="text-muted text-sm">Заказов пока нет.</div>}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center gap-3 mt-6">
              <button
                className="px-3 py-1.5 rounded-lg text-sm bg-transparent border border-border text-muted cursor-pointer hover:text-txt disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
                ← Назад
              </button>
              <span className="text-sm text-muted">{page} / {totalPages}</span>
              <button
                className="px-3 py-1.5 rounded-lg text-sm bg-transparent border border-border text-muted cursor-pointer hover:text-txt disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Далее →
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
