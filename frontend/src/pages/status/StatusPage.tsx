import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { usePollOrderStatus } from '@features/poll-order-status'
import { orderStorage } from '@shared/lib/storage'
import { MusicPlayer } from '@widgets/player'
import { Button, TextField } from '@shared/ui'
import type { OrderDetail } from '@entities/order'

const ACCENT = '#00e5c0'
const DARK   = '#080808'
const TEXT2  = 'rgba(255,255,255,0.5)'
const TEXT3  = 'rgba(255,255,255,0.25)'
const BORDER = 'rgba(255,255,255,0.07)'

export function StatusPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()

  const urlId    = searchParams.get('order_id')
  const urlToken = searchParams.get('token')

  useEffect(() => {
    if (urlId && urlToken) orderStorage.saveOrder(urlId, urlToken)
  }, [urlId, urlToken])

  const initId   = urlId ?? orderStorage.getOrderId()
  const [input,  setInput]  = useState(initId ?? '')
  const [active, setActive] = useState<string | null>(initId)

  const { order, loading, error } = usePollOrderStatus(active)

  if (loading && !order) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 'calc(100vh - 60px)' }}>
        <div className="spin-anim" style={{ width: 36, height: 36, borderRadius: '50%', border: '2px solid rgba(255,255,255,0.07)', borderTopColor: ACCENT }} />
      </div>
    )
  }

  if (!active || (error && !order)) {
    return (
      <div style={{ maxWidth: 480, margin: '0 auto', padding: '64px 24px' }}>
        <div style={{ background: '#0f0f0f', border: `1px solid ${BORDER}`, borderRadius: '24px', padding: '40px 36px', textAlign: 'center' }}>
          <div style={{ fontSize: '48px', marginBottom: '16px' }}>🔍</div>
          <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '8px' }}>Статус заказа</div>
          <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '28px' }}>Введите ID вашего заказа</div>

          {error && (
            <div style={{ background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)', borderRadius: '10px', padding: '10px 14px', fontSize: '13px', color: '#ef4444', marginBottom: '16px' }}>
              {error}
            </div>
          )}

          <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-start', textAlign: 'left' }}>
            <div style={{ flex: 1 }} onKeyDown={(e) => e.key === 'Enter' && setActive(input.trim())}>
              <TextField
                label="ID заказа"
                value={input}
                onChange={setInput}
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                surfaceColor="#0f0f0f"
              />
            </div>
            <Button size="lg" onClick={() => setActive(input.trim())}>Найти</Button>
          </div>
        </div>
      </div>
    )
  }

  if (!order) return null

  return (
    <div style={{ maxWidth: 680, margin: '0 auto', padding: '40px 24px 60px' }}>
      <OrderCard
        order={order}
        onClear={() => { orderStorage.clear(); setActive(null); setInput('') }}
        onBack={() => navigate('/')}
      />
    </div>
  )
}

/* ── status steps ── */
const STEPS   = ['Оплата', 'Очередь', 'Создание', 'Готово']
const STEPKEYS = ['paid', 'queued', 'processing', 'completed'] as const

function OrderCard({ order, onClear, onBack }: { order: OrderDetail; onClear: () => void; onBack: () => void }) {
  const gs = order.generation_status
  const ps = order.payment_status

  let icon  = '⏳'
  let title = 'Обрабатываем заказ'
  let sub   = 'Пожалуйста, подождите...'

  if (ps === 'pending')         { icon = '💳'; title = 'Ожидание оплаты';    sub = 'После оплаты начнём создавать песню' }
  else if (gs === 'completed')  { icon = '🎵'; title = 'Ваша песня готова!'; sub = 'Все версии доступны для прослушивания' }
  else if (gs === 'failed')     { icon = '💔'; title = 'Ошибка генерации';   sub = 'Свяжитесь с нами — мы поможем' }
  else if (gs === 'processing') { icon = '🎹'; title = 'Создаём песню';      sub = 'AI-студия работает над вашим треком...' }
  else if (gs === 'queued')     { icon = '⏱'; title = 'В очереди';          sub = 'Скоро начнём создание' }

  const idx       = gs === 'failed' ? 2 : STEPKEYS.findIndex(k => k === gs || (gs === 'new' && k === 'paid'))
  const isTerminal = gs === 'completed' || gs === 'failed'

  return (
    <div className="fade-in">
      {/* Header */}
      <div style={{
        background: '#0f0f0f', border: `1px solid ${BORDER}`,
        borderRadius: '24px', padding: '36px 36px 28px', textAlign: 'center', marginBottom: '16px',
        position: 'relative', overflow: 'hidden',
      }}>
        {gs !== 'failed' && (
          <div style={{
            position: 'absolute', inset: 0, pointerEvents: 'none',
            background: 'radial-gradient(ellipse 60% 50% at 50% 0%, rgba(0,229,192,0.08) 0%, transparent 70%)',
          }} />
        )}

        <div style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', marginBottom: '14px' }}>
          {!isTerminal && (
            <span aria-hidden className="progress-pulse" style={{
              position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)',
              width: 60, height: 60, borderRadius: '50%', background: 'rgba(0,229,192,0.1)',
            }} />
          )}
          <div style={{ fontSize: '52px', lineHeight: 1, position: 'relative' }}>{icon}</div>
        </div>
        <div style={{ fontSize: '24px', fontWeight: 800, letterSpacing: '-0.03em', marginBottom: '8px' }}>{title}</div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '32px' }}>{sub}</div>

        {/* Steps */}
        <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 0 }}>
          {STEPS.map((label, i) => (
            <div key={label} style={{ display: 'flex', alignItems: 'center' }}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                <div
                  className={i === idx && !isTerminal ? 'progress-pulse' : ''}
                  style={{
                    width: 32, height: 32, borderRadius: '50%',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: '12px', fontWeight: 700,
                    background: i <= idx ? ACCENT : 'rgba(255,255,255,0.07)',
                    color: i <= idx ? DARK : 'rgba(255,255,255,0.2)',
                    transition: 'all 0.3s',
                  }}
                >
                  {i < idx ? '✓' : i + 1}
                </div>
                <div style={{ fontSize: '11px', marginTop: '6px', color: i <= idx ? TEXT2 : TEXT3, width: 60, textAlign: 'center', fontWeight: 500 }}>
                  {label}
                </div>
              </div>
              {i < STEPS.length - 1 && (
                <div style={{
                  width: 52, height: 2, marginBottom: 22,
                  background: i < idx ? ACCENT : 'rgba(255,255,255,0.07)',
                  transition: 'background 0.3s',
                }} />
              )}
            </div>
          ))}
        </div>

        <div style={{ marginTop: '20px', fontSize: '12px', color: TEXT3 }}>
          ID: <code style={{ color: 'rgba(0,229,192,0.6)', fontFamily: 'monospace' }}>{order.id}</code>
        </div>
        {!isTerminal && (
          <div style={{ fontSize: '12px', color: TEXT3, marginTop: '4px' }}>
            Обновляется каждые 10 секунд
          </div>
        )}
      </div>

      {/* Player */}
      {gs === 'completed' && order.tracks.length > 0 && (
        <div style={{ marginBottom: '16px' }}>
          <MusicPlayer tracks={order.tracks} />
        </div>
      )}

      {/* Actions */}
      {isTerminal && (
        <div style={{ display: 'flex', gap: '10px' }}>
          <Button variant="outlined" size="lg" fullWidth onClick={onClear}>Новый заказ</Button>
          <Button size="lg" fullWidth onClick={onBack}>На главную</Button>
        </div>
      )}
    </div>
  )
}
