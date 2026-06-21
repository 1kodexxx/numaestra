import { useState } from 'react'
import { Button, TextField } from '@shared/ui'

const ACCENT = '#00e5c0'
const TEXT2 = 'rgba(255,255,255,0.48)'
const TEXT3 = 'rgba(255,255,255,0.22)'

interface ContactModalProps {
  loading: boolean
  error: string | null
  onClose: () => void
  onSubmit: (email: string, phone: string) => void
}

/** Email/phone capture + price summary, shown before redirecting to payment. */
export function ContactModal({ loading, error, onClose, onSubmit }: ContactModalProps) {
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [err, setErr] = useState('')

  function go() {
    if (!email && !phone) { setErr('Укажите email или телефон'); return }
    setErr('')
    onSubmit(email, phone)
  }

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0, zIndex: 100,
        background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(6px)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: '16px',
      }}
    >
      <div className="scale-in" onClick={(e) => e.stopPropagation()} style={{
        background: '#0f0f0f', border: '1px solid rgba(255,255,255,0.08)',
        borderRadius: '28px', padding: '36px 32px',
        width: '100%', maxWidth: '420px',
        boxShadow: 'var(--elevation-5)',
      }}>
        <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '6px' }}>
          Оформление заказа
        </div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '28px' }}>
          Отправим готовые треки на вашу почту
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', marginBottom: '24px' }}>
          <TextField label="Email" type="email" value={email} onChange={setEmail} placeholder="your@email.com" surfaceColor="#0f0f0f" />
          <TextField label="Телефон (необязательно)" type="tel" value={phone} onChange={setPhone} placeholder="+7 999 000 00 00" surfaceColor="#0f0f0f" />
        </div>

        <div style={{
          background: 'rgba(0,229,192,0.07)', border: '1px solid rgba(0,229,192,0.18)',
          borderRadius: '16px', padding: '16px 20px',
          textAlign: 'center', marginBottom: '20px',
        }}>
          <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '4px' }}>4 уникальных версии</div>
          <div style={{ fontSize: '28px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em' }}>2 000 ₽</div>
          <div style={{ fontSize: '12px', color: TEXT3, marginTop: '2px' }}>Один платёж, без подписок</div>
        </div>

        {(err || error) && (
          <div style={{
            background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
            borderRadius: '12px', padding: '10px 14px',
            fontSize: '13px', color: '#ef4444', marginBottom: '14px',
          }}>
            {err || error}
          </div>
        )}

        <div style={{ display: 'flex', gap: '10px' }}>
          <Button variant="text" size="lg" onClick={onClose} style={{ flex: 1 }}>Отмена</Button>
          <Button size="lg" onClick={go} loading={loading} style={{ flex: 2 }}>К оплате →</Button>
        </div>
      </div>
    </div>
  )
}
