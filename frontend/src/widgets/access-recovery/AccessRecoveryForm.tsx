import { useState } from 'react'
import { orderApi } from '@entities/order'
import { Button, TextField } from '@shared/ui'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

interface AccessRecoveryFormProps {
  orderId: string
  compact?: boolean
}

export function AccessRecoveryForm({ orderId, compact }: AccessRecoveryFormProps) {
  const [email, setEmail] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  async function handleSubmit() {
    setError(null)
    setSuccess(null)
    const trimmed = email.trim()
    if (!trimmed) {
      setError('Укажите email, который вы вводили при заказе')
      return
    }
    if (!EMAIL_RE.test(trimmed)) {
      setError('Некорректный формат email')
      return
    }

    setBusy(true)
    try {
      const { message } = await orderApi.requestAccessLink(orderId, trimmed)
      setSuccess(message)
      setEmail('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось отправить письмо')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{
      marginTop: compact ? 0 : '16px',
      padding: compact ? '14px 16px' : '18px 20px',
      background: 'rgba(0,229,192,0.06)',
      border: '1px solid rgba(0,229,192,0.16)',
      borderRadius: '14px',
      textAlign: 'left',
    }}>
      <div style={{ fontSize: compact ? '13px' : '14px', fontWeight: 700, color: '#fff', marginBottom: '6px' }}>
        Восстановить доступ к заказу
      </div>
      <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '12px', lineHeight: 1.5 }}>
        Укажите email из заказа — пришлём ссылку для оплаты и управления заказом.
      </div>
      <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <div style={{ flex: '1 1 200px' }} onKeyDown={(e) => e.key === 'Enter' && handleSubmit()}>
          <TextField
            label="Email"
            type="email"
            value={email}
            onChange={setEmail}
            placeholder="your@email.com"
            surfaceColor="#0f0f0f"
          />
        </div>
        <Button size="lg" loading={busy} onClick={handleSubmit}>
          Выслать ссылку
        </Button>
      </div>
      {success && (
        <div style={{
          marginTop: '12px', fontSize: '13px', color: '#4ade80',
          background: 'rgba(34,197,94,0.08)', border: '1px solid rgba(34,197,94,0.2)',
          borderRadius: '10px', padding: '10px 14px',
        }}>
          {success}
        </div>
      )}
      {error && (
        <div style={{
          marginTop: '12px', fontSize: '13px', color: '#ef4444',
          background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
          borderRadius: '10px', padding: '10px 14px',
        }}>
          {error}
        </div>
      )}
    </div>
  )
}
