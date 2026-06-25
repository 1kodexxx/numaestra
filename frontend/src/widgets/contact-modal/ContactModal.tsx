import { useEffect, useId, useRef, useState } from 'react'
import { Button, TextField } from '@shared/ui'

const ACCENT = '#00e5c0'
const TEXT2 = 'rgba(255,255,255,0.48)'
const TEXT3 = 'rgba(255,255,255,0.22)'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

interface ContactModalProps {
  loading: boolean
  error: string | null
  priceLabel: string
  onClose: () => void
  onSubmit: (email: string, phone: string) => void
}

/** Email/phone capture + price summary, shown before redirecting to payment. */
export function ContactModal({ loading, error, priceLabel, onClose, onSubmit }: ContactModalProps) {
  const titleId = useId()
  const dialogRef = useRef<HTMLDivElement>(null)
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [agree, setAgree] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    dialogRef.current?.focus()
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape' && !loading) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [loading, onClose])

  function go() {
    if (!email && !phone) { setErr('Укажите email или телефон'); return }
    if (email && !EMAIL_RE.test(email.trim())) { setErr('Некорректный формат email'); return }
    if (!agree) { setErr('Необходимо согласие с условиями'); return }
    setErr('')
    onSubmit(email.trim(), phone.trim())
  }

  return (
    <div
      role="presentation"
      onClick={() => { if (!loading) onClose() }}
      style={{
        position: 'fixed', inset: 0, zIndex: 100,
        background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(6px)',
        display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
        overflowY: 'auto', padding: '16px',
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="scale-in"
        onClick={(e) => e.stopPropagation()}
        style={{
          background: '#0f0f0f', border: '1px solid rgba(255,255,255,0.08)',
          borderRadius: '28px', padding: 'clamp(24px, 5vw, 36px) clamp(20px, 5vw, 32px)',
          width: '100%', maxWidth: '420px', margin: 'auto',
          boxShadow: 'var(--elevation-5)', outline: 'none',
        }}
      >
        <div id={titleId} style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '6px' }}>
          Оформление заказа
        </div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '28px' }}>
          Отправим готовые треки на вашу почту
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', marginBottom: '24px' }}>
          <TextField label="Email" type="email" required value={email} onChange={setEmail} placeholder="your@email.com" surfaceColor="#0f0f0f" />
          <TextField label="Телефон (необязательно)" type="tel" value={phone} onChange={setPhone} placeholder="+7 999 000 00 00" surfaceColor="#0f0f0f" />
        </div>

        <div style={{
          background: 'rgba(0,229,192,0.07)', border: '1px solid rgba(0,229,192,0.18)',
          borderRadius: '16px', padding: '16px 20px',
          textAlign: 'center', marginBottom: '20px',
        }}>
          <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '4px' }}>4 уникальных версии</div>
          <div style={{ fontSize: '28px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em' }}>{priceLabel}</div>
          <div style={{ fontSize: '12px', color: TEXT3, marginTop: '2px' }}>Один платёж, без подписок</div>
        </div>

        <label style={{ display: 'flex', alignItems: 'flex-start', gap: '10px', cursor: 'pointer', marginBottom: '18px' }}>
          <input
            type="checkbox"
            checked={agree}
            onChange={(e) => { setAgree(e.target.checked); if (e.target.checked) setErr('') }}
            style={{ width: 18, height: 18, marginTop: '1px', accentColor: ACCENT, cursor: 'pointer', flexShrink: 0 }}
          />
          <span style={{ fontSize: '12px', color: TEXT2, lineHeight: 1.5 }}>
            Я согласен с{' '}
            <a href="/legal/offer" target="_blank" rel="noopener noreferrer" style={{ color: ACCENT, textDecoration: 'none' }}>условиями оферты</a>,{' '}
            <a href="/legal/privacy" target="_blank" rel="noopener noreferrer" style={{ color: ACCENT, textDecoration: 'none' }}>политикой конфиденциальности</a>
            {' '}и{' '}
            <a href="/legal/consent" target="_blank" rel="noopener noreferrer" style={{ color: ACCENT, textDecoration: 'none' }}>обработкой персональных данных</a>.
          </span>
        </label>

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
          <Button variant="text" size="lg" onClick={onClose} disabled={loading} style={{ flex: 1 }}>Отмена</Button>
          <Button size="lg" onClick={go} loading={loading} disabled={!agree} style={{ flex: 2 }}>К оплате →</Button>
        </div>
      </div>
    </div>
  )
}
