import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { markPaidPending } from '@shared/lib/paidPending'
import { Button } from '@shared/ui'
import { theme } from '@shared/lib/theme'
import { GOALS, reachGoal } from '@shared/lib/analytics'
import { resolveOrderId } from './resolveOrderId'

const ACCENT = theme.accent
const TEXT2 = theme.text2
const BORDER = theme.border

const REDIRECT_SEC = 4

/** Robokassa SuccessURL → короткий экран успеха, затем статус заказа. */
export function OrderSuccessPage() {
  const navigate = useNavigate()
  const [seconds, setSeconds] = useState(REDIRECT_SEC)
  const [orderId, setOrderId] = useState<string | null>(null)
  // resolving — пока не знаем, какой заказ открывать (идёт резолв по InvId).
  const [resolving, setResolving] = useState(true)
  const [manualId, setManualId] = useState('')

  const params = new URLSearchParams(window.location.search)
  const invId = params.get('InvId')

  useEffect(() => {
    let cancelled = false
    resolveOrderId(invId).then(id => {
      if (cancelled) return
      setOrderId(id)
      setResolving(false)
      reachGoal(GOALS.PAYMENT_SUCCESS, { order_id: id ?? undefined })
      if (id) markPaidPending(id)
    })
    return () => { cancelled = true }
  }, [invId])

  // Обратный отсчёт стартует ТОЛЬКО после успешного резолва — иначе таймер мог бы
  // сработать раньше async-резолва и увести на /status без ID (или на чужой заказ).
  useEffect(() => {
    if (resolving || !orderId) return
    const t = setInterval(() => setSeconds((s) => s - 1), 1000)
    return () => clearInterval(t)
  }, [resolving, orderId])

  useEffect(() => {
    if (resolving || !orderId || seconds > 0) return
    navigate(`/status/${orderId}?paid=1`, { replace: true })
  }, [seconds, resolving, orderId, navigate])

  const panelStyle = { background: theme.surface, border: `1px solid ${BORDER}`, borderRadius: '24px', padding: '40px 36px', textAlign: 'center' as const }

  // Резолв завершён, но заказ не определён (InvId не сопоставился, либо его не было
  // и нет сохранённого заказа). Платёж прошёл — даём открыть заказ по ID вручную
  // (он есть в письме/на странице статуса) и связаться с поддержкой. Без слепого редиректа.
  if (!resolving && !orderId) {
    const trimmed = manualId.trim()
    return (
      <div className="fade-in" style={{ maxWidth: 480, margin: '0 auto', padding: '64px 24px' }}>
        <div className="modal-panel interactive-card" style={panelStyle}>
          <div style={{ fontSize: '48px', marginBottom: '16px' }}>✓</div>
          <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '8px' }}>Оплата прошла!</div>
          <div style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.6, marginBottom: '20px' }}>
            Платёж подтверждён, но не удалось автоматически открыть заказ в этом браузере.
            Введите номер заказа (он в письме или на странице статуса) или напишите в поддержку.
          </div>
          <input
            value={manualId}
            onChange={(e) => setManualId(e.target.value)}
            placeholder="ID заказа"
            style={{ width: '100%', boxSizing: 'border-box', padding: '12px 14px', borderRadius: '12px', border: `1px solid ${BORDER}`, background: 'rgba(255,255,255,0.03)', color: '#fff', fontSize: '14px', marginBottom: '12px' }}
          />
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            <Button size="lg" fullWidth disabled={!trimmed} onClick={() => navigate(`/status/${trimmed}?paid=1`)}>
              Открыть заказ →
            </Button>
            <Button size="lg" variant="outlined" fullWidth onClick={() => navigate('/legal/contacts')}>
              Написать в поддержку
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="fade-in" style={{ maxWidth: 480, margin: '0 auto', padding: '64px 24px' }}>
      <div className="modal-panel interactive-card" style={panelStyle}>
        <div style={{ fontSize: '48px', marginBottom: '16px' }}>✓</div>
        <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '8px' }}>Оплата прошла!</div>
        <div style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.6, marginBottom: '20px' }}>
          {resolving
            ? 'Подтверждаем платёж и находим ваш заказ…'
            : `Мы подтверждаем платёж и запускаем создание песни. Перенаправим на страницу статуса через ${seconds} с…`}
        </div>
        <div className="spin-anim" style={{ width: 28, height: 28, borderRadius: '50%', border: '2px solid rgba(255,255,255,0.07)', borderTopColor: ACCENT, margin: '0 auto 24px' }} />
        <Button
          size="lg"
          fullWidth
          disabled={resolving || !orderId}
          onClick={() => orderId && navigate(`/status/${orderId}?paid=1`, { replace: true })}
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
