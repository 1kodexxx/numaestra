import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { adminOrderApi } from '@entities/admin-order'
import type { AdminOrder } from '@entities/admin-order'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, Panel, ErrorBanner, SuccessBanner, paymentBadge, generationBadge } from '@widgets/admin-layout'

export function AdminOrderDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const [order, setOrder] = useState<AdminOrder | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const [sendOk, setSendOk] = useState(false)

  const [refunding, setRefunding] = useState(false)
  const [refundError, setRefundError] = useState<string | null>(null)

  const [regenerating, setRegenerating] = useState(false)
  const [regenError, setRegenError] = useState<string | null>(null)

  function load() {
    setLoading(true)
    adminOrderApi
      .get(id)
      .then((o) => { setOrder(o); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [id])

  async function handleSendFeedback(e: React.FormEvent) {
    e.preventDefault()
    setSendError(null); setSendOk(false); setSending(true)
    try {
      await adminOrderApi.sendFeedback(id, message)
      setMessage(''); setSendOk(true); load()
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : 'Не удалось отправить сообщение')
    } finally {
      setSending(false)
    }
  }

  async function handleRefund() {
    if (!confirm('Инициировать возврат платежа клиенту через Robokassa?')) return
    setRefundError(null); setRefunding(true)
    try {
      await adminOrderApi.refund(id)
      load()
    } catch (err) {
      setRefundError(err instanceof ApiError ? err.message : 'Не удалось выполнить возврат')
    } finally {
      setRefunding(false)
    }
  }

  async function handleRegenerate() {
    if (!confirm('Поставить заказ заново в очередь генерации?')) return
    setRegenError(null); setRegenerating(true)
    try {
      await adminOrderApi.regenerate(id)
      load()
    } catch (err) {
      setRegenError(err instanceof ApiError ? err.message : 'Не удалось перегенерировать')
    } finally {
      setRegenerating(false)
    }
  }

  if (loading) return <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>
  if (error) return <ErrorBanner>{error}</ErrorBanner>
  if (!order) return null

  return (
    <div style={{ maxWidth: 760 }}>
      <Link to="/admin/orders" style={{ color: A.txt2, textDecoration: 'none', fontSize: '13px' }}>← К списку заказов</Link>

      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', margin: '12px 0 28px', flexWrap: 'wrap' }}>
        <h1 style={{ fontSize: '26px', fontWeight: 800, letterSpacing: '-0.03em' }}>Заказ #{order.invoice_id}</h1>
        {paymentBadge(order.payment_status)}
        {generationBadge(order.generation_status)}
      </div>

      {/* Info grid */}
      <Panel style={{ padding: '22px 24px', marginBottom: '16px' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '18px' }}>
          <Info label="Email" value={order.email || '—'} />
          <Info label="Телефон" value={order.phone || '—'} />
          <Info label="Сумма" value={`${(order.amount_kopecks / 100).toFixed(0)} ₽`} />
          <Info label="Создан" value={new Date(order.created_at).toLocaleString('ru-RU')} />
          <div style={{ gridColumn: '1 / -1' }}>
            <Info label="Бриф" value={order.brief} />
          </div>
        </div>
      </Panel>

      {/* Tracks */}
      {order.tracks.length > 0 && (
        <Panel style={{ padding: '22px 24px', marginBottom: '16px' }}>
          <SectionTitle>Готовые треки</SectionTitle>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px' }}>
            {order.tracks.map((t) => (
              <a
                key={t.index} href={t.audio_url} target="_blank" rel="noreferrer"
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: '8px',
                  background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.3)',
                  borderRadius: '12px', padding: '9px 14px',
                  color: A.accent, fontSize: '13px', fontWeight: 600, textDecoration: 'none',
                }}
              >
                🎧 Вариант {t.index}
              </a>
            ))}
          </div>
        </Panel>
      )}

      {/* Перегенерация — для оплаченных заказов с ошибкой генерации */}
      {order.payment_status === 'paid' && order.generation_status === 'failed' && (
        <Panel style={{ padding: '20px 24px', marginBottom: '16px' }}>
          <SectionTitle>Перегенерация</SectionTitle>
          <div style={{ fontSize: '13px', color: A.txt2, marginBottom: '14px' }}>
            Заказ оплачен, но генерация сорвалась. Поставьте его заново в очередь — клиент получит трек без повторной оплаты.
          </div>
          <Button onClick={handleRegenerate} loading={regenerating}>Перегенерировать</Button>
          {regenError && <div style={{ marginTop: '12px' }}><ErrorBanner>{regenError}</ErrorBanner></div>}
        </Panel>
      )}

      {/* Refund */}
      {order.payment_status === 'paid' && (
        <Panel style={{ padding: '20px 24px', marginBottom: '16px' }}>
          <SectionTitle>Возврат оплаты</SectionTitle>
          <div style={{ fontSize: '13px', color: A.txt2, marginBottom: '14px' }}>
            Вернуть платёж клиенту через Robokassa. Действие необратимо.
          </div>
          <Button variant="outlined" onClick={handleRefund} loading={refunding} style={{ color: '#f87171', borderColor: 'rgba(239,68,68,0.4)' }}>
            Вернуть оплату
          </Button>
          {refundError && <div style={{ marginTop: '12px' }}><ErrorBanner>{refundError}</ErrorBanner></div>}
        </Panel>
      )}

      {/* Feedback */}
      <Panel style={{ padding: '22px 24px' }}>
        <SectionTitle>Обратная связь клиенту</SectionTitle>

        {order.admin_feedback && (
          <div style={{
            background: A.surface2, border: `1px solid ${A.border}`, borderRadius: '12px',
            padding: '14px 16px', fontSize: '13px', color: A.txt, marginBottom: '16px', lineHeight: 1.5,
          }}>
            <div style={{ fontSize: '11px', color: A.txt3, marginBottom: '6px' }}>
              Последнее сообщение{order.admin_feedback_at ? ` · ${new Date(order.admin_feedback_at).toLocaleString('ru-RU')}` : ''}
            </div>
            {order.admin_feedback}
          </div>
        )}

        <form onSubmit={handleSendFeedback} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <TextField
            label="Сообщение клиенту"
            value={message} onChange={setMessage}
            multiline rows={3}
            placeholder="Например: уточните, пожалуйста, дату праздника"
            surfaceColor={A.surface}
          />
          {sendError && <ErrorBanner>{sendError}</ErrorBanner>}
          {sendOk && <SuccessBanner>Письмо отправлено клиенту.</SuccessBanner>}
          <Button type="submit" loading={sending} disabled={!order.email || !message.trim()} style={{ alignSelf: 'flex-start' }}>
            Отправить
          </Button>
          {!order.email && <div style={{ fontSize: '12px', color: A.txt3 }}>У заказа не указан email — отправка недоступна.</div>}
        </form>
      </Panel>
    </div>
  )
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div style={{ fontSize: '11px', color: A.txt3, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '4px' }}>{label}</div>
      <div style={{ fontSize: '14px', color: A.txt, lineHeight: 1.5, wordBreak: 'break-word' }}>{value}</div>
    </div>
  )
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <div style={{ fontSize: '11px', fontWeight: 700, color: A.txt3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '14px' }}>{children}</div>
}
