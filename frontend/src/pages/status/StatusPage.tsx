import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { usePollOrderStatus } from '@features/poll-order-status'
import { orderStorage } from '@shared/lib/storage'
import { Spinner } from '@shared/ui'
import type { OrderDetail } from '@entities/order'

const inputCls = 'flex-1 bg-bg3 border border-border rounded-lg px-3.5 py-2.5 text-txt text-sm outline-none transition-colors focus:border-accent2 font-[inherit]'
const btnOutlineCls = 'px-5 py-2.5 rounded-lg text-sm font-semibold bg-transparent border border-accent2 text-accent cursor-pointer hover:bg-accent/10 transition-all'

export function StatusPage() {
  const savedId = orderStorage.getOrderId()
  const [lookupId, setLookupId] = useState(savedId ?? '')
  const [activeId, setActiveId] = useState<string | null>(savedId)
  const navigate = useNavigate()

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

  if (loading) return <div className="text-center py-20"><Spinner /></div>

  if (!activeId || error) {
    return (
      <div className="max-w-2xl mx-auto px-6 pt-10 pb-20">
        <div className="bg-bg2 border border-border rounded-xl p-8 text-center">
          <div className="text-[56px] mb-4">🔍</div>
          <div className="text-[22px] font-bold mb-2">Проверить статус заказа</div>
          <div className="text-muted mb-6">Введите ID вашего заказа</div>
          {error && (
            <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm my-3">
              {error}
            </div>
          )}
          <div className="flex gap-2.5 max-w-120 mx-auto mt-6">
            <input
              className={inputCls}
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              value={lookupId}
              onChange={(e) => setLookupId(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleLookup()}
            />
            <button
              className="px-5 py-2.5 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] transition-all"
              onClick={handleLookup}
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

function OrderStatusCard({ order, onClear, onBack }: {
  order: OrderDetail; onClear: () => void; onBack: () => void
}) {
  const gs = order.generation_status
  const ps = order.payment_status

  let icon = '⏳', title = 'Обрабатываем заказ', sub = 'Это займёт несколько минут...'
  if (ps === 'pending')         { icon = '💳'; title = 'Ожидается оплата';    sub = 'После оплаты начнём создавать песню' }
  else if (gs === 'completed')  { icon = '🎉'; title = 'Ваша песня готова!';  sub = 'Слушайте все варианты ниже' }
  else if (gs === 'failed')     { icon = '😔'; title = 'Произошла ошибка';   sub = 'Обратитесь в поддержку с ID заказа' }
  else if (gs === 'processing') { icon = '🎹'; title = 'Создаём вашу песню'; sub = 'Работает наша AI-студия...' }
  else if (gs === 'queued')     { icon = '🎶'; title = 'Заказ в очереди';    sub = 'Скоро начнём генерацию' }

  const steps = ['Оплата', 'В очереди', 'Создание', 'Готово']
  const stepKeys = ['paid', 'queued', 'processing', 'completed'] as const
  const currentIdx = gs === 'failed' ? 2 : stepKeys.findIndex((k) => k === gs || (gs === 'new' && k === 'paid'))
  const isTerminal = gs === 'completed' || gs === 'failed'

  return (
    <div className="bg-bg2 border border-border rounded-xl p-8 text-center">
      <div className="text-[56px] mb-4">{icon}</div>
      <div className="text-[22px] font-bold mb-2">{title}</div>
      <div className="text-muted mb-6">{sub}</div>

      <div className="flex justify-center my-6">
        {steps.map((label, i) => (
          <div key={label} className="flex items-center">
            <div className="flex flex-col items-center">
              <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold transition-colors ${
                i < currentIdx
                  ? 'bg-accent2'
                  : i === currentIdx
                    ? 'bg-accent progress-pulse'
                    : 'bg-border'
              }`}>
                {i < currentIdx ? '✓' : i + 1}
              </div>
              <div className="text-[11px] text-muted text-center mt-1 w-14">{label}</div>
            </div>
            {i < steps.length - 1 && (
              <div className={`w-10 h-0.5 mb-5 transition-colors ${i < currentIdx ? 'bg-accent2' : 'bg-border'}`} />
            )}
          </div>
        ))}
      </div>

      {gs === 'completed' && order.tracks.length > 0 && (
        <div className="flex flex-col gap-3 mt-6 text-left">
          {order.tracks.map((t, i) => (
            <div key={t.index} className="bg-bg3 border border-border rounded-[10px] p-4">
              <div className="font-semibold mb-2">🎵 Вариант {t.index || i + 1}</div>
              <audio controls src={t.audio_url} className="w-full mt-1">
                Ваш браузер не поддерживает аудио
              </audio>
            </div>
          ))}
        </div>
      )}

      <p className="mt-6 text-muted text-[13px]">
        ID заказа: <code className="text-accent font-mono">{order.id}</code>
      </p>
      {!isTerminal && (
        <p className="text-muted text-xs mt-2">Страница обновляется автоматически каждые 10 секунд</p>
      )}
      {isTerminal && (
        <div className="flex gap-3 justify-center mt-4">
          <button className={btnOutlineCls} onClick={onClear}>Новый заказ</button>
          <button className={btnOutlineCls} onClick={onBack}>На главную</button>
        </div>
      )}
    </div>
  )
}
