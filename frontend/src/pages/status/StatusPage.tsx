import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { usePollOrderStatus } from '@features/poll-order-status'
import { clearPaidPending, isPaidPending, markPaidPending } from '@shared/lib/paidPending'
import { orderStorage } from '@shared/lib/storage'
import { MusicPlayer } from '@widgets/player'
import { Button, TextField, copyText } from '@shared/ui'
import { orderApi } from '@entities/order'
import { useSeo } from '@shared/lib/seo'
import { downloadFile } from '@shared/lib/download'
import { ShareBar } from '@widgets/share-bar'
import { AccessRecoveryForm } from '@widgets/access-recovery'
import { GenerationProgress } from './GenerationProgress'
import { StatusOrderSkeleton } from './StatusOrderSkeleton'
import type { OrderDetail, OrderSummary } from '@entities/order'
import { theme } from '@shared/lib/theme'
import { GOALS, reachGoal } from '@shared/lib/analytics'

const ACCENT = theme.accent
const DARK   = theme.dark
const TEXT2  = theme.text2
const TEXT3  = theme.text3
const BORDER = theme.border

export function StatusPage() {
  const { orderId: pathOrderId } = useParams<{ orderId?: string }>()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()

  const queryOrderId = searchParams.get('order_id')
  const urlToken = searchParams.get('token')
  const orderIdFromUrl = queryOrderId ?? pathOrderId ?? null

  useEffect(() => {
    if (orderIdFromUrl && urlToken) orderStorage.saveOrder(orderIdFromUrl, urlToken)
  }, [orderIdFromUrl, urlToken])

  // Click-tracking в письмах (RuSender) иногда съедает order_id из query.
  // Если token есть, а id нет — не подставляем старый заказ из localStorage.
  const brokenEmailLink = Boolean(urlToken && !orderIdFromUrl)

  const initId = orderIdFromUrl ?? (brokenEmailLink ? null : orderStorage.getOrderId())
  const [input,  setInput]  = useState(initId ?? '')
  const [active, setActive] = useState<string | null>(initId)

  const justPaid = searchParams.get('paid') === '1'
  // Robokassa редиректит на SuccessURL с OutSum и InvId в query (если не /order/success).
  const returnedFromRobokassa = searchParams.has('OutSum') && searchParams.has('InvId')
  const confirmPayment = Boolean(active && (justPaid || returnedFromRobokassa || isPaidPending(active)))

  useEffect(() => {
    if (active && (justPaid || returnedFromRobokassa)) markPaidPending(active)
  }, [active, justPaid, returnedFromRobokassa])

  const { order, loading, error, canManage, pollingTooLong } = usePollOrderStatus(active, { confirmPayment })

  useEffect(() => {
    if (order?.payment_status !== 'pending') clearPaidPending(order?.id ?? '')
  }, [order?.id, order?.payment_status])

  const completedTracked = useRef<string | null>(null)
  useEffect(() => {
    if (order?.generation_status === 'completed' && order.id !== completedTracked.current) {
      completedTracked.current = order.id
      reachGoal(GOALS.ORDER_COMPLETED, { order_id: order.id })
    }
  }, [order?.id, order?.generation_status])

  useSeo({ title: 'Статус заказа', description: 'Отслеживайте создание вашей персональной песни.', noindex: true })

  if (loading && !order && active) {
    return (
      <div style={{ maxWidth: 680, margin: '0 auto', padding: 'clamp(24px, 6vw, 40px) clamp(16px, 4vw, 24px) 60px' }}>
        <StatusOrderSkeleton />
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

          {(error || brokenEmailLink) && (
            <div style={{ background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)', borderRadius: '10px', padding: '10px 14px', fontSize: '13px', color: '#ef4444', marginBottom: '16px' }}>
              {brokenEmailLink
                ? 'Ссылка из письма открылась некорректно. Скопируйте ID заказа из письма или вставьте полную ссылку ещё раз.'
                : error}
            </div>
          )}

          <div className="status-find-form">
            <div className="status-find-form__field" onKeyDown={(e) => e.key === 'Enter' && setActive(input.trim())}>
              <TextField
                label="ID заказа"
                value={input}
                onChange={setInput}
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                surfaceColor={theme.surface}
              />
            </div>
            <Button className="status-find-form__btn" size="lg" onClick={() => setActive(input.trim())}>Найти</Button>
          </div>

          {input.trim().length >= 8 && (
            <AccessRecoveryForm orderId={input.trim()} compact />
          )}
        </div>
      </div>
    )
  }

  if (!order) return null

  return (
    <div style={{ maxWidth: 680, margin: '0 auto', padding: 'clamp(24px, 6vw, 40px) clamp(16px, 4vw, 24px) 60px' }}>
      <OrderCard
        order={order}
        justPaid={justPaid || returnedFromRobokassa}
        confirmAwaitingPayment={confirmPayment}
        canManage={canManage}
        pollingTooLong={pollingTooLong}
        onClear={() => { orderStorage.clear(); setActive(null); setInput('') }}
        onBack={() => navigate('/')}
      />
      <OrdersOverview
        currentId={order.id}
        enabled={canManage}
        onSelect={(id) => {
          const token = orderStorage.getAccessToken()
          if (token) orderStorage.saveOrder(id, token)
          setActive(id)
          setInput(id)
          navigate(`/status/${id}`, { replace: true })
        }}
      />
    </div>
  )
}

/* ── обзор всех заказов владельца (по токену) ── */
function summaryBadge(o: OrderSummary): { label: string; color: string; bg: string } {
  if (o.payment_status === 'pending') return { label: 'Ожидает оплаты', color: '#fbbf24', bg: 'rgba(245,158,11,0.12)' }
  switch (o.generation_status) {
    case 'completed': return { label: 'Готов', color: '#4ade80', bg: 'rgba(34,197,94,0.12)' }
    case 'failed':    return { label: 'Ошибка', color: '#f87171', bg: 'rgba(239,68,68,0.12)' }
    case 'processing':return { label: 'Создаётся', color: ACCENT, bg: 'rgba(0,229,192,0.12)' }
    case 'queued':    return { label: 'В очереди', color: ACCENT, bg: 'rgba(0,229,192,0.12)' }
    default:          return { label: 'Оплачен', color: 'rgba(255,255,255,0.6)', bg: 'rgba(255,255,255,0.06)' }
  }
}

function OrdersOverview({ currentId, enabled, onSelect }: { currentId: string; enabled: boolean; onSelect: (id: string) => void }) {
  const [orders, setOrders] = useState<OrderSummary[] | null>(null)

  useEffect(() => {
    if (!enabled) {
      setOrders(null)
      return
    }
    const token = orderStorage.getAccessToken() ?? undefined
    orderApi.list(token).then(setOrders).catch(() => setOrders([]))
  }, [currentId, enabled])

  if (!enabled || !orders || orders.length < 2) return null

  return (
    <div className="fade-in" style={{ marginTop: '24px' }}>
      <div style={{ fontSize: '11px', fontWeight: 700, color: TEXT3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '12px', padding: '0 4px' }}>
        Ваши заказы · {orders.length}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {orders.map((o) => {
          const b = summaryBadge(o)
          const isCurrent = o.id === currentId
          return (
            <button
              key={o.id}
              type="button"
              onClick={() => { if (!isCurrent) onSelect(o.id) }}
              disabled={isCurrent}
              style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px',
                width: '100%', textAlign: 'left', fontFamily: 'inherit',
                background: isCurrent ? 'rgba(0,229,192,0.06)' : '#0f0f0f',
                border: `1px solid ${isCurrent ? 'rgba(0,229,192,0.3)' : BORDER}`,
                borderRadius: '14px', padding: '13px 16px',
                cursor: isCurrent ? 'default' : 'pointer',
                transition: 'border-color 0.15s, background 0.15s',
              }}
              onMouseEnter={(e) => {
                if (!isCurrent) {
                  e.currentTarget.style.borderColor = 'rgba(0,229,192,0.2)'
                  e.currentTarget.style.background = 'rgba(255,255,255,0.03)'
                }
              }}
              onMouseLeave={(e) => {
                if (!isCurrent) {
                  e.currentTarget.style.borderColor = BORDER
                  e.currentTarget.style.background = '#0f0f0f'
                }
              }}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px' }}>
                  <span style={{ fontSize: '14px', fontWeight: 700, color: '#fff' }}>#{o.invoice_id}</span>
                  {isCurrent && <span style={{ fontSize: '11px', color: ACCENT, fontWeight: 600 }}>текущий</span>}
                </div>
                <div style={{ fontSize: '12px', color: TEXT2, marginTop: '3px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '380px' }}>
                  {o.brief}
                </div>
              </div>
              <span style={{
                flexShrink: 0, fontSize: '12px', fontWeight: 600, color: b.color, background: b.bg,
                border: `1px solid ${b.color}33`, borderRadius: '20px', padding: '4px 11px', whiteSpace: 'nowrap',
              }}>
                {b.label}
              </span>
            </button>
          )
        })}
      </div>
      <div style={{ fontSize: '11px', color: TEXT3, marginTop: '10px', padding: '0 4px' }}>
        Нажмите на заказ, чтобы переключиться между ними.
      </div>
    </div>
  )
}

/* ── status steps ── */
const STEPS   = ['Оплата', 'Очередь', 'Создание', 'Готово']
const STEPKEYS = ['paid', 'queued', 'processing', 'completed'] as const

function OrderCard({ order, justPaid, confirmAwaitingPayment, canManage, pollingTooLong, onClear, onBack }: { order: OrderDetail; justPaid?: boolean; confirmAwaitingPayment?: boolean; canManage: boolean; pollingTooLong?: boolean; onClear: () => void; onBack: () => void }) {
  const gs = order.generation_status
  const ps = order.payment_status
  const awaitingPaymentConfirm = Boolean(confirmAwaitingPayment && ps === 'pending')
  const [celebrate, setCelebrate] = useState(false)

  useEffect(() => {
    if (gs !== 'completed') return
    const key = `numaestra_celebrated_${order.id}`
    if (!sessionStorage.getItem(key)) {
      setCelebrate(true)
      sessionStorage.setItem(key, '1')
    }
  }, [gs, order.id])

  let icon  = '⏳'
  let title = 'Обрабатываем заказ'
  let sub   = 'Пожалуйста, подождите...'

  if (awaitingPaymentConfirm)   { icon = '✓'; title = 'Подтверждаем оплату';  sub = 'Обычно занимает до минуты — не закрывайте страницу' }
  else if (ps === 'pending')    { icon = '💳'; title = 'Ожидание оплаты';     sub = 'После оплаты начнём создавать песню' }
  else if (gs === 'completed')  { icon = '🎵'; title = 'Ваша песня готова!'; sub = 'Все версии доступны для прослушивания' }
  else if (gs === 'failed')     { icon = '💔'; title = 'Ошибка генерации';   sub = 'Мы уже видим проблему — напишите нам, и мы поможем' }
  else if (gs === 'processing') { icon = '🎹'; title = 'Создаём песню';      sub = 'AI-студия работает над вашим треком...' }
  else if (gs === 'queued')     { icon = '⏱'; title = 'В очереди';          sub = 'Скоро начнём создание' }

  const idx       = gs === 'failed' ? 2 : STEPKEYS.findIndex(k => k === gs || (gs === 'new' && k === 'paid'))
  const isTerminal = gs === 'completed' || gs === 'failed'

  const [paying, setPaying] = useState(false)
  const [payError, setPayError] = useState<string | null>(null)

  async function handlePay() {
    setPayError(null); setPaying(true)
    try {
      const token = orderStorage.getAccessToken() ?? undefined
      const { payment_url } = await orderApi.paymentUrl(order.id, token)
      window.location.href = payment_url
    } catch (err) {
      setPayError(err instanceof Error ? err.message : 'Не удалось получить ссылку на оплату')
      setPaying(false)
    }
  }

  return (
    <div className="fade-in">
      {justPaid && ps !== 'pending' && (
        <div style={{
          background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.25)',
          borderRadius: '14px', padding: '14px 18px', marginBottom: '14px', fontSize: '14px', color: ACCENT,
        }}>
          Оплата принята — начинаем создавать вашу песню. Статус обновится автоматически.
        </div>
      )}
      {pollingTooLong && !isTerminal && ps !== 'pending' && (
        <div style={{
          background: 'rgba(245,158,11,0.08)', border: '1px solid rgba(245,158,11,0.25)',
          borderRadius: '14px', padding: '14px 18px', marginBottom: '14px', fontSize: '13px', color: '#fbbf24',
          display: 'flex', alignItems: 'flex-start', gap: '10px',
        }}>
          <span style={{ fontSize: '18px', lineHeight: 1, flexShrink: 0 }}>⚠️</span>
          <div>
            Генерация занимает дольше обычного. Обновите страницу через несколько минут — если проблема не исчезнет,{' '}
            <Link to="/legal/contacts" style={{ color: '#fbbf24', fontWeight: 600 }}>напишите нам</Link>.
          </div>
        </div>
      )}
      {/* Header */}
      <div className={`status-order-card${celebrate ? ' status-order-card--celebrate' : ''}`}>
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
        <div style={{ fontSize: 'clamp(20px, 5vw, 24px)', fontWeight: 800, letterSpacing: '-0.03em', marginBottom: '8px' }}>{title}</div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: gs === 'failed' ? '12px' : '32px' }}>{sub}</div>
        {gs === 'failed' && (
          <Link to="/legal/contacts" style={{ display: 'inline-block', fontSize: '14px', color: ACCENT, marginBottom: '24px', textDecoration: 'none' }}>
            Связаться с поддержкой →
          </Link>
        )}

        {/* Steps */}
        <div className="status-steps" aria-label="Этапы заказа">
          {STEPS.flatMap((label, i) => {
            const nodes = [
              <div key={label} className="status-step-col">
                <div
                  className={i === idx && !isTerminal ? 'progress-pulse status-step-dot' : 'status-step-dot'}
                  style={{
                    background: i <= idx ? ACCENT : 'rgba(255,255,255,0.07)',
                    color: i <= idx ? DARK : 'rgba(255,255,255,0.2)',
                  }}
                >
                  {i < idx ? '✓' : i + 1}
                </div>
                <div
                  className="status-step-label"
                  style={{ color: i <= idx ? TEXT2 : TEXT3 }}
                >
                  {label}
                </div>
              </div>,
            ]
            if (i < STEPS.length - 1) {
              nodes.push(
                <div
                  key={`${label}-line`}
                  className="status-step-line"
                  style={{ background: i < idx ? ACCENT : 'rgba(255,255,255,0.07)' }}
                  aria-hidden
                />,
              )
            }
            return nodes
          })}
        </div>

        {/* Бесплатное демо до оплаты: короткий фрагмент, без скачивания. */}
        {ps === 'pending' && <DemoPreview status={order.demo_status} url={order.demo_url} />}

        {/* Ожидание оплаты — только если клиент ещё не платил (не после SuccessURL). */}
        {ps === 'pending' && canManage && !awaitingPaymentConfirm && (
          <div style={{ marginTop: '28px' }}>
            <Button size="lg" fullWidth loading={paying} onClick={handlePay}>Перейти к оплате →</Button>
            {payError && <div style={{ fontSize: '13px', color: '#f87171', marginTop: '10px' }}>{payError}</div>}
            <div style={{ fontSize: '12px', color: TEXT3, marginTop: '10px' }}>
              Первая попытка не прошла? Нажмите ещё раз — заказ сохранён.
            </div>
          </div>
        )}

        {awaitingPaymentConfirm && (
          <div style={{ marginTop: '28px', fontSize: '13px', color: TEXT2 }}>
            Повторная оплата не нужна — дождитесь обновления статуса.
          </div>
        )}

        {/* Непрерывный прогресс генерации (после оплаты, до завершения) */}
        {ps !== 'pending' && !isTerminal && (
          <div style={{ marginTop: '28px', textAlign: 'left' }}>
            <GenerationProgress
              status={gs}
              paidAt={order.paid_at}
              generationPhase={order.generation_phase}
              generationProgress={order.generation_progress}
              tracksReady={order.tracks_ready}
            />
          </div>
        )}

        <div className="status-order-id" style={{ marginTop: '20px', fontSize: '12px', color: TEXT3, display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
          ID:
          <button
            onClick={() => copyText(order.id, 'ID заказа скопирован')}
            title="Скопировать ID"
            style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: 0,
              color: 'rgba(0,229,192,0.7)', fontFamily: 'monospace', fontSize: '12px',
              display: 'inline-flex', alignItems: 'center', gap: '5px',
            }}
          >
            {order.id}
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
            </svg>
          </button>
        </div>
        {!isTerminal && (
          <div style={{ fontSize: '12px', color: TEXT3, marginTop: '4px' }}>
            {awaitingPaymentConfirm ? 'Проверяем оплату каждые 2 секунды' : 'Обновляется каждые 10 секунд'}
          </div>
        )}
      </div>

      {/* Share — выше плеера и скачивания */}
      {gs === 'completed' && (
        <div className="status-share-highlight">
          <ShareSection orderId={order.id} shareRevoked={order.share_revoked ?? false} canManage={canManage} />
        </div>
      )}

      {/* Player */}
      {gs === 'completed' && order.tracks.length > 0 && (
        <div style={{ marginBottom: '16px' }}>
          <MusicPlayer tracks={order.tracks} />
        </div>
      )}

      {/* Downloads */}
      {gs === 'completed' && order.tracks.length > 0 && (
        <div style={{
          background: '#0f0f0f', border: `1px solid ${BORDER}`, borderRadius: '20px',
          padding: '20px 22px', marginBottom: '16px',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '14px' }}>
            <span style={{ fontSize: '11px', fontWeight: 700, color: TEXT3, letterSpacing: '0.07em', textTransform: 'uppercase' }}>
              Скачать треки
            </span>
            {order.tracks.length > 1 && (
              <button
                onClick={() => order.tracks.forEach((t, i) => setTimeout(
                  () => downloadFile(t.audio_url, `numaestra-${order.invoice_id}-v${t.index || i + 1}.mp3`), i * 400,
                ))}
                style={{
                  background: 'none', border: 'none', color: ACCENT, fontSize: '13px', fontWeight: 600,
                  cursor: 'pointer', fontFamily: 'inherit', padding: 0,
                }}
              >
                Скачать все ↓
              </button>
            )}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
            {order.tracks.map((t, i) => (
              <button
                key={t.index || i}
                onClick={() => downloadFile(t.audio_url, `numaestra-${order.invoice_id}-v${t.index || i + 1}.mp3`)}
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: '8px',
                  padding: '10px 14px', borderRadius: '12px', cursor: 'pointer', fontFamily: 'inherit',
                  background: 'rgba(0,229,192,0.08)', border: '1px solid rgba(0,229,192,0.25)',
                  color: ACCENT, fontSize: '13px', fontWeight: 600,
                }}
                onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(0,229,192,0.16)' }}
                onMouseLeave={(e) => { e.currentTarget.style.background = 'rgba(0,229,192,0.08)' }}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" />
                </svg>
                Вариант {t.index || i + 1}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Actions */}
      {!canManage && (
        <div style={{ marginBottom: '16px' }}>
          <AccessRecoveryForm orderId={order.id} />
        </div>
      )}

      {isTerminal && (
        <div style={{ display: 'flex', gap: '10px' }}>
          <Button variant="outlined" size="lg" fullWidth onClick={onClear}>Новый заказ</Button>
          <Button size="lg" fullWidth onClick={onBack}>На главную</Button>
        </div>
      )}
    </div>
  )
}

/**
 * Демо-фрагмент до оплаты: показываем «готовим…» пока генерируется и плеер-стрим
 * без скачивания, когда готово. Это тизер (короткий фрагмент с водяным знаком в
 * будущих версиях) — полную песню из 4 версий клиент получает после оплаты.
 */
function DemoPreview({ status, url }: { status?: string; url?: string }) {
  if (status === 'processing') {
    return (
      <div style={{
        marginTop: '24px', background: 'rgba(0,229,192,0.06)', border: `1px solid rgba(0,229,192,0.2)`,
        borderRadius: '16px', padding: '18px 20px', display: 'flex', alignItems: 'center', gap: '12px',
      }}>
        <div className="spin-anim" style={{ width: 20, height: 20, borderRadius: '50%', border: '2px solid rgba(255,255,255,0.1)', borderTopColor: ACCENT, flexShrink: 0 }} />
        <div style={{ fontSize: '13px', color: TEXT2 }}>
          Готовим бесплатное демо — это займёт пару минут. Можно дождаться его здесь.
        </div>
      </div>
    )
  }

  if (status === 'ready' && url) {
    return (
      <div style={{
        marginTop: '24px', background: '#0f0f0f', border: `1px solid ${BORDER}`,
        borderRadius: '16px', padding: '18px 20px',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '10px' }}>
          <span style={{
            fontSize: '10px', fontWeight: 800, letterSpacing: '0.08em', textTransform: 'uppercase',
            color: '#080808', background: ACCENT, borderRadius: '6px', padding: '3px 7px',
          }}>Демо</span>
          <span style={{ fontSize: '13px', fontWeight: 600 }}>Послушайте фрагмент бесплатно</span>
        </div>
        <audio
          src={url}
          controls
          controlsList="nodownload noplaybackrate"
          onContextMenu={(e) => e.preventDefault()}
          style={{ width: '100%', height: 40 }}
        />
        <div style={{ fontSize: '12px', color: TEXT3, marginTop: '10px', lineHeight: 1.5 }}>
          Это короткий фрагмент. После оплаты вы получите <b style={{ color: TEXT2 }}>4 полные версии</b> песни с возможностью скачать.
        </div>
      </div>
    )
  }

  return null
}

function ShareSection({ orderId, shareRevoked: initialRevoked, canManage }: { orderId: string; shareRevoked: boolean; canManage: boolean }) {
  const [shareRevoked, setShareRevoked] = useState(initialRevoked)
  const [shareBusy, setShareBusy] = useState(false)
  const [shareError, setShareError] = useState<string | null>(null)

  useEffect(() => {
    setShareRevoked(initialRevoked)
  }, [initialRevoked])

  async function toggleShare(revoke: boolean) {
    setShareError(null)
    setShareBusy(true)
    try {
      const token = orderStorage.getAccessToken() ?? undefined
      const res = revoke
        ? await orderApi.revokeShare(orderId, token)
        : await orderApi.restoreShare(orderId, token)
      setShareRevoked(res.share_revoked)
    } catch (err) {
      setShareError(err instanceof Error ? err.message : 'Не удалось изменить доступ к ссылке')
    } finally {
      setShareBusy(false)
    }
  }

  const shareUrl = `${window.location.origin}/s/${orderId}`

  return (
    <div>
      {!shareRevoked ? (
        <>
          <ShareBar
            url={shareUrl}
            text="Послушайте песню, которую мне сделали в Numaestra 🎵"
          />
          <div style={{ marginTop: '12px', textAlign: 'center' }}>
            {canManage && (
              <button
                type="button"
                onClick={() => toggleShare(true)}
                disabled={shareBusy}
                style={{
                  background: 'transparent', border: `1px solid ${BORDER}`, borderRadius: '10px',
                  padding: '8px 14px', fontSize: '12px', color: TEXT2, cursor: shareBusy ? 'wait' : 'pointer',
                  fontFamily: 'inherit',
                }}
              >
                {shareBusy ? 'Сохраняем…' : 'Отозвать публичную ссылку'}
              </button>
            )}
          </div>
        </>
      ) : (
        <div style={{
          background: 'rgba(245,158,11,0.08)', border: '1px solid rgba(245,158,11,0.25)',
          borderRadius: '14px', padding: '16px 18px', fontSize: '13px', color: '#fbbf24', textAlign: 'center',
        }}>
          <div style={{ marginBottom: '10px' }}>
            Публичная ссылка отозвана — по адресу <code style={{ fontSize: '12px' }}>/s/{orderId.slice(0, 8)}…</code> песню больше не открыть.
          </div>
          <button
            type="button"
            onClick={() => toggleShare(false)}
            disabled={shareBusy || !canManage}
            style={{
              background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.25)',
              borderRadius: '10px', padding: '8px 14px', fontSize: '12px', color: ACCENT,
              cursor: shareBusy || !canManage ? 'not-allowed' : 'pointer', fontFamily: 'inherit', fontWeight: 600,
              opacity: canManage ? 1 : 0.5,
            }}
          >
            {shareBusy ? 'Сохраняем…' : 'Восстановить доступ'}
          </button>
        </div>
      )}
      {shareError && (
        <div style={{
          marginTop: '10px', background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
          borderRadius: '10px', padding: '10px 14px', fontSize: '13px', color: '#ef4444', textAlign: 'center',
        }}>
          {shareError}
        </div>
      )}
    </div>
  )
}
