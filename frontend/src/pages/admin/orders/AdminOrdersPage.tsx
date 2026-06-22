import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { adminOrderApi } from '@entities/admin-order'
import type { AdminOrder } from '@entities/admin-order'
import { Spinner, Button } from '@shared/ui'
import { A, PageHeader, Panel, ErrorBanner, EmptyState, paymentBadge, generationBadge } from '@widgets/admin-layout'

const PER_PAGE = 20

export function AdminOrdersPage() {
  const navigate = useNavigate()
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
    <div style={{ maxWidth: 980 }}>
      <PageHeader title="Заказы" subtitle={total > 0 ? `Всего заказов: ${total}` : 'Список заказов клиентов'} />

      {loading && <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>}
      {error && <ErrorBanner>{error}</ErrorBanner>}

      {!loading && orders.length === 0 && <Panel><EmptyState icon="🧾" text="Заказов пока нет." /></Panel>}

      {!loading && orders.length > 0 && (
        <>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            {orders.map((o) => (
              <OrderRow key={o.id} order={o} onClick={() => navigate(`/admin/orders/${o.id}`)} />
            ))}
          </div>

          {totalPages > 1 && (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '14px', marginTop: '24px' }}>
              <Button variant="outlined" size="sm" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Назад</Button>
              <span style={{ fontSize: '13px', color: A.txt2 }}>{page} / {totalPages}</span>
              <Button variant="outlined" size="sm" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Далее →</Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function OrderRow({ order, onClick }: { order: AdminOrder; onClick: () => void }) {
  const [h, setH] = useState(false)
  return (
    <div
      onClick={onClick}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      style={{
        background: A.surface,
        border: `1px solid ${h ? 'rgba(0,229,192,0.35)' : A.border}`,
        borderRadius: '16px', boxShadow: 'var(--elevation-1)',
        padding: '16px 20px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px',
        cursor: 'pointer',
        transform: h ? 'translateY(-1px)' : 'translateY(0)',
        transition: 'all 0.15s',
      }}
    >
      <div style={{ minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: '10px' }}>
          <span style={{ fontSize: '15px', fontWeight: 700, color: A.txt }}>#{order.invoice_id}</span>
          <span style={{ fontSize: '13px', color: A.txt2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {order.email || order.phone || '—'}
          </span>
        </div>
        <div style={{ fontSize: '13px', color: A.txt3, marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '420px' }}>
          {order.brief}
        </div>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexShrink: 0 }}>
        <span style={{ fontSize: '14px', fontWeight: 700, color: A.txt }}>{(order.amount_kopecks / 100).toFixed(0)} ₽</span>
        {paymentBadge(order.payment_status)}
        {generationBadge(order.generation_status)}
      </div>
    </div>
  )
}
