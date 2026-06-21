import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { usePollOrderStatus } from '@features/poll-order-status'
import { orderStorage } from '@shared/lib/storage'
import { Spinner } from '@shared/ui'
import type { OrderDetail } from '@entities/order'
import styles from './StatusPage.module.css'

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

  if (loading) return <div className={styles.center}><Spinner /></div>

  if (!activeId || error) {
    return (
      <div className={styles.container}>
        <div className={styles.card}>
          <div className={styles.icon}>🔍</div>
          <div className={styles.title}>Проверить статус заказа</div>
          <div className={styles.sub}>Введите ID вашего заказа</div>
          {error && <div className={styles.error}>{error}</div>}
          <div className={styles.lookupForm}>
            <input
              className={styles.input}
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              value={lookupId}
              onChange={(e) => setLookupId(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleLookup()}
            />
            <button className={styles.btnPrimary} onClick={handleLookup}>Найти</button>
          </div>
        </div>
      </div>
    )
  }

  if (!order) return null

  return (
    <div className={styles.container}>
      <OrderStatusCard order={order} onClear={handleClear} onBack={() => navigate('/')} />
    </div>
  )
}

function OrderStatusCard({ order, onClear, onBack }: { order: OrderDetail; onClear: () => void; onBack: () => void }) {
  const gs = order.generation_status
  const ps = order.payment_status

  let icon = '⏳', title = 'Обрабатываем заказ', sub = 'Это займёт несколько минут...'
  if (ps === 'pending')        { icon = '💳'; title = 'Ожидается оплата'; sub = 'После оплаты начнём создавать песню' }
  else if (gs === 'completed') { icon = '🎉'; title = 'Ваша песня готова!'; sub = 'Слушайте все варианты ниже' }
  else if (gs === 'failed')    { icon = '😔'; title = 'Произошла ошибка'; sub = 'Обратитесь в поддержку с ID заказа' }
  else if (gs === 'processing'){ icon = '🎹'; title = 'Создаём вашу песню'; sub = 'Работает наша AI-студия...' }
  else if (gs === 'queued')    { icon = '🎶'; title = 'Заказ в очереди'; sub = 'Скоро начнём генерацию' }

  const steps = ['Оплата', 'В очереди', 'Создание', 'Готово']
  const stepKeys = ['paid', 'queued', 'processing', 'completed'] as const
  const currentIdx = gs === 'failed' ? 2 : stepKeys.findIndex((k) => k === gs || (gs === 'new' && k === 'paid'))

  const isTerminal = gs === 'completed' || gs === 'failed'

  return (
    <div className={styles.card}>
      <div className={styles.icon}>{icon}</div>
      <div className={styles.title}>{title}</div>
      <div className={styles.sub}>{sub}</div>

      <div className={styles.progress}>
        {steps.map((label, i) => (
          <div key={label} className={styles.progStep}>
            <div className={styles.progStepInner}>
              <div className={`${styles.progDot} ${i < currentIdx ? styles.done : ''} ${i === currentIdx ? styles.current : ''}`}>
                {i < currentIdx ? '✓' : i + 1}
              </div>
              <div className={styles.progLabel}>{label}</div>
            </div>
            {i < steps.length - 1 && <div className={`${styles.progLine} ${i < currentIdx ? styles.done : ''}`} />}
          </div>
        ))}
      </div>

      {gs === 'completed' && order.tracks.length > 0 && (
        <div className={styles.tracks}>
          {order.tracks.map((t, i) => (
            <div key={t.index} className={styles.track}>
              <div className={styles.trackTitle}>🎵 Вариант {t.index || i + 1}</div>
              <audio controls src={t.audio_url} style={{ width: '100%', marginTop: 4 }}>
                Ваш браузер не поддерживает аудио
              </audio>
            </div>
          ))}
        </div>
      )}

      <p className={styles.orderId}>
        ID заказа: <code>{order.id}</code>
      </p>
      {!isTerminal && (
        <p className={styles.pollNote}>Страница обновляется автоматически каждые 10 секунд</p>
      )}
      {isTerminal && (
        <div className={styles.actions}>
          <button className={styles.btnOutline} onClick={onClear}>Новый заказ</button>
          <button className={styles.btnOutline} onClick={onBack}>На главную</button>
        </div>
      )}
    </div>
  )
}
