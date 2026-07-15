import { useEffect, useId, useMemo, useState } from 'react'
import { Button, TextField } from '@shared/ui'
import { theme } from '@shared/lib/theme'
import { useFocusTrap } from '@shared/lib/useFocusTrap'
import { suggestEmailFix } from '@shared/lib/emailHint'
import { usePublicConfig } from '@shared/lib/usePublicConfig'

const ACCENT = theme.accent
const TEXT2 = theme.text2
const TEXT3 = theme.text3

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

// formatRub форматирует копейки в «1 500 ₽» (тот же стиль, что price_label).
function formatRub(kopecks: number): string {
  return `${Math.round(kopecks / 100).toLocaleString('ru-RU')} ₽`
}

interface ContactModalProps {
  loading: boolean
  error: string | null
  priceLabel: string
  // Зачёркнутая «старая» цена (маркетинг). Показывается только без промокода —
  // при активной скидке перечёркивается сама priceLabel, второй строкой не мешаем.
  oldPriceLabel?: string
  // Полная цена и скидка по промокоду (в копейках). При discountKopecks > 0 блок
  // цены показывает перечёркнутую полную цену, итоговую со скидкой и экономию.
  // Это ровно та сумма, что уйдёт в Robokassa — бэкенд применяет тот же промокод
  // при создании заказа (order.AmountKopecks), вебхук сверяет её же.
  priceKopecks?: number
  discountKopecks?: number
  discountLabel?: string
  onClose: () => void
  onSubmit: (email: string, phone: string) => void
}

/** Email/phone capture + price summary, shown before redirecting to payment. */
export function ContactModal({ loading, error, priceLabel, oldPriceLabel, priceKopecks, discountKopecks = 0, discountLabel, onClose, onSubmit }: ContactModalProps) {
  const { demo_enabled: demoEnabled } = usePublicConfig()
  const titleId = useId()
  const trapRef = useFocusTrap(true)
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [agree, setAgree] = useState(false)
  const [err, setErr] = useState('')

  // Подсказка об опечатке в домене («gmial.com» → «gmail.com») и признак валидного
  // адреса — для строки подтверждения перед оплатой. Email после создания заказа
  // не изменить, поэтому ловим ошибку здесь, до отправки.
  const suggestion = useMemo(() => suggestEmailFix(email), [email])
  const emailValid = EMAIL_RE.test(email.trim())

  // Скидка по промокоду: показываем итоговую цену и экономию (та же сумма уйдёт в оплату).
  const hasDiscount = discountKopecks > 0 && typeof priceKopecks === 'number'
  const finalKopecks = hasDiscount ? priceKopecks! - discountKopecks : (priceKopecks ?? 0)

  // Фокус на контейнер диалога — ТОЛЬКО при открытии (a11y/скринридер). Раньше
  // это жило в эффекте с [loading, onClose], который перезапускался на каждый
  // ре-рендер родителя (onClose — инлайн-функция). На мобиле повторный focus крал
  // фокус у тапнутого инпута, и экранная клавиатура мгновенно закрывалась.
  useEffect(() => {
    trapRef.current?.focus()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape' && !loading) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [loading, onClose])

  function go() {
    if (!email.trim()) { setErr('Укажите email — мы отправим треки на почту'); return }
    if (!EMAIL_RE.test(email.trim())) { setErr('Некорректный формат email'); return }
    if (!agree) { setErr('Необходимо согласие с условиями'); return }
    setErr('')
    onSubmit(email.trim(), phone.trim())
  }

  return (
    <div
      role="presentation"
      className="modal-backdrop"
      onClick={() => { if (!loading) onClose() }}
      style={{
        position: 'fixed', inset: 0, zIndex: 100,
        background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(6px)',
        display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
        overflowY: 'auto', padding: '16px',
      }}
    >
      <div
        ref={trapRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-busy={loading}
        tabIndex={-1}
        className="modal-panel"
        onClick={(e) => e.stopPropagation()}
        style={{
          background: theme.surface, border: `1px solid ${theme.border}`,
          borderRadius: '28px', padding: 'clamp(24px, 5vw, 36px) clamp(20px, 5vw, 32px)',
          width: '100%', maxWidth: '420px', margin: 'auto',
          boxShadow: 'var(--elevation-5)', outline: 'none',
        }}
      >
        <div id={titleId} style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '6px' }}>
          {loading
            ? demoEnabled ? 'Готовим демо…' : 'Создаём заказ…'
            : demoEnabled ? 'Получить бесплатное демо' : 'Заказать песню'}
        </div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '28px' }}>
          {loading
            ? demoEnabled
              ? 'Секунду — создаём заказ и открываем страницу с вашим демо.'
              : 'Секунду — создаём заказ и открываем страницу оплаты.'
            : demoEnabled
              ? 'Оставьте email — пришлём демо вашей песни. Оплата потом, только если понравится.'
              : 'Оставьте email — на него придёт ссылка на готовую песню. 4 версии за ~10 минут после оплаты.'}
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', marginBottom: '24px' }}>
          <div>
            <TextField label="Email" type="email" required value={email} onChange={setEmail} placeholder="your@email.com" surfaceColor={theme.surface} disabled={loading} />
            {suggestion && !loading && (
              <button
                type="button"
                onClick={() => { setEmail(suggestion); setErr('') }}
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: '6px',
                  marginTop: '8px', padding: '7px 12px',
                  background: 'rgba(0,229,192,0.08)', border: '1px solid rgba(0,229,192,0.22)',
                  borderRadius: '10px', cursor: 'pointer', fontFamily: 'inherit',
                  fontSize: '12.5px', color: TEXT2, textAlign: 'left', lineHeight: 1.4,
                }}
              >
                Возможно, вы имели в виду&nbsp;<b style={{ color: ACCENT }}>{suggestion}</b>?
              </button>
            )}
          </div>
          <TextField label="Телефон (необязательно)" type="tel" value={phone} onChange={setPhone} placeholder="+7 999 000 00 00" surfaceColor={theme.surface} disabled={loading} />
        </div>

        <div style={{
          background: 'rgba(0,229,192,0.07)', border: '1px solid rgba(0,229,192,0.18)',
          borderRadius: '16px', padding: '16px 20px',
          textAlign: 'center', marginBottom: '20px',
        }}>
          <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '4px' }}>
            {demoEnabled ? 'Полная песня — потом, если понравится' : '4 версии песни · готово за ~10 минут'}
          </div>
          {hasDiscount ? (
            <>
              <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'center', gap: '10px' }}>
                <span style={{ fontSize: '16px', color: TEXT3, textDecoration: 'line-through' }}>{priceLabel}</span>
                <span style={{ fontSize: '28px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em' }}>{formatRub(finalKopecks)}</span>
              </div>
              <div style={{ fontSize: '12px', color: ACCENT, fontWeight: 700, marginTop: '4px' }}>
                {discountLabel ? `${discountLabel} · ` : ''}сэкономите {formatRub(discountKopecks)}
              </div>
            </>
          ) : oldPriceLabel ? (
            <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'center', gap: '10px' }}>
              <span style={{ fontSize: '16px', color: TEXT3, textDecoration: 'line-through' }}>{oldPriceLabel}</span>
              <span style={{ fontSize: '28px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em' }}>{priceLabel}</span>
            </div>
          ) : (
            <div style={{ fontSize: '28px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em' }}>{priceLabel}</div>
          )}
          <div style={{ fontSize: '12px', color: TEXT3, marginTop: '2px' }}>4 версии · один платёж · без подписок</div>
        </div>

        <label style={{ display: 'flex', alignItems: 'flex-start', gap: '10px', cursor: loading ? 'default' : 'pointer', marginBottom: '18px', opacity: loading ? 0.6 : 1 }}>
          <input
            type="checkbox"
            checked={agree}
            disabled={loading}
            onChange={(e) => { setAgree(e.target.checked); if (e.target.checked) setErr('') }}
            style={{ width: 18, height: 18, marginTop: '1px', accentColor: ACCENT, cursor: loading ? 'default' : 'pointer', flexShrink: 0 }}
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
            fontSize: '13px', color: theme.error, marginBottom: '14px',
          }}>
            {err || error}
          </div>
        )}

        {emailValid && !loading && !suggestion && (
          <div style={{
            display: 'flex', alignItems: 'flex-start', gap: '8px',
            background: 'rgba(255,255,255,0.04)', border: `1px solid ${theme.border}`,
            borderRadius: '12px', padding: '10px 14px', marginBottom: '14px',
            fontSize: '12.5px', color: TEXT2, lineHeight: 1.45,
          }}>
            <span aria-hidden style={{ flexShrink: 0 }}>📧</span>
            <span>{demoEnabled ? 'Демо и ссылка на заказ придут' : 'Ссылка на заказ и готовая песня придут'} на <b style={{ color: '#fff', wordBreak: 'break-all' }}>{email.trim()}</b> — проверьте, что адрес верный.</span>
          </div>
        )}

        <div style={{ display: 'flex', gap: '10px' }}>
          <Button variant="text" size="lg" onClick={onClose} disabled={loading} style={{ flex: 1 }}>Отмена</Button>
          <Button size="lg" onClick={go} loading={loading} disabled={!agree || loading} style={{ flex: 2 }}>
            {loading
              ? demoEnabled ? 'Готовим демо…' : 'Создаём заказ…'
              : demoEnabled ? 'Слушать демо бесплатно →' : 'Продолжить →'}
          </Button>
        </div>
      </div>
    </div>
  )
}
