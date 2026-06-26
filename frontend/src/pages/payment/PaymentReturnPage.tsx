import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { markPaidPending } from '@shared/lib/paidPending'
import { orderStorage } from '@shared/lib/storage'
import { orderApi } from '@entities/order'
import { Button } from '@shared/ui'
import { theme } from '@shared/lib/theme'
import { GOALS, reachGoal } from '@shared/lib/analytics'

const ACCENT = theme.accent
const TEXT2 = theme.text2
const BORDER = theme.border

const REDIRECT_SEC = 4

/**
 * Резолвим orderId по InvId:
 * 1) localStorage invoice map (same browser)
 * 2) API /orders/by-invoice/:invoiceId (другой браузер / очищенный storage)
 * 3) primary localStorage order id (last-resort)
 */
async function resolveOrderId(invId: string | null): Promise<string | null> {
  if (invId) {
    const fromStorage = orderStorage.getOrderIdByInvoice(Number(invId))
    if (fromStorage) return fromStorage
    // Пробуем API — нет токена, ответ только UUID, без PII.
    try {
      const res = await orderApi.getOrderIdByInvoice(Number(invId))
      if (res?.id) {
        orderStorage.saveInvoiceOrder(Number(invId), res.id)
        return res.id
      }
    } catch {
      // Недоступен API — пробуем основной ключ.
    }
  }
  return orderStorage.getOrderId()
}

/** Robokassa SuccessURL → короткий экран успеха, затем статус заказа. */
export function OrderSuccessPage() {
  const navigate = useNavigate()
  const [seconds, setSeconds] = useState(REDIRECT_SEC)
  const [orderId, setOrderId] = useState<string | null>(null)

  const params = new URLSearchParams(window.location.search)
  const invId = params.get('InvId')

  useEffect(() => {
    resolveOrderId(invId).then(id => {
      setOrderId(id)
      reachGoal(GOALS.PAYMENT_SUCCESS, { order_id: id ?? undefined })
      if (id) markPaidPending(id)
    })
  }, [invId])

  useEffect(() => {
    const t = setInterval(() => setSeconds((s) => s - 1), 1000)
    return () => clearInterval(t)
  }, [])

  useEffect(() => {
    if (seconds > 0) return
    if (orderId) {
      navigate(`/status/${orderId}?paid=1`, { replace: true })
    } else {
      navigate('/status?paid=1', { replace: true })
    }
  }, [seconds, orderId, navigate])

  return (
    <div className="fade-in" style={{ maxWidth: 480, margin: '0 auto', padding: '64px 24px' }}>
      <div className="modal-panel interactive-card" style={{ background: theme.surface, border: `1px solid ${BORDER}`, borderRadius: '24px', padding: '40px 36px', textAlign: 'center' }}>
        <div style={{ fontSize: '48px', marginBottom: '16px' }}>✓</div>
        <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '8px' }}>Оплата прошла!</div>
        <div style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.6, marginBottom: '20px' }}>
          Мы подтверждаем платёж и запускаем создание песни. Перенаправим на страницу статуса через {seconds} с…
        </div>
        <div className="spin-anim" style={{ width: 28, height: 28, borderRadius: '50%', border: '2px solid rgba(255,255,255,0.07)', borderTopColor: ACCENT, margin: '0 auto 24px' }} />
        <Button
          size="lg"
          fullWidth
          onClick={() => (orderId ? navigate(`/status/${orderId}?paid=1`, { replace: true }) : navigate('/status?paid=1', { replace: true }))}
        >
          К статусу заказа →
        </Button>
      </div>
    </div>
  )
}

/** Robokassa FailURL — понятное сообщение и путь вернуться к заказу. */
export function OrderFailPage() {
  const navigate = useNavigate()
  const [orderId, setOrderId] = useState<string | null>(null)

  const params = new URLSearchParams(window.location.search)
  const invId = params.get('InvId')

  useEffect(() => {
    resolveOrderId(invId).then(id => {
      setOrderId(id)
      reachGoal(GOALS.PAYMENT_FAIL, { order_id: id ?? undefined })
    })
  }, [invId])

  return (
    <div className="fade-in" style={{ maxWidth: 480, margin: '0 auto', padding: '64px 24px' }}>
      <div className="modal-panel interactive-card" style={{ background: theme.surface, border: `1px solid ${BORDER}`, borderRadius: '24px', padding: '40px 36px', textAlign: 'center' }}>
        <div style={{ fontSize: '48px', marginBottom: '16px' }}>💳</div>
        <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '8px' }}>Оплата не завершена</div>
        <div style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.6, marginBottom: '28px' }}>
          Платёж был отменён или отклонён банком. Заказ сохранён — попробуйте оплатить снова на странице статуса.
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <Button
            size="lg"
            fullWidth
            onClick={() => (orderId ? navigate(`/status/${orderId}`) : navigate('/status'))}
          >
            Вернуться к заказу →
          </Button>
          <Button
            size="lg"
            variant="outlined"
            fullWidth
            onClick={() => navigate('/legal/contacts')}
          >
            Написать в поддержку
          </Button>
        </div>
      </div>
    </div>
  )
}
