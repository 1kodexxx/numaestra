import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { markPaidPending } from '@shared/lib/paidPending'
import { orderStorage } from '@shared/lib/storage'
import { Button } from '@shared/ui'

const ACCENT = '#00e5c0'
const TEXT2 = 'rgba(255,255,255,0.55)'
const BORDER = 'rgba(255,255,255,0.08)'

/** Robokassa SuccessURL → сохранённый заказ + баннер об успешной оплате. */
export function OrderSuccessPage() {
  const navigate = useNavigate()

  useEffect(() => {
    const id = orderStorage.getOrderId()
    if (id) {
      markPaidPending(id)
      navigate(`/status/${id}?paid=1`, { replace: true })
    } else {
      navigate('/status?paid=1', { replace: true })
    }
  }, [navigate])

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 'calc(100dvh - 60px)' }}>
      <div className="spin-anim" style={{ width: 36, height: 36, borderRadius: '50%', border: '2px solid rgba(255,255,255,0.07)', borderTopColor: ACCENT }} />
    </div>
  )
}

/** Robokassa FailURL — понятное сообщение и путь вернуться к заказу. */
export function OrderFailPage() {
  const navigate = useNavigate()
  const orderId = orderStorage.getOrderId()

  return (
    <div className="fade-in" style={{ maxWidth: 480, margin: '0 auto', padding: '64px 24px' }}>
      <div className="modal-panel interactive-card" style={{ background: '#0f0f0f', border: `1px solid ${BORDER}`, borderRadius: '24px', padding: '40px 36px', textAlign: 'center' }}>
        <div style={{ fontSize: '48px', marginBottom: '16px' }}>💳</div>
        <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '8px' }}>Оплата не завершена</div>
        <div style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.6, marginBottom: '28px' }}>
          Платёж был отменён или отклонён банком. Вы можете попробовать снова — заказ сохранён.
        </div>
        <Button
          size="lg"
          fullWidth
          onClick={() => (orderId ? navigate(`/status/${orderId}`) : navigate('/status'))}
        >
          Вернуться к заказу →
        </Button>
      </div>
    </div>
  )
}
