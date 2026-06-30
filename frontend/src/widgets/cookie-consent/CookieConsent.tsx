import { useState } from 'react'
import { Link } from 'react-router-dom'
import { theme } from '@shared/lib/theme'
import { grantAnalyticsConsent, hasAnalyticsConsent } from '@shared/lib/analytics'

/**
 * Минимальный баннер-уведомление о cookie/аналитике. Показывается один раз, пока
 * пользователь не нажал «Хорошо» (решение хранится в localStorage). Счётчик
 * Метрики работает по умолчанию (стандартная веб-аналитика); согласие здесь —
 * дополнительный opt-in на запись сессий (Вебвизор), см. grantAnalyticsConsent().
 *
 * Слим-бар у нижнего края: компактный, с safe-area, переносится в столбик на
 * узких экранах, чтобы не перекрывать критичный UI и исчезать за один тап.
 */
export function CookieConsent() {
  const [visible, setVisible] = useState(() => !hasAnalyticsConsent())
  if (!visible) return null

  function accept() {
    grantAnalyticsConsent()
    setVisible(false)
  }

  return (
    <div
      role="dialog"
      aria-label="Согласие на использование cookie"
      style={{
        position: 'fixed',
        left: '12px',
        right: '12px',
        bottom: 'calc(12px + env(safe-area-inset-bottom, 0px))',
        // Ниже навбара (50) и оверлея/листа мобильного меню (48/49), чтобы не
        // перекрывать навигацию: открытое меню оказывается поверх баннера.
        zIndex: 47,
        maxWidth: '680px',
        margin: '0 auto',
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        flexWrap: 'wrap',
        padding: '12px 16px',
        borderRadius: '14px',
        background: theme.surface2,
        border: `1px solid ${theme.border}`,
        boxShadow: '0 12px 40px -16px rgba(0,0,0,0.8)',
      }}
    >
      <span style={{ flex: 1, minWidth: '200px', fontSize: '12.5px', lineHeight: 1.5, color: theme.text2 }}>
        Мы используем cookie и Яндекс.Метрику для работы сайта и аналитики.
        Нажимая «Хорошо», вы соглашаетесь с записью сессий для улучшения сервиса.{' '}
        <Link to="/legal/privacy" style={{ color: theme.accent, textDecoration: 'none' }}>
          Политика конфиденциальности
        </Link>
        .
      </span>
      <button
        type="button"
        onClick={accept}
        style={{
          flexShrink: 0,
          border: 'none',
          cursor: 'pointer',
          borderRadius: '10px',
          padding: '9px 20px',
          fontSize: '13px',
          fontWeight: 700,
          color: '#04130f',
          background: `linear-gradient(135deg, ${theme.accent}, ${theme.accent2})`,
          WebkitTapHighlightColor: 'transparent',
        }}
      >
        Хорошо
      </button>
    </div>
  )
}
