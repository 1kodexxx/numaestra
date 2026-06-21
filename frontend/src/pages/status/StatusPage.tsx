import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { usePollOrderStatus } from '@features/poll-order-status'
import { orderStorage } from '@shared/lib/storage'
import { MusicPlayer } from '@widgets/player'
import type { OrderDetail } from '@entities/order'

const CYAN = '#00e5c0'
const DARK = '#0d0d0d'

const inputStyle = {
  background: '#1c1c1c', border: '1px solid #333',
  borderRadius: '12px', padding: '12px 16px',
  color: '#fff', fontSize: '14px', outline: 'none',
  width: '100%',
} as const

export function StatusPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()

  const urlOrderId = searchParams.get('order_id')
  const urlToken = searchParams.get('token')

  useEffect(() => {
    if (urlOrderId && urlToken) orderStorage.saveOrder(urlOrderId, urlToken)
  }, [urlOrderId, urlToken])

  const initId = urlOrderId ?? orderStorage.getOrderId()
  const [lookupId, setLookupId] = useState(initId ?? '')
  const [activeId, setActiveId] = useState<string | null>(initId)

  const { order, loading, error } = usePollOrderStatus(activeId)

  function handleLookup() {
    const id = lookupId.trim()
    if (id) setActiveId(id)
  }

  function handleClear() {
    orderStorage.clear()
    setActiveId(null)
    setLookupId('')
  }

  if (loading && !order) {
    return (
      <div className="flex items-center justify-center" style={{ height: 'calc(100vh - 56px)' }}>
        <div
          className="w-10 h-10 rounded-full border-2 border-t-transparent spin-anim"
          style={{ borderColor: `${CYAN} transparent transparent transparent` }}
        />
      </div>
    )
  }

  if (!activeId || error) {
    return (
      <div className="max-w-lg mx-auto px-6 pt-16 pb-20">
        <div
          className="rounded-2xl p-8 text-center"
          style={{ background: '#141414', border: '1px solid #252525' }}
        >
          <div className="text-5xl mb-4">🔍</div>
          <div className="text-xl font-bold mb-2">Проверить статус заказа</div>
          <div className="text-sm mb-6" style={{ color: '#777' }}>Введите ID вашего заказа</div>

          {error && (
            <div className="text-sm mb-4 p-3 rounded-lg" style={{ background: '#2a1010', color: '#ef4444', border: '1px solid #4a2020' }}>
              {error}
            </div>
          )}

          <div className="flex gap-2.5">
            <input
              style={inputStyle}
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              value={lookupId}
              onChange={(e) => setLookupId(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleLookup()}
              onFocus={(e) => { e.target.style.borderColor = CYAN }}
              onBlur={(e) => { e.target.style.borderColor = '#333' }}
            />
            <button
              onClick={handleLookup}
              className="px-5 rounded-xl text-sm font-semibold shrink-0"
              style={{ background: CYAN, color: DARK, border: 'none', cursor: 'pointer' }}
            >
              Найти
            </button>
          </div>
        </div>
      </div>
    )
  }

  if (!order) return null

  return (
    <div className="max-w-2xl mx-auto px-6 pt-10 pb-20">
      <OrderStatusCard order={order} onClear={handleClear} onBack={() => navigate('/')} />
    </div>
  )
}

function OrderStatusCard({
  order, onClear, onBack,
}: { order: OrderDetail; onClear: () => void; onBack: () => void }) {
  const gs = order.generation_status
  const ps = order.payment_status

  let statusIcon = '⏳'
  let statusTitle = 'Обрабатываем заказ'
  let statusSub = 'Это займёт несколько минут...'

  if (ps === 'pending')        { statusIcon = '💳'; statusTitle = 'Ожидается оплата';    statusSub = 'После оплаты начнём создавать песню' }
  else if (gs === 'completed') { statusIcon = '🎉'; statusTitle = 'Ваша песня готова!';  statusSub = 'Все 4 версии доступны для прослушивания' }
  else if (gs === 'failed')    { statusIcon = '😔'; statusTitle = 'Произошла ошибка';   statusSub = 'Обратитесь в поддержку с ID заказа' }
  else if (gs === 'processing'){ statusIcon = '🎹'; statusTitle = 'Создаём вашу песню'; statusSub = 'AI-студия работает над вашим треком...' }
  else if (gs === 'queued')    { statusIcon = '🎶'; statusTitle = 'Заказ в очереди';    statusSub = 'Скоро начнём генерацию' }

  const steps = ['Оплата', 'Очередь', 'Создание', 'Готово']
  const stepKeys = ['paid', 'queued', 'processing', 'completed'] as const
  const currentIdx = gs === 'failed' ? 2 : stepKeys.findIndex((k) => k === gs || (gs === 'new' && k === 'paid'))
  const isTerminal = gs === 'completed' || gs === 'failed'

  return (
    <div>
      {/* Status header */}
      <div
        className="rounded-2xl p-8 text-center mb-6"
        style={{ background: '#141414', border: '1px solid #252525' }}
      >
        <div className="text-5xl mb-4">{statusIcon}</div>
        <div className="text-2xl font-bold mb-2">{statusTitle}</div>
        <div className="text-sm mb-8" style={{ color: '#777' }}>{statusSub}</div>

        {/* Progress steps */}
        <div className="flex justify-center items-center">
          {steps.map((label, i) => (
            <div key={label} className="flex items-center">
              <div className="flex flex-col items-center">
                <div
                  className={`flex items-center justify-center rounded-full text-xs font-bold${i === currentIdx && !isTerminal ? ' progress-pulse' : ''}`}
                  style={{
                    width: '32px', height: '32px',
                    background: i <= currentIdx ? CYAN : '#252525',
                    color: i <= currentIdx ? DARK : '#555',
                  }}
                >
                  {i < currentIdx ? '✓' : i + 1}
                </div>
                <div className="text-xs mt-1.5" style={{ color: i <= currentIdx ? '#aaa' : '#444', width: '56px', textAlign: 'center' }}>
                  {label}
                </div>
              </div>
              {i < steps.length - 1 && (
                <div
                  className="mb-5"
                  style={{
                    width: '48px', height: '2px',
                    background: i < currentIdx ? CYAN : '#252525',
                  }}
                />
              )}
            </div>
          ))}
        </div>

        <p className="mt-6 text-xs" style={{ color: '#444' }}>
          ID: <code style={{ color: CYAN, fontFamily: 'monospace' }}>{order.id}</code>
        </p>
        {!isTerminal && (
          <p className="text-xs mt-1" style={{ color: '#444' }}>Обновляется автоматически каждые 10 секунд</p>
        )}
      </div>

      {/* Music Player */}
      {gs === 'completed' && order.tracks.length > 0 && (
        <div className="mb-6">
          <div className="text-sm font-semibold mb-3" style={{ color: '#aaa' }}>Ваши треки</div>
          <MusicPlayer tracks={order.tracks} />
        </div>
      )}

      {/* Actions */}
      {isTerminal && (
        <div className="flex gap-3">
          <button
            onClick={onClear}
            className="flex-1 py-3 rounded-xl text-sm font-semibold"
            style={{ background: 'transparent', border: `1px solid ${CYAN}`, color: CYAN, cursor: 'pointer' }}
          >
            Новый заказ
          </button>
          <button
            onClick={onBack}
            className="flex-1 py-3 rounded-xl text-sm font-semibold"
            style={{ background: CYAN, color: DARK, border: 'none', cursor: 'pointer' }}
          >
            На главную
          </button>
        </div>
      )}
    </div>
  )
}
