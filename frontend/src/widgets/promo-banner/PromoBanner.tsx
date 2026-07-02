import { useState } from 'react'
import { Link } from 'react-router-dom'
import { theme } from '@shared/lib/theme'

// Параметры акции — меняются здесь. Чтобы показать плашку заново после смены
// условий, поменяй code (ключ скрытия завязан на него).
const PROMO = {
  code: '1WEEK',
  discount: '−500 ₽',
  note: 'только на этой неделе',
}

const DISMISS_KEY = `numaestra_promo_dismissed_${PROMO.code}`

// PromoBanner — тонкая плашка-объявление акции вверху публичных страниц.
// Закрывается крестиком (запоминается в localStorage по коду акции), клик по
// «Применить» ведёт на главную с ?promo=CODE — код применится автоматически.
export function PromoBanner() {
  const [hidden, setHidden] = useState(() => {
    try {
      return localStorage.getItem(DISMISS_KEY) === '1'
    } catch {
      return false
    }
  })

  if (hidden) return null

  function dismiss() {
    try {
      localStorage.setItem(DISMISS_KEY, '1')
    } catch {
      /* приватный режим — плашка просто вернётся в следующей сессии */
    }
    setHidden(true)
  }

  return (
    <div
      role="region"
      aria-label="Акция"
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '12px',
        flexWrap: 'wrap',
        padding: '9px 44px 9px 16px',
        position: 'relative',
        background: `linear-gradient(135deg, ${theme.accent}, ${theme.accent2})`,
        color: '#04130f',
        fontSize: '13.5px',
        fontWeight: 600,
        lineHeight: 1.4,
        textAlign: 'center',
      }}
    >
      <span>
        🎉 Скидка <b>{PROMO.discount}</b> на песню по промокоду{' '}
        <b style={{ letterSpacing: '0.03em' }}>{PROMO.code}</b> · {PROMO.note}
      </span>
      <Link
        to={`/?promo=${PROMO.code}`}
        style={{
          flexShrink: 0,
          background: '#04130f',
          color: theme.accent,
          fontWeight: 700,
          fontSize: '12.5px',
          borderRadius: '999px',
          padding: '5px 14px',
          textDecoration: 'none',
          whiteSpace: 'nowrap',
        }}
      >
        Применить →
      </Link>
      <button
        type="button"
        onClick={dismiss}
        aria-label="Закрыть"
        style={{
          position: 'absolute',
          right: '10px',
          top: '50%',
          transform: 'translateY(-50%)',
          background: 'rgba(4,19,15,0.12)',
          border: 'none',
          borderRadius: '50%',
          width: '22px',
          height: '22px',
          cursor: 'pointer',
          color: '#04130f',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '14px',
          lineHeight: 1,
        }}
      >
        ✕
      </button>
    </div>
  )
}
