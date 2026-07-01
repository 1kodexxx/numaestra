import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { adminOrderApi } from '@entities/admin-order'
import type { AdminOrder } from '@entities/admin-order'
import { Spinner, Button, TextField } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, Panel, ErrorBanner, SuccessBanner, paymentBadge, generationBadge } from '@widgets/admin-layout'

export function AdminOrderDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [order, setOrder] = useState<AdminOrder | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const [sendOk, setSendOk] = useState(false)

  const [refunding, setRefunding] = useState(false)
  const [refundError, setRefundError] = useState<string | null>(null)

  const [confirming, setConfirming] = useState(false)
  const [confirmError, setConfirmError] = useState<string | null>(null)

  const [regenerating, setRegenerating] = useState(false)
  const [regenError, setRegenError] = useState<string | null>(null)

  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const [deletingDemo, setDeletingDemo] = useState(false)
  const [demoError, setDemoError] = useState<string | null>(null)

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

  async function handleConfirmPayment() {
    if (!confirm('Подтвердить оплату вручную? Убедитесь, что платёж есть в кабинете Robokassa.')) return
    setConfirmError(null); setConfirming(true)
    try {
      await adminOrderApi.confirmPayment(id)
      load()
    } catch (err) {
      setConfirmError(err instanceof ApiError ? err.message : 'Не удалось подтвердить оплату')
    } finally {
      setConfirming(false)
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

  async function handleDeleteDemo() {
    if (!confirm('Удалить демо этого заказа из хранилища? Превью и сохранённые полные клипы будут удалены безвозвратно.')) return
    setDemoError(null); setDeletingDemo(true)
    try {
      await adminOrderApi.deleteDemo(id)
      load()
    } catch (err) {
      setDemoError(err instanceof ApiError ? err.message : 'Не удалось удалить демо')
    } finally {
      setDeletingDemo(false)
    }
  }

  async function handleDelete() {
    if (!confirm('Удалить заказ безвозвратно? Треки в хранилище и запись в БД будут удалены.')) return
    setDeleteError(null); setDeleting(true)
    try {
      await adminOrderApi.remove(id)
      navigate('/admin/orders')
    } catch (err) {
      setDeleteError(err instanceof ApiError ? err.message : 'Не удалось удалить заказ')
    } finally {
      setDeleting(false)
    }
  }

  if (loading) return <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>
  if (error) return <ErrorBanner>{error}</ErrorBanner>
  if (!order) return null

  return (
    <div className="admin-page admin-page--order">
      <Link to="/admin/orders" style={{ color: A.txt2, textDecoration: 'none', fontSize: '13px' }}>← К списку заказов</Link>

      <div className="admin-order-title-row">
        <h1>Заказ #{order.invoice_id}</h1>
        {paymentBadge(order.payment_status)}
        {generationBadge(order.generation_status)}
      </div>

      {/* Info grid */}
      <Panel style={{ padding: '22px 24px', marginBottom: '16px' }}>
        <div className="admin-info-grid">
          <Info label="Email" value={order.email || '—'} />
          <Info label="Телефон" value={order.phone || '—'} />
          <Info label="Сумма" value={`${(order.amount_kopecks / 100).toFixed(0)} ₽`} />
          <Info label="Создан" value={new Date(order.created_at).toLocaleString('ru-RU')} />
          {order.consent_doc_version && (
            <Info
              label="Согласие"
              value={`${order.consent_doc_version}${order.consent_given_at ? ` · ${new Date(order.consent_given_at).toLocaleString('ru-RU')}` : ''}`}
            />
          )}
          <div className="admin-info-grid__full">
            <BriefBlock brief={order.brief} />
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

      {/* Демо: превью + полные клипы — просмотр, скачивание, удаление */}
      {(order.demo_url || (order.demo_clips && order.demo_clips.length > 0)) && (
        <Panel style={{ padding: '22px 24px', marginBottom: '16px' }}>
          <SectionTitle>Демо заказа</SectionTitle>

          {order.demo_url && (
            <div style={{ marginBottom: '16px' }}>
              <div style={{ fontSize: '12px', color: A.txt3, marginBottom: '8px' }}>Превью (с водяным знаком)</div>
              <audio controls src={order.demo_url} style={{ width: '100%', maxWidth: '420px', display: 'block', marginBottom: '8px' }} />
              <a href={order.demo_url} target="_blank" rel="noreferrer" style={{ color: A.accent, fontSize: '13px', fontWeight: 600, textDecoration: 'none' }}>
                ⬇ Скачать превью
              </a>
            </div>
          )}

          {order.demo_clips && order.demo_clips.length > 0 && (
            <div style={{ marginBottom: '16px' }}>
              <div style={{ fontSize: '12px', color: A.txt3, marginBottom: '8px' }}>Полные клипы ({order.demo_clips.length})</div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px' }}>
                {order.demo_clips.map((c) => (
                  <a
                    key={c.index} href={c.audio_url} target="_blank" rel="noreferrer"
                    style={{
                      display: 'inline-flex', alignItems: 'center', gap: '8px',
                      background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.3)',
                      borderRadius: '12px', padding: '9px 14px',
                      color: A.accent, fontSize: '13px', fontWeight: 600, textDecoration: 'none',
                    }}
                  >
                    ⬇ Клип {c.index}
                  </a>
                ))}
              </div>
            </div>
          )}

          {order.generation_status === 'completed' ? (
            <div style={{ fontSize: '12px', color: A.txt3 }}>
              Заказ завершён — полные клипы уже доставлены клиенту как треки, удалять демо нельзя. Используйте «Удалить заказ».
            </div>
          ) : (
            <>
              <div style={{ fontSize: '13px', color: A.txt2, marginBottom: '12px' }}>
                Освободить место в хранилище: удалить превью и сохранённые полные клипы этого заказа. Действие необратимо.
              </div>
              <Button
                variant="outlined"
                onClick={handleDeleteDemo}
                loading={deletingDemo}
                style={{ color: '#f87171', borderColor: 'rgba(239,68,68,0.4)' }}
              >
                Удалить демо
              </Button>
            </>
          )}
          {demoError && <div style={{ marginTop: '12px' }}><ErrorBanner>{demoError}</ErrorBanner></div>}
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

      {/* Manual payment confirm */}
      {order.payment_status === 'pending' && (
        <Panel style={{ padding: '20px 24px', marginBottom: '16px' }}>
          <SectionTitle>Подтверждение оплаты</SectionTitle>
          <div style={{ fontSize: '13px', color: A.txt2, marginBottom: '14px' }}>
            Клиент оплатил, но заказ остался в ожидании? Проверьте платёж в кабинете Robokassa и подтвердите вручную — генерация запустится без повторной оплаты.
          </div>
          <Button onClick={handleConfirmPayment} loading={confirming}>
            Подтвердить оплату
          </Button>
          {confirmError && <div style={{ marginTop: '12px' }}><ErrorBanner>{confirmError}</ErrorBanner></div>}
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

        <form onSubmit={handleSendFeedback} className="admin-form-stack" style={{ gap: '14px' }}>
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

      <Panel style={{ padding: '20px 24px', marginTop: '16px' }}>
        <SectionTitle>Удаление заказа</SectionTitle>
        <div style={{ fontSize: '13px', color: A.txt2, marginBottom: '14px' }}>
          Полностью удалить заказ из базы и MP3-треки из хранилища. Действие необратимо.
        </div>
        <Button
          variant="outlined"
          onClick={handleDelete}
          loading={deleting}
          style={{ color: '#f87171', borderColor: 'rgba(239,68,68,0.4)' }}
        >
          Удалить заказ
        </Button>
        {deleteError && <div style={{ marginTop: '12px' }}><ErrorBanner>{deleteError}</ErrorBanner></div>}
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

// formatBrief раскладывает бриф для чтения: служебные маркеры Suno и заголовки
// секций текста ([Куплет], [Припев], [Аутро], [Bridge]…) выносятся на новую строку,
// существующие переносы сохраняются. Значение в БД не меняется — это только показ.
function formatBrief(brief: string): string {
  return brief
    // Каждый маркер Suno — с новой строки, с пустой строкой-разделителем перед ним.
    .replace(/[ \t]*(#SUNO_(?:TAGS|DESC|LYRICS)#)/g, '\n\n$1')
    // Заголовки секций текста в квадратных скобках — с новой строки (стихи, не блок).
    .replace(/[ \t]*(\[[^\]]+\])/g, '\n\n$1')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

// BriefBlock — читаемый вывод брифа в админке: секции и куплеты по строкам,
// моноширинный шрифт, сохранение переносов (white-space: pre-wrap).
function BriefBlock({ brief }: { brief: string }) {
  return (
    <div>
      <div style={{ fontSize: '11px', color: A.txt3, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '8px' }}>Бриф</div>
      <pre
        style={{
          margin: 0,
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
          fontSize: '13px',
          color: A.txt,
          lineHeight: 1.65,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          background: 'rgba(255,255,255,0.02)',
          border: `1px solid ${A.border}`,
          borderRadius: '12px',
          padding: '14px 16px',
        }}
      >
        {formatBrief(brief)}
      </pre>
    </div>
  )
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <div style={{ fontSize: '11px', fontWeight: 700, color: A.txt3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '14px' }}>{children}</div>
}
